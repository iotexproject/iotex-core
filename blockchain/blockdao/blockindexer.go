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
	blockIndexerCheckerKeyType struct{}

	// BlockIndexer defines an interface to accept block to build index
	BlockIndexer interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		Height() (uint64, error)
		PutBlock(context.Context, *block.Block) error
	}

	// BlockIndexerChecker defines a checker of block indexer
	BlockIndexerChecker interface {
		Height() (uint64, error)
		CheckIndexer(ctx context.Context, indexer BlockIndexer, targetHeight uint64, progressReporter func(uint64)) error
	}
	blockIndexerChecker struct {
		dao BlockDAO
	}
)

// WithChecker adds BlockIndexerChecker to context
func WithChecker(ctx context.Context, checker BlockIndexerChecker) context.Context {
	return context.WithValue(ctx, blockIndexerCheckerKeyType{}, checker)
}

// GetChecker returns the BlockIndexerChecker from context
func GetChecker(ctx context.Context) BlockIndexerChecker {
	if v := ctx.Value(blockIndexerCheckerKeyType{}); v != nil {
		if checker, ok := v.(BlockIndexerChecker); ok {
			return checker
		}
	}
	return nil
}

// MustGetChecker returns the BlockIndexerChecker from context, panic if not found
func MustGetChecker(ctx context.Context) BlockIndexerChecker {
	checker := GetChecker(ctx)
	if checker == nil {
		log.L().Panic("failed to get BlockIndexerChecker from context")
	}
	return checker
}

// NewBlockIndexerChecker creates a new block indexer checker
func NewBlockIndexerChecker(dao BlockDAO) BlockIndexerChecker {
	return &blockIndexerChecker{dao: dao}
}

func (bic *blockIndexerChecker) Height() (uint64, error) {
	return bic.dao.Height()
}

// CheckIndexer checks a block indexer against block dao
func (bic *blockIndexerChecker) CheckIndexer(ctx context.Context, indexer BlockIndexer, targetHeight uint64, progressReporter func(uint64)) error {
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
	for i := startHeight; i <= targetHeight; i++ {
		// terminate if context is done
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
		for {
			if err = indexer.PutBlock(protocol.WithFeatureCtx(protocol.WithBlockCtx(
				protocol.WithBlockchainCtx(ctx, bcCtx),
				protocol.BlockCtx{
					BlockHeight:    i,
					BlockTimeStamp: blk.Timestamp(),
					Producer:       producer,
					GasLimit:       g.BlockGasLimitByHeight(i),
					BaseFee:        blk.BaseFee(),
					ExcessBlobGas:  blk.ExcessBlobGas(),
				},
			)), blk); err == nil {
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
