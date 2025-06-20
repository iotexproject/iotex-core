package staking

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type (
	BlockStore interface {
		GetBlockByHeight(uint64) (*block.Block, error)
		GetReceipts(uint64) ([]*action.Receipt, error)
	}

	contractStakeViewBuilder struct {
		indexer  ContractStakingIndexer
		blockdao BlockStore
	}
)

func NewContractStakeViewBuilder(
	indexer ContractStakingIndexer,
	blockdao BlockStore,
) *contractStakeViewBuilder {
	return &contractStakeViewBuilder{
		indexer:  indexer,
		blockdao: blockdao,
	}
}

func (b *contractStakeViewBuilder) Build(ctx context.Context, height uint64) (ContractStakeView, error) {
	view, err := b.indexer.StartView(ctx)
	if err != nil {
		return nil, err
	}
	indexerHeight, err := b.indexer.Height()
	if err != nil {
		return nil, err
	}
	if indexerHeight == height {
		return view, nil
	} else if indexerHeight > height {
		return nil, errors.Errorf("indexer height %d is greater than requested height %d", indexerHeight, height)
	}
	if b.blockdao == nil {
		return nil, errors.Errorf("blockdao is nil, cannot build view for height %d", height)
	}
	for h := indexerHeight + 1; h <= height; h++ {
		blk, err := b.blockdao.GetBlockByHeight(h)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get block at height %d", h)
		}
		if blk.Receipts == nil {
			receipts, err := b.blockdao.GetReceipts(h)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get receipts at height %d", h)
			}
			blk.Receipts = receipts
		}
		if err = view.BuildWithBlock(ctx, blk); err != nil {
			return nil, errors.Wrapf(err, "failed to build view with block at height %d", h)
		}
	}
	return view, nil
}
