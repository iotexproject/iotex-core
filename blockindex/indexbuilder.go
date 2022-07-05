// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/log/zlog"
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

// these NS belong to old DB before migrating to separate index
// they are left here only for record
// do NOT use them in the future to avoid potential conflict
const (
	_blockActionBlockMappingNS        = "a2b"
	_blockAddressActionMappingNS      = "a2a"
	_blockAddressActionCountMappingNS = "a2c"
	_blockActionReceiptMappingNS      = "a2r"
	_numActionsNS                     = "nac"
	_transferAmountNS                 = "tfa"
)

// IndexBuilder defines the index builder
type IndexBuilder struct {
	timerFactory *prometheustimer.TimerFactory
	dao          blockdao.BlockDAO
	indexer      Indexer
	genesis      genesis.Genesis
}

// NewIndexBuilder instantiates an index builder
func NewIndexBuilder(chainID uint32, g genesis.Genesis, dao blockdao.BlockDAO, indexer Indexer) (*IndexBuilder, error) {
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
		timerFactory: timerFactory,
		dao:          dao,
		indexer:      indexer,
		genesis:      g,
	}, nil
}

// Start starts the index builder
func (ib *IndexBuilder) Start(ctx context.Context) error {
	if err := ib.indexer.Start(ctx); err != nil {
		return err
	}
	if err := ib.init(ctx); err != nil {
		return err
	}
	// start handler to index incoming new block
	return nil
}

// Stop stops the index builder
func (ib *IndexBuilder) Stop(ctx context.Context) error {
	return ib.indexer.Stop(ctx)
}

// Indexer returns the indexer
func (ib *IndexBuilder) Indexer() Indexer {
	return ib.indexer
}

// ReceiveBlock handles the block and create the indices for the actions and receipts in it
func (ib *IndexBuilder) ReceiveBlock(blk *block.Block) error {
	timer := ib.timerFactory.NewTimer("indexBlock")
	if err := ib.indexer.PutBlock(genesis.WithGenesisContext(context.Background(), ib.genesis), blk); err != nil {
		log.L().Error(
			"Error when indexing the block",
			zap.Uint64("height", blk.Height()),
			zap.Error(err),
		)
		return err
	}
	timer.End()
	if blk.Height()%100 == 0 {
		log.L().Info("indexing new block", zap.Uint64("height", blk.Height()))
	}
	return nil
}

func (ib *IndexBuilder) init(ctx context.Context) error {
	startHeight, err := ib.indexer.Height()
	if err != nil {
		return err
	}
	tipHeight, err := ib.dao.Height()
	if err != nil {
		return err
	}
	if startHeight == tipHeight {
		// indexer height consistent with dao height
		zlog.L().Info().Uint64("height", startHeight).Msg("Consistent DB")
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
	var (
		gCtx = genesis.WithGenesisContext(ctx, ib.genesis)
		blks = make([]*block.Block, 0, 5000)
	)
	for startHeight++; startHeight <= tipHeight; startHeight++ {
		blk, err := ib.dao.GetBlockByHeight(startHeight)
		if err != nil {
			return err
		}
		blks = append(blks, blk)
		// commit once every 5000 blocks
		if startHeight%5000 == 0 || startHeight == tipHeight {
			if err := ib.indexer.PutBlocks(gCtx, blks); err != nil {
				return err
			}
			blks = blks[:0]
			zlog.L().Info().Uint64("height", startHeight).Msg("Finished indexing blocks up to")
		}
	}
	if startHeight >= tipHeight {
		// successfully migrated to latest block
		zap.L().Info("Finished migrating DB", zap.Uint64("height", startHeight))
	}
	return nil
}
