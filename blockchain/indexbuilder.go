// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"strconv"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var batchSizeMtc = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "iotex_indexer_batch_size",
		Help: "Indexer batch size",
	},
	[]string{},
)

func init() {
	prometheus.MustRegister(batchSizeMtc)
}

// IndexBuilder defines the index builder
type IndexBuilder struct {
	store        db.KVStore
	pendingBlks  chan *block.Block
	cancelChan   chan interface{}
	timerFactory *prometheustimer.TimerFactory
	dao          *blockDAO
}

// NewIndexBuilder instantiates an index builder
func NewIndexBuilder(chain Blockchain) (*IndexBuilder, error) {
	bc, ok := chain.(*blockchain)
	if !ok {
		log.S().Panic("unexpected blockchain implementation")
	}
	timerFactory, err := prometheustimer.New(
		"iotex_indexer_batch_time",
		"Indexer batch time",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(bc.ChainID()), 10)},
	)
	if err != nil {
		return nil, err
	}
	return &IndexBuilder{
		store:        bc.dao.kvstore,
		pendingBlks:  make(chan *block.Block, 64), // Actually 1 should be enough
		cancelChan:   make(chan interface{}),
		timerFactory: timerFactory,
		dao:          bc.dao,
	}, nil
}

// Start starts the index builder
func (ib *IndexBuilder) Start(_ context.Context) error {
	if err := ib.initAndLoadActions(); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ib.cancelChan:
				return
			case blk := <-ib.pendingBlks:
				timer := ib.timerFactory.NewTimer("indexBlock")
				batch := db.NewBatch()
				if err := indexBlock(ib.store, blk, batch); err != nil {
					log.L().Info(
						"Error when indexing the block",
						zap.Uint64("height", blk.Height()),
						zap.Error(err),
					)
				}
				// index receipts
				putReceipts(blk.Height(), blk.Receipts, batch)
				batchSizeMtc.WithLabelValues().Set(float64(batch.Size()))
				if err := ib.store.Commit(batch); err != nil {
					log.L().Info(
						"Error when indexing the block",
						zap.Uint64("height", blk.Height()),
						zap.Error(err),
					)
				}
				timer.End()
			}
		}
	}()
	return nil
}

// Stop stops the index builder
func (ib *IndexBuilder) Stop(_ context.Context) error {
	close(ib.cancelChan)
	return nil
}

// HandleBlock handles the block and create the indices for the actions and receipts in it
func (ib *IndexBuilder) HandleBlock(blk *block.Block) error {
	ib.pendingBlks <- blk
	return nil
}
func initIndexActionsKey(store db.KVStore) error {
	_, err := store.Get(blockActionBlockMappingNS, indexActionsTipIndexKey)
	if err != nil && errors.Cause(err) == db.ErrNotExist {
		if err = store.Put(blockActionBlockMappingNS, indexActionsTipIndexKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for tip index of index actions")
		}
	}

	_, err = store.Get(blockActionBlockMappingNS, indexActionsTipHeightKey)
	if err != nil && errors.Cause(err) == db.ErrNotExist {
		if err = store.Put(blockActionBlockMappingNS, indexActionsTipHeightKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for tip height of index actions")
		}
	}
	return err
}
func (ib *IndexBuilder) getStartHeightAndIndex() (startHeight, startIndex uint64, err error) {
	// get index that already builded
	startIndex, err = getNextIndex(ib.store)
	if err != nil {
		return
	}
	// get height that already builded
	startHeight, err = getNextHeight(ib.store)
	if err != nil {
		return
	}
	return
}
func (ib *IndexBuilder) commitBatchAndClear(tipIndex, tipHeight uint64, batch db.KVStoreBatch) error {
	tipIndexBytes := byteutil.Uint64ToBytes(tipIndex)
	batch.Put(blockActionBlockMappingNS, indexActionsTipIndexKey, tipIndexBytes, "failed to put tip index of actions")
	tipHeightBytes := byteutil.Uint64ToBytes(tipHeight)
	batch.Put(blockActionBlockMappingNS, indexActionsTipHeightKey, tipHeightBytes, "failed to put tip height")
	if err := ib.store.Commit(batch); err != nil {
		return err
	}
	return nil
}
func (ib *IndexBuilder) initAndLoadActions() error {
	err := initIndexActionsKey(ib.store)
	if err != nil {
		return err
	}
	tipHeight, err := ib.dao.getBlockchainHeight()
	if err != nil {
		return err
	}
	startHeight, startIndex, err := ib.getStartHeightAndIndex()
	if err != nil {
		return err
	}
	zap.L().Info("Loading actions", zap.Uint64("startHeight", startHeight), zap.Uint64("startIndex", startIndex))
	batch := db.NewBatch()
	i := startHeight
	for ; i <= tipHeight; i++ {
		hash, err := ib.dao.getBlockHash(i)
		if err != nil {
			return err
		}
		body, err := ib.dao.body(hash)
		if err != nil {
			return err
		}
		blk := &block.Block{
			Body: *body,
		}
		err = indexBlockHash(startIndex, hash, ib.store, blk, batch)
		if err != nil {
			return err
		}
		receipts, err := ib.dao.getReceipts(i)
		// if receipts are not available,this error will be ignored
		if err != nil && errors.Cause(err) != db.ErrNotExist {
			return err
		}
		putReceipts(i, receipts, batch)
		startIndex += uint64(len(blk.Actions))
		// commit once every 10000 heights
		if i%10000 == 0 || i == tipHeight {
			if err := ib.commitBatchAndClear(startIndex, i, batch); err != nil {
				return err
			}
		}
		if i%1000 == 0 {
			zap.L().Info("committing actions", zap.Uint64("height", i), zap.Uint64("index", startIndex))
		}
	}
	return nil
}
func getNextIndex(store db.KVStore) (uint64, error) {
	value, err := store.Get(blockActionBlockMappingNS, indexActionsTipIndexKey)
	if err != nil && errors.Cause(err) == db.ErrNotExist {
		return 0, initIndexActionsKey(store)
	}
	nextIndex := enc.MachineEndian.Uint64(value)
	return nextIndex, nil
}
func getNextHeight(store db.KVStore) (uint64, error) {
	value, err := store.Get(blockActionBlockMappingNS, indexActionsTipHeightKey)
	if err != nil && errors.Cause(err) == db.ErrNotExist {
		return 0, initIndexActionsKey(store)
	}
	nextHeight := enc.MachineEndian.Uint64(value)
	nextHeight++
	return nextHeight, nil
}
func indexBlock(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	hash := blk.HashBlock()
	// get index that already builded
	startIndex, err := getNextIndex(store)
	if err != nil {
		return err
	}
	// get height that already builded
	startHeight, err := getNextHeight(store)
	if err != nil {
		return err
	}
	if err = indexBlockHash(startIndex, hash, store, blk, batch); err != nil {
		return err
	}
	tipIndexBytes := byteutil.Uint64ToBytes(startIndex + uint64(len(blk.Actions)))
	batch.Put(blockActionBlockMappingNS, indexActionsTipIndexKey, tipIndexBytes, "failed to put index actions")
	tipHeightBytes := byteutil.Uint64ToBytes(startHeight)
	batch.Put(blockActionBlockMappingNS, indexActionsTipHeightKey, tipHeightBytes, "failed to put tip height")
	return nil
}
func indexBlockHash(startActionsNum uint64, blkHash hash.Hash256, store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	for i, elp := range blk.Actions {
		actHash := elp.Hash()
		batch.Put(blockActionBlockMappingNS, actHash[hashOffset:], blkHash[:], "failed to put action hash %x", actHash)
		indexActionsBytes := byteutil.Uint64ToBytes(startActionsNum + uint64(i))
		batch.Put(blockActionBlockMappingNS, indexActionsBytes, actHash[:], "failed to put index of actions %x", actHash)
	}

	return putActions(store, blk, batch)
}

func putActions(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	senderDelta := make(map[hash.Hash160]uint64)
	recipientDelta := make(map[hash.Hash160]uint64)

	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		callerAddrBytes := hash.BytesToHash160(selp.SrcPubkey().Hash())

		// get action count for sender
		senderActionCount, err := getActionCountBySenderAddress(store, callerAddrBytes)
		if err != nil {
			return errors.Wrapf(err, "for sender %x", callerAddrBytes)
		}
		if delta, ok := senderDelta[callerAddrBytes]; ok {
			senderActionCount += delta
			senderDelta[callerAddrBytes]++
		} else {
			senderDelta[callerAddrBytes] = 1
		}

		// put new action to sender
		senderKey := append(actionFromPrefix, callerAddrBytes[:]...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderActionCount)...)
		batch.Put(blockAddressActionMappingNS, senderKey, actHash[:],
			"failed to put action hash %x for sender %x", actHash, callerAddrBytes)

		// update sender action count
		senderActionCountKey := append(actionFromPrefix, callerAddrBytes[:]...)
		batch.Put(blockAddressActionCountMappingNS, senderActionCountKey,
			byteutil.Uint64ToBytes(senderActionCount+1),
			"failed to bump action count %x for sender %x", actHash, callerAddrBytes)

		dst, ok := selp.Destination()
		if !ok || dst == "" {
			continue
		}
		dstAddr, err := address.FromString(dst)
		if err != nil {
			return err
		}
		dstAddrBytes := hash.BytesToHash160(dstAddr.Bytes())

		if dstAddrBytes == callerAddrBytes {
			// recipient is same as sender
			continue
		}

		// get action count for recipient
		recipientActionCount, err := getActionCountByRecipientAddress(store, dstAddrBytes)
		if err != nil {
			return errors.Wrapf(err, "for recipient %x", dstAddrBytes)
		}
		if delta, ok := recipientDelta[dstAddrBytes]; ok {
			recipientActionCount += delta
			recipientDelta[dstAddrBytes]++
		} else {
			recipientDelta[dstAddrBytes] = 1
		}

		// put new action to recipient
		recipientKey := append(actionToPrefix, dstAddrBytes[:]...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientActionCount)...)
		batch.Put(blockAddressActionMappingNS, recipientKey, actHash[:],
			"failed to put action hash %x for recipient %x", actHash, dstAddrBytes)

		// update recipient action count
		recipientActionCountKey := append(actionToPrefix, dstAddrBytes[:]...)
		batch.Put(blockAddressActionCountMappingNS, recipientActionCountKey,
			byteutil.Uint64ToBytes(recipientActionCount+1), "failed to bump action count %x for recipient %x",
			actHash, dstAddrBytes)
	}
	return nil
}

// putReceipts store receipt into db
func putReceipts(blkHeight uint64, blkReceipts []*action.Receipt, batch db.KVStoreBatch) {
	if blkReceipts == nil {
		zap.L().Info("Block contains no receipt", zap.Uint64("height", blkHeight))
		return
	}
	var heightBytes [8]byte
	enc.MachineEndian.PutUint64(heightBytes[:], blkHeight)
	for _, r := range blkReceipts {
		batch.Put(
			blockActionReceiptMappingNS,
			r.ActionHash[hashOffset:],
			heightBytes[:],
			"Failed to put receipt index for action %x",
			r.ActionHash[:],
		)
	}
}

func getBlockHashByActionHash(store db.KVStore, h hash.Hash256) (hash.Hash256, error) {
	var blkHash hash.Hash256
	value, err := store.Get(blockActionBlockMappingNS, h[hashOffset:])
	if err != nil {
		return blkHash, errors.Wrapf(err, "failed to get action %x", h)
	}
	if len(value) == 0 {
		return blkHash, errors.Wrapf(db.ErrNotExist, "action %x missing", h)
	}
	copy(blkHash[:], value)
	return blkHash, nil
}

// getActionCountBySenderAddress returns action count by sender address
func getActionCountBySenderAddress(store db.KVStore, addrBytes hash.Hash160) (uint64, error) {
	senderActionCountKey := append(actionFromPrefix, addrBytes[:]...)
	value, err := store.Get(blockAddressActionCountMappingNS, senderActionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of actions by sender is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getActionsBySenderAddress returns actions for sender
func getActionsBySenderAddress(store db.KVStore, addrBytes hash.Hash160) ([]hash.Hash256, error) {
	// get action count for sender
	senderActionCount, err := getActionCountBySenderAddress(store, addrBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "for sender %x", addrBytes)
	}

	res, err := getActionsByAddress(store, addrBytes, senderActionCount, actionFromPrefix)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// getActionsByRecipientAddress returns actions for recipient
func getActionsByRecipientAddress(store db.KVStore, addrBytes hash.Hash160) ([]hash.Hash256, error) {
	// get action count for recipient
	recipientActionCount, getCountErr := getActionCountByRecipientAddress(store, addrBytes)
	if getCountErr != nil {
		return nil, errors.Wrapf(getCountErr, "for recipient %x", addrBytes)
	}

	res, err := getActionsByAddress(store, addrBytes, recipientActionCount, actionToPrefix)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// getActionCountByRecipientAddress returns action count by recipient address
func getActionCountByRecipientAddress(store db.KVStore, addrBytes hash.Hash160) (uint64, error) {
	recipientActionCountKey := append(actionToPrefix, addrBytes[:]...)
	value, err := store.Get(blockAddressActionCountMappingNS, recipientActionCountKey)
	if err != nil {
		return 0, nil
	}
	if len(value) == 0 {
		return 0, errors.New("count of actions by recipient is broken")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getActionsByAddress returns actions by address
func getActionsByAddress(store db.KVStore, addrBytes hash.Hash160, count uint64, keyPrefix []byte) ([]hash.Hash256, error) {
	var res []hash.Hash256

	for i := uint64(0); i < count; i++ {
		key := append(keyPrefix, addrBytes[:]...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := store.Get(blockAddressActionMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get action for index %d", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "action for index %d missing", i)
		}
		res = append(res, hash.BytesToHash256(value))
	}

	return res, nil
}
