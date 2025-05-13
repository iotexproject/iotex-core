package contractstaking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

type stakeView struct {
	helper *Indexer
	cache  *contractStakingCache
	height uint64
}

func (s *stakeView) Clone() staking.ContractStakeView {
	return &stakeView{
		helper: s.helper,
		cache:  s.cache.Clone(),
		height: s.height,
	}
}

func (s *stakeView) BucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	return s.cache.bucketsByCandidate(candidate, s.height)
}

func (s *stakeView) CreatePreStates(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	return nil
}

func (s *stakeView) Handle(ctx context.Context, receipt *action.Receipt) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	// new event handler for this receipt
	handler := newContractStakingEventHandler(s.cache)

	// handle events of receipt
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}
	for _, log := range receipt.Logs() {
		if log.Address != s.helper.config.ContractAddress {
			continue
		}
		if err := handler.HandleEvent(ctx, blkCtx.BlockHeight, log); err != nil {
			return err
		}
	}
	_, delta := handler.Result()
	// update cache
	if err := s.cache.Merge(delta, blkCtx.BlockHeight); err != nil {
		return err
	}
	return nil
}
