package contractstaking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

type stakeView struct {
	helper *Indexer
	clean  *contractStakingCache
	dirty  *contractStakingCache
	height uint64
}

func (s *stakeView) Clone() staking.ContractStakeView {
	clone := &stakeView{
		helper: s.helper,
		clean:  s.clean,
		dirty:  nil,
		height: s.height,
	}
	if s.dirty != nil {
		clone.clean = s.dirty.Clone()
	}
	return clone
}

func (s *stakeView) BucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	if s.dirty != nil {
		return s.dirty.bucketsByCandidate(candidate, s.height)
	}
	return s.clean.bucketsByCandidate(candidate, s.height)
}

func (s *stakeView) CreatePreStates(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	return nil
}

func (s *stakeView) Handle(ctx context.Context, receipt *action.Receipt) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	// new event handler for this receipt
	if s.dirty == nil {
		s.dirty = s.clean.Clone()
	}
	handler := newContractStakingEventHandler(s.dirty)

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
	if err := s.dirty.Merge(delta, blkCtx.BlockHeight); err != nil {
		return err
	}
	return nil
}

func (s *stakeView) Commit() {
	if s.dirty != nil {
		s.clean = s.dirty
		s.dirty = nil
	}
}
