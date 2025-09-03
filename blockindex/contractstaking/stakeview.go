package contractstaking

import (
	"context"
	"slices"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
)

type stakeView struct {
	contractAddr       address.Address
	config             Config
	cache              stakingCache
	genBlockDurationFn func(view uint64) blocksDurationFn
	height             uint64
}

func (s *stakeView) Height() uint64 {
	return s.height
}

func (s *stakeView) Wrap() staking.ContractStakeView {
	return &stakeView{
		contractAddr:       s.contractAddr,
		config:             s.config,
		cache:              newWrappedCache(s.cache),
		height:             s.height,
		genBlockDurationFn: s.genBlockDurationFn,
	}
}

func (s *stakeView) Fork() staking.ContractStakeView {
	return &stakeView{
		contractAddr:       s.contractAddr,
		cache:              newWrappedCacheWithCloneInCommit(s.cache),
		height:             s.height,
		genBlockDurationFn: s.genBlockDurationFn,
	}
}

func (s *stakeView) assembleBuckets(ids []uint64, types []*BucketType, infos []*bucketInfo) []*Bucket {
	vbs := make([]*Bucket, 0, len(ids))
	for i, id := range ids {
		bt := types[i]
		info := infos[i]
		if bt != nil && info != nil {
			vbs = append(vbs, s.assembleBucket(id, info, bt))
		}
	}
	return vbs
}

func (s *stakeView) IsDirty() bool {
	return s.cache.IsDirty()
}

func (s *stakeView) Migrate(handler staking.EventHandler) error {
	bts := s.cache.BucketTypes()
	tids := make([]uint64, 0, len(bts))
	for id := range bts {
		tids = append(tids, id)
	}
	slices.Sort(tids)
	for _, id := range tids {
		if err := handler.PutBucketType(s.contractAddr, bts[id]); err != nil {
			return err
		}
	}
	ids, types, infos := s.cache.Buckets()
	bucketMap := make(map[uint64]*bucketInfo, len(ids))
	typeMap := make(map[uint64]*BucketType, len(ids))
	for i, id := range ids {
		bucketMap[id] = infos[i]
		typeMap[id] = types[i]
	}
	slices.Sort(ids)
	for _, id := range ids {
		info, ok := bucketMap[id]
		if !ok {
			continue
		}
		bt := typeMap[id]
		if err := handler.PutBucket(s.contractAddr, id, &contractstaking.Bucket{
			Candidate:        info.Delegate,
			Owner:            info.Owner,
			StakedAmount:     bt.Amount,
			StakedDuration:   bt.Duration,
			CreatedAt:        info.CreatedAt,
			UnstakedAt:       info.UnstakedAt,
			UnlockedAt:       info.UnlockedAt,
			Muted:            false,
			IsTimestampBased: false,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *stakeView) BucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	ids, types, infos := s.cache.BucketsByCandidate(candidate)
	return s.assembleBuckets(ids, types, infos), nil
}

func (s *stakeView) assembleBucket(token uint64, bi *bucketInfo, bt *BucketType) *Bucket {
	return assembleBucket(token, bi, bt, s.contractAddr.String(), s.genBlockDurationFn(s.height))
}

func (s *stakeView) CreatePreStates(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	return nil
}

func (s *stakeView) Handle(ctx context.Context, receipt *action.Receipt) error {
	// new event handler for this receipt
	handler := newContractStakingDirty(newWrappedCache(s.cache))
	processor := newContractStakingEventProcessor(s.contractAddr, handler)
	if err := processor.ProcessReceipts(ctx, receipt); err != nil {
		return err
	}
	_, delta := handler.Finalize()
	s.cache = delta

	return nil
}

func (s *stakeView) Commit(ctx context.Context, sm protocol.StateManager) error {
	cache, err := s.cache.Commit(ctx, s.contractAddr, sm)
	if err != nil {
		return err
	}
	s.cache = cache
	return nil
}

func (s *stakeView) AddBlockReceipts(ctx context.Context, receipts []*action.Receipt) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	height := blkCtx.BlockHeight
	expectHeight := s.height + 1
	if expectHeight < s.config.ContractDeployHeight {
		expectHeight = s.config.ContractDeployHeight
	}
	if height < expectHeight {
		return nil
	}
	if height > expectHeight {
		return errors.Errorf("invalid block height %d, expect %d", height, expectHeight)
	}

	handler := newContractStakingDirty(newWrappedCache(s.cache))
	processor := newContractStakingEventProcessor(s.contractAddr, handler)
	if err := processor.ProcessReceipts(ctx, receipts...); err != nil {
		return err
	}
	_, delta := handler.Finalize()
	s.cache = delta
	s.height = height
	return nil
}
