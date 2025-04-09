// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	// BlockIndexer defines an interface to accept block to build index
	BlockIndexer interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		Height() (uint64, error)
		PutBlock(context.Context, *block.Block) error
	}

	// BlockIndexerWithStart defines an interface to accept block to build index from a start height
	BlockIndexerWithStart interface {
		BlockIndexer
		// StartHeight returns the start height of the indexer
		StartHeight() uint64
	}

	BlockIndexerStateDBWriter interface {
		BlockIndexer
		// StateDBWriter indicates this indexer writes to state DB
		StateDBWriter()
	}

	// BlockIndexerChecker defines a checker of block indexer
	BlockIndexerChecker struct {
		dao       BlockDAO
		validator block.BlockValidator
	}
)

// NewBlockIndexerChecker creates a new block indexer checker
func NewBlockIndexerChecker(dao BlockDAO, v block.BlockValidator) *BlockIndexerChecker {
	return &BlockIndexerChecker{
		dao:       dao,
		validator: v,
	}
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
	startHeight := tipHeight + 1
	if indexerWS, ok := indexer.(BlockIndexerWithStart); ok {
		indexStartHeight := indexerWS.StartHeight()
		if indexStartHeight > startHeight {
			startHeight = indexStartHeight
		}
	}
	for i := startHeight; i <= targetHeight; i++ {
		// ternimate if context is done
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "terminate the indexer checking")
		}
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
		pk := blk.PublicKey()
		if pk == nil {
			return errors.New("failed to get pubkey")
		}
		producer := pk.Address()
		if producer == nil {
			return errors.New("failed to get producer address")
		}
		bcCtx.Tip.Height = tipBlk.Height()
		if bcCtx.Tip.Height > 0 {
			bcCtx.Tip.GasUsed = tipBlk.GasUsed()
			bcCtx.Tip.Hash = tipBlk.HashHeader()
			bcCtx.Tip.Timestamp = tipBlk.Timestamp()
			bcCtx.Tip.BaseFee = tipBlk.BaseFee()
			bcCtx.Tip.BlobGasUsed = tipBlk.BlobGasUsed()
			bcCtx.Tip.ExcessBlobGas = tipBlk.ExcessBlobGas()
		} else {
			bcCtx.Tip.Hash = g.Hash()
			bcCtx.Tip.Timestamp = time.Unix(g.Timestamp, 0)
		}
		checkCtx := protocol.WithFeatureCtx(protocol.WithBlockCtx(
			protocol.WithBlockchainCtx(ctx, bcCtx),
			protocol.BlockCtx{
				BlockHeight:    i,
				BlockTimeStamp: blk.Timestamp(),
				Producer:       producer,
				GasLimit:       g.BlockGasLimitByHeight(i),
				BaseFee:        blk.BaseFee(),
				ExcessBlobGas:  blk.ExcessBlobGas(),
			},
		))
		if bic.validator != nil {
			if err := bic.validator(checkCtx, blk); err != nil {
				return err
			}
		}
		for {
			if err = indexer.PutBlock(checkCtx, blk); err == nil {
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
