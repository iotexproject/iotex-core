package staking

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type (
	BlockStore interface {
		GetReceipts(uint64) ([]*action.Receipt, error)
		HeaderByHeight(height uint64) (*block.Header, error)
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

func (b *contractStakeViewBuilder) Build(ctx context.Context, sr protocol.StateReader, height uint64) (ContractStakeView, error) {
	view, err := b.indexer.LoadStakeView(ctx, sr)
	if err != nil {
		return nil, err
	}
	indexerHeight, err := b.indexer.Height()
	if err != nil {
		return nil, err
	}
	if indexerHeight == height {
		return view, nil
	}
	if indexerHeight > height {
		return nil, errors.Errorf("indexer height %d is greater than requested height %d", indexerHeight, height)
	}
	if b.blockdao == nil {
		return nil, errors.Errorf("blockdao is nil, cannot build view for height %d", height)
	}
	for h := indexerHeight + 1; h <= height; h++ {
		receipts, err := b.blockdao.GetReceipts(h)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get receipts at height %d", h)
		}
		header, err := b.blockdao.HeaderByHeight(h)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get header at height %d", h)
		}
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    h,
			BlockTimeStamp: header.Timestamp(),
		})
		if err = view.AddBlockReceipts(ctx, receipts); err != nil {
			return nil, errors.Wrapf(err, "failed to build view with block at height %d", h)
		}
	}
	return view, nil
}
