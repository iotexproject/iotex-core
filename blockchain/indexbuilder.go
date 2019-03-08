// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
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
	}, nil
}

// Start starts the index builder
func (ib *IndexBuilder) Start(_ context.Context) error {
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

func indexBlock(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	hash := blk.HashBlock()
	value, err := store.Get(blockNS, totalActionsKey)
	if err != nil {
		return errors.Wrap(err, "failed to get total actions")
	}
	totalActions := enc.MachineEndian.Uint64(value)
	totalActions += uint64(len(blk.Actions))
	totalActionsBytes := byteutil.Uint64ToBytes(totalActions)
	batch.Put(blockNS, totalActionsKey, totalActionsBytes, "failed to put total actions")
	for _, elp := range blk.Actions {
		actHash := elp.Hash()
		batch.Put(blockActionBlockMappingNS, actHash[:], hash[:], "failed to put action hash %x", actHash)
	}

	return putActions(store, blk, batch)
}

func putActions(store db.KVStore, blk *block.Block, batch db.KVStoreBatch) error {
	senderDelta := make(map[string]uint64)
	recipientDelta := make(map[string]uint64)

	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
		if err != nil {
			return err
		}
		callerAddrStr := callerAddr.String()

		// get action count for sender
		senderActionCount, err := getActionCountBySenderAddress(store, callerAddrStr)
		if err != nil {
			return errors.Wrapf(err, "for sender %s", callerAddrStr)
		}
		if delta, ok := senderDelta[callerAddrStr]; ok {
			senderActionCount += delta
			senderDelta[callerAddrStr]++
		} else {
			senderDelta[callerAddrStr] = 1
		}

		// put new action to sender
		senderKey := append(actionFromPrefix, callerAddrStr...)
		senderKey = append(senderKey, byteutil.Uint64ToBytes(senderActionCount)...)
		batch.Put(blockAddressActionMappingNS, senderKey, actHash[:],
			"failed to put action hash %x for sender %s", actHash, callerAddrStr)

		// update sender action count
		senderActionCountKey := append(actionFromPrefix, callerAddrStr...)
		batch.Put(blockAddressActionCountMappingNS, senderActionCountKey,
			byteutil.Uint64ToBytes(senderActionCount+1),
			"failed to bump action count %x for sender %s", actHash, callerAddrStr)

		dst, ok := selp.Destination()
		if !ok {
			continue
		}
		// get action count for recipient
		recipientActionCount, err := getActionCountByRecipientAddress(store, dst)
		if err != nil {
			return errors.Wrapf(err, "for recipient %s", dst)
		}
		if delta, ok := recipientDelta[dst]; ok {
			recipientActionCount += delta
			recipientDelta[dst]++
		} else {
			recipientDelta[dst] = 1
		}

		// put new action to recipient
		recipientKey := append(actionToPrefix, dst...)
		recipientKey = append(recipientKey, byteutil.Uint64ToBytes(recipientActionCount)...)
		batch.Put(blockAddressActionMappingNS, recipientKey, actHash[:],
			"failed to put action hash %x for recipient %s", actHash, dst)

		// update recipient action count
		recipientActionCountKey := append(actionToPrefix, dst...)
		batch.Put(blockAddressActionCountMappingNS, recipientActionCountKey,
			byteutil.Uint64ToBytes(recipientActionCount+1), "failed to bump action count %x for recipient %s",
			actHash, dst)
	}
	return nil
}

// putReceipts store receipt into db
func putReceipts(blkHeight uint64, blkReceipts []*action.Receipt, batch db.KVStoreBatch) {
	if blkReceipts == nil {
		return
	}
	var heightBytes [8]byte
	enc.MachineEndian.PutUint64(heightBytes[:], blkHeight)
	for _, r := range blkReceipts {
		batch.Put(
			blockActionReceiptMappingNS,
			r.ActHash[:],
			heightBytes[:],
			"Failed to put receipt index for action %x",
			r.ActHash[:],
		)
	}
}

func getBlockHashByActionHash(store db.KVStore, h hash.Hash256) (hash.Hash256, error) {
	var blkHash hash.Hash256
	value, err := store.Get(blockActionBlockMappingNS, h[:])
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
func getActionCountBySenderAddress(store db.KVStore, address string) (uint64, error) {
	senderActionCountKey := append(actionFromPrefix, address...)
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
func getActionsBySenderAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	// get action count for sender
	senderActionCount, err := getActionCountBySenderAddress(store, address)
	if err != nil {
		return nil, errors.Wrapf(err, "for sender %x", address)
	}

	res, err := getActionsByAddress(store, address, senderActionCount, actionFromPrefix)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// getActionsByRecipientAddress returns actions for recipient
func getActionsByRecipientAddress(store db.KVStore, address string) ([]hash.Hash256, error) {
	// get action count for recipient
	recipientActionCount, getCountErr := getActionCountByRecipientAddress(store, address)
	if getCountErr != nil {
		return nil, errors.Wrapf(getCountErr, "for recipient %x", address)
	}

	res, err := getActionsByAddress(store, address, recipientActionCount, actionToPrefix)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// getActionCountByRecipientAddress returns action count by recipient address
func getActionCountByRecipientAddress(store db.KVStore, address string) (uint64, error) {
	recipientActionCountKey := append(actionToPrefix, address...)
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
func getActionsByAddress(store db.KVStore, address string, count uint64, keyPrefix []byte) ([]hash.Hash256, error) {
	var res []hash.Hash256

	for i := uint64(0); i < count; i++ {
		key := append(keyPrefix, address...)
		key = append(key, byteutil.Uint64ToBytes(i)...)
		value, err := store.Get(blockAddressActionMappingNS, key)
		if err != nil {
			return res, errors.Wrapf(err, "failed to get action for index %d", i)
		}
		if len(value) == 0 {
			return res, errors.Wrapf(db.ErrNotExist, "action for index %d missing", i)
		}
		actHash := hash.ZeroHash256
		copy(actHash[:], value)
		res = append(res, actHash)
	}

	return res, nil
}
