// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"strconv"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
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
	pendingBlks  chan *block.Block
	cancelChan   chan interface{}
	timerFactory *prometheustimer.TimerFactory
	dao          BlockDAO
	indexer      blockindex.Indexer
}

// NewIndexBuilder instantiates an index builder
func NewIndexBuilder(chainID uint32, dao BlockDAO, indexer blockindex.Indexer, bufferSize uint64) (*IndexBuilder, error) {
	timerFactory, err := prometheustimer.New(
		"iotex_indexer_batch_time",
		"Indexer batch time",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(chainID), 10)},
	)
	if err != nil {
		return nil, err
	}
	return &IndexBuilder{
		pendingBlks:  make(chan *block.Block, bufferSize),
		cancelChan:   make(chan interface{}),
		timerFactory: timerFactory,
		dao:          dao,
		indexer:      indexer,
	}, nil
}

// Start starts the index builder
func (ib *IndexBuilder) Start(ctx context.Context) error {
	if err := ib.indexer.Start(ctx); err != nil {
		return err
	}
	if err := ib.init(); err != nil {
		return err
	}
	// start handler to index incoming new block
	go ib.handler()
	return nil
}

// Stop stops the index builder
func (ib *IndexBuilder) Stop(ctx context.Context) error {
	close(ib.cancelChan)
	return ib.indexer.Stop(ctx)
}

// Indexer returns the indexer
func (ib *IndexBuilder) Indexer() blockindex.Indexer {
	return ib.indexer
}

// ReceiveBlock handles the block and create the indices for the actions and receipts in it
func (ib *IndexBuilder) ReceiveBlock(blk *block.Block) error {
	ib.pendingBlks <- blk
	return nil
}

func (ib *IndexBuilder) handler() {
	for {
		select {
		case <-ib.cancelChan:
			return
		case blk := <-ib.pendingBlks:
			timer := ib.timerFactory.NewTimer("indexBlock")
			if err := ib.indexer.PutBlock(blk); err != nil {
				log.L().Error(
					"Error when indexing the block",
					zap.Uint64("height", blk.Height()),
					zap.Error(err),
				)
			}
			if err := ib.indexer.Commit(); err != nil {
				log.L().Error(
					"Error when committing the block index",
					zap.Uint64("height", blk.Height()),
					zap.Error(err),
				)
			}
			timer.End()
			if blk.Height()%100 == 0 {
				log.L().Info("indexing new block", zap.Uint64("height", blk.Height()))
			}
		}
	}
}

func (ib *IndexBuilder) init() error {
	startHeight, err := ib.indexer.GetBlockchainHeight()
	if err != nil {
		return err
	}
	tipHeight := ib.dao.GetTipHeight()
	if startHeight == tipHeight {
		// indexer height consistent with dao height
		zap.L().Info("Consistent DB", zap.Uint64("height", startHeight))
		return nil
	}
	if startHeight > tipHeight {
		// indexer height > dao height
		// this shouldn't happen unless blocks are deliberately removed from dao w/o removing index
		// in this case we revert the extra block index, but nothing we can do to revert action index
		err := errors.Errorf("Inconsistent DB: indexer height %d > blockDAO height %d", startHeight, tipHeight)
		zap.L().Error(err.Error())
		return err
	}
	// update index to latest block
	for startHeight++; startHeight <= tipHeight; startHeight++ {
		blk, err := ib.dao.GetBlockByHeight(startHeight)
		if err != nil {
			return err
		}
		if err := ib.indexer.PutBlock(blk); err != nil {
			return err
		}
		// commit once every 5000 blocks
		if startHeight%5000 == 0 || startHeight == tipHeight {
			if err := ib.indexer.Commit(); err != nil {
				return err
			}
			zap.L().Info("Finished indexing blocks up to", zap.Uint64("height", startHeight))
		}
	}
	if startHeight >= tipHeight {
		// successfully migrated to latest block
		zap.L().Info("Finished migrating DB", zap.Uint64("height", startHeight))
		return ib.purgeObsoleteIndex()
	}
	return nil
}

func (ib *IndexBuilder) purgeObsoleteIndex() error {
	store := ib.dao.KVStore()
	if err := store.Delete(blockAddressActionMappingNS, nil); err != nil {
		return err
	}
	if err := store.Delete(blockAddressActionCountMappingNS, nil); err != nil {
		return err
	}
	if err := store.Delete(blockActionBlockMappingNS, nil); err != nil {
		return err
	}
	if err := store.Delete(blockActionReceiptMappingNS, nil); err != nil {
		return err
	}
	if err := store.Delete(numActionsNS, nil); err != nil {
		return err
	}
	if err := store.Delete(transferAmountNS, nil); err != nil {
		return err
	}
	return nil
}
