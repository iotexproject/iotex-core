package contractstaking

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

type historyIndexer struct {
	staking.ContractStakingIndexer
}

func newHistoryIndexer(indexer staking.ContractStakingIndexer) *historyIndexer {
	return &historyIndexer{
		ContractStakingIndexer: indexer,
	}
}

func (h *historyIndexer) BucketTypes(height uint64) ([]*staking.ContractStakingBucketType, error) {
	return nil, errors.New("not implemented")
}
