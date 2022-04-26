// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	// BlockIndexer defines an interface to accept block to build index
	BlockIndexer interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		Height() (uint64, error)
		PutBlock(context.Context, *block.Block) error
		DeleteTipBlock(context.Context, *block.Block) error
	}

	// BlockIndexerChecker defines a checker of block indexer
	BlockIndexerChecker struct {
		dao BlockDAO
	}
)

// NewBlockIndexerChecker creates a new block indexer checker
func NewBlockIndexerChecker(dao BlockDAO) *BlockIndexerChecker {
	return &BlockIndexerChecker{dao: dao}
}

// CheckIndexer checks a block indexer against block dao
func (bic *BlockIndexerChecker) CheckIndexer(ctx context.Context, indexer BlockIndexer, targetHeight uint64, progressReporter func(uint64)) error {
	bcCtx, ok := protocol.GetBlockchainCtx(ctx)
	if !ok {
		return errors.New("failed to find blockchain ctx")
	}
	g, ok := genesis.ExtractGenesisContext(ctx)
	if !ok {
		return errors.New("failed to find genesis ctx")
	}
	tipHeight, err := indexer.Height()
	if err != nil {
		return err
	}
	daoTip, err := bic.dao.Height()
	if err != nil {
		return err
	}
	if tipHeight > daoTip {
		return errors.New("indexer tip height cannot by higher than dao tip height")
	}
	tipBlk, err := bic.dao.GetBlockByHeight(tipHeight)
	if err != nil {
		return err
	}
	if targetHeight == 0 || targetHeight > daoTip {
		targetHeight = daoTip
	}
	for i := tipHeight + 1; i <= targetHeight; i++ {
		blk, err := bic.dao.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		if blk.Receipts == nil {
			blk.Receipts, err = bic.dao.GetReceipts(i)
			if err != nil {
				return err
			}
		}
		producer := blk.PublicKey().Address()
		if producer == nil {
			return errors.New("failed to get address")
		}
		bcCtx.Tip.Height = tipBlk.Height()
		if bcCtx.Tip.Height > 0 {
			bcCtx.Tip.Hash = tipBlk.HashHeader()
			bcCtx.Tip.Timestamp = tipBlk.Timestamp()
		} else {
			bcCtx.Tip.Hash = g.Hash()
			bcCtx.Tip.Timestamp = time.Unix(g.Timestamp, 0)
		}
		for {
			if err = indexer.PutBlock(protocol.WithBlockCtx(
				protocol.WithBlockchainCtx(ctx, bcCtx),
				protocol.BlockCtx{
					BlockHeight:    i,
					BlockTimeStamp: blk.Timestamp(),
					Producer:       producer,
					GasLimit:       g.BlockGasLimit,
				},
			), blk); err == nil {
				break
			}
			if i < g.HawaiiBlockHeight && errors.Cause(err) == block.ErrDeltaStateMismatch {
				log.L().Info("delta state mismatch", zap.Uint64("block", i))
				continue
			}
			return err
		}
		if progressReporter != nil {
			progressReporter(i)
		}
		tipBlk = blk
	}
	return nil
}
