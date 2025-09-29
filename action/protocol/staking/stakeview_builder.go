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
	index, err := contractStakingIndexerAt(b.indexer, sr, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contract staking indexer at height")
	}
	return index.LoadStakeView(ctx, sr)
}
