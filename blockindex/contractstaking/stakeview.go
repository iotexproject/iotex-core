package contractstaking

import (
	"context"
	"sync"

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
	mu     sync.RWMutex
}

func (s *stakeView) Clone() staking.ContractStakeView {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.dirty != nil {
		return s.dirty.bucketsByCandidate(candidate, s.height)
	}
	return s.clean.bucketsByCandidate(candidate, s.height)
}

func (s *stakeView) CreatePreStates(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	return nil
}

func (s *stakeView) Handle(ctx context.Context, receipt *action.Receipt) error {
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}
	var (
		blkCtx  = protocol.MustGetBlockCtx(ctx)
		handler *contractStakingEventHandler
	)
	for _, log := range receipt.Logs() {
		if log.Address != s.helper.config.ContractAddress {
			continue
		}
		if handler == nil {
			s.mu.Lock()
			defer s.mu.Unlock()
			// new event handler for this receipt
			if s.dirty == nil {
				s.dirty = s.clean.Clone()
			}
			handler = newContractStakingEventHandler(s.dirty)
		}
		if err := handler.HandleEvent(ctx, blkCtx.BlockHeight, log); err != nil {
			return err
		}
	}
	if handler == nil {
		return nil
	}
	_, delta := handler.Result()
	// update cache
	if err := s.dirty.Merge(delta, blkCtx.BlockHeight); err != nil {
		return err
	}
	return nil
}

func (s *stakeView) Commit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dirty != nil {
		s.clean = s.dirty
		s.dirty = nil
	}
}
