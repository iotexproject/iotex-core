// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
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

const (
	blockAddressActionMappingNS      = "a2a"
	blockAddressActionCountMappingNS = "a2c"
	blockActionReceiptMappingNS      = "a2r"
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

type addrIndex map[hash.Hash160]db.CountingIndex

// IndexBuilder defines the index builder
type IndexBuilder struct {
	store        db.KVStore
	pendingBlks  chan *block.Block
	cancelChan   chan interface{}
	timerFactory *prometheustimer.TimerFactory
	dao          *blockDAO
	reindex      bool
	dirtyAddr    addrIndex
}

// NewIndexBuilder instantiates an index builder
func NewIndexBuilder(chain Blockchain, reindex bool) (*IndexBuilder, error) {
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
		reindex:      reindex,
		dirtyAddr:    make(addrIndex),
	}, nil
}

// Start starts the index builder
func (ib *IndexBuilder) Start(_ context.Context) error {
	if err := ib.init(); err != nil {
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
				if err := indexBlock(ib.store, blk.HashBlock(), blk.Height(), blk.Actions, batch, nil); err != nil {
					log.L().Error(
						"Error when indexing the block",
						zap.Uint64("height", blk.Height()),
						zap.Error(err),
					)
				}
				if err := ib.store.Commit(batch); err != nil {
					log.L().Error(
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

func (ib *IndexBuilder) init() error {
	tipHeight, err := ib.dao.getBlockchainHeight()
	if err != nil {
		return err
	}
	// check if we need to re-index
	if err := ib.checkReindex(); err != nil {
		return err
	}

	startHeight := uint64(1)
	if !ib.reindex {
		startHeight, err = ib.getNextHeight()
		if err != nil {
			return err
		}
	}
	// update index to latest block
	zap.L().Info("Loading blocks", zap.Uint64("startHeight", startHeight))
	batch := db.NewBatch()
	for i := startHeight; i <= tipHeight; i++ {
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
		err = indexBlock(ib.store, hash, i, blk.Actions, batch, ib.dirtyAddr)
		if err != nil {
			return err
		}
		// commit once every 10000 blocks
		if i%10000 == 0 || i == tipHeight {
			if err := ib.commitBatchAndClear(i, batch); err != nil {
				return err
			}
			zap.L().Info("Finished indexing blocks", zap.Uint64("height", i))
		}
	}
	return nil
}

func (ib *IndexBuilder) checkReindex() error {
	if _, err := ib.store.CountingIndex(totalActionsKey); err != nil && errors.Cause(err) == db.ErrBucketNotExist {
		// counting index does not exist, need to re-index
		ib.reindex = true
	}
	if ib.reindex {
		return ib.purgeObsoleteIndex()
	}
	return nil
}

func (ib *IndexBuilder) purgeObsoleteIndex() error {
	if err := ib.dao.kvstore.Delete(blockAddressActionMappingNS, nil); err != nil {
		return err
	}
	if err := ib.dao.kvstore.Delete(blockAddressActionCountMappingNS, nil); err != nil {
		return err
	}
	if err := ib.dao.kvstore.Delete(blockActionBlockMappingNS, nil); err != nil {
		return err
	}
	if err := ib.dao.kvstore.Delete(blockActionReceiptMappingNS, nil); err != nil {
		return err
	}
	return nil
}

func (ib *IndexBuilder) commitBatchAndClear(tipHeight uint64, batch db.KVStoreBatch) error {
	for _, v := range ib.dirtyAddr {
		if err := v.Commit(); err != nil {
			return err
		}
	}
	ib.dirtyAddr = nil
	ib.dirtyAddr = make(addrIndex)
	// update indexed height
	batch.Put(blockNS, topIndexedHeightKey, byteutil.Uint64ToBytes(tipHeight), "failed to put indexed height")
	return ib.store.Commit(batch)
}

func (ib *IndexBuilder) getNextHeight() (uint64, error) {
	value, err := ib.store.Get(blockNS, topIndexedHeightKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get indexed height")
	}
	nextHeight := enc.MachineEndian.Uint64(value)
	return nextHeight + 1, nil
}

// getIndexerForAddr returns the indexer for an address
// if indexer does not exist, one will be created and added to dirtyAddr
func getIndexerForAddr(kvstore db.KVStore, addr []byte, dirtyAddr addrIndex) (db.CountingIndex, error) {
	if dirtyAddr == nil {
		return nil, errors.New("empty dirtyAddr map")
	}

	address := hash.BytesToHash160(addr)
	indexer, ok := dirtyAddr[address]
	if !ok {
		// create indexer for addr if not exist
		var err error
		indexer, err = kvstore.CreateCountingIndexNX(addr)
		if err != nil {
			return nil, err
		}
		dirtyAddr[address] = indexer
	}
	return indexer, nil
}

// indexBlock builds index for the block
func indexBlock(kvstore db.KVStore, blkHash hash.Hash256, height uint64, actions []action.SealedEnvelope, batch db.KVStoreBatch, dirtyAddr addrIndex) error {
	localDirtyMap := dirtyAddr == nil
	if localDirtyMap {
		// use local dirtyAddr map if no external is provided
		dirtyAddr = make(addrIndex)
	}
	// get indexer for total actions
	total, err := getIndexerForAddr(kvstore, totalActionsKey, dirtyAddr)
	if err != nil {
		return err
	}

	// store 32-byte hash + 8-byte height, so getReceiptByActionHash() can use last 8-byte to directly pull receipts
	hashAndHeight := append(blkHash[:], byteutil.Uint64ToBytes(height)...)
	for _, elp := range actions {
		actHash := elp.Hash()
		batch.Put(blockActionBlockMappingNS, actHash[hashOffset:], hashAndHeight, "failed to put action hash %x", actHash)
		// add to total account index
		if err := total.Add(actHash[:], true); err != nil {
			return err
		}
		if err := indexAction(kvstore, actHash, elp, dirtyAddr); err != nil {
			return err
		}
	}
	if localDirtyMap {
		// commit local dirtyAddr map
		for k := range dirtyAddr {
			if err := dirtyAddr[k].Commit(); err != nil {
				return err
			}
		}
		// update indexed height
		batch.Put(blockNS, topIndexedHeightKey, byteutil.Uint64ToBytes(height), "failed to put indexed height")
	}
	return nil
}

// indexAction builds index for an action
func indexAction(kvstore db.KVStore, actHash hash.Hash256, elp action.SealedEnvelope, dirtyAddr addrIndex) error {
	// add to sender's index
	callerAddrBytes := elp.SrcPubkey().Hash()
	sender, err := getIndexerForAddr(kvstore, callerAddrBytes, dirtyAddr)
	if err != nil {
		return err
	}
	if err := sender.Add(actHash[:], true); err != nil {
		return err
	}

	dst, ok := elp.Destination()
	if !ok || dst == "" {
		return nil
	}
	dstAddr, err := address.FromString(dst)
	if err != nil {
		return err
	}
	dstAddrBytes := dstAddr.Bytes()

	if bytes.Compare(dstAddrBytes, callerAddrBytes) == 0 {
		// recipient is same as sender
		return nil
	}

	// add to recipient's index
	recipient, err := getIndexerForAddr(kvstore, dstAddrBytes, dirtyAddr)
	if err != nil {
		return err
	}
	return recipient.Add(actHash[:], true)
}
