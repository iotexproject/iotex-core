package contractstaking

import (
	"context"
	"slices"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
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

func (s *stakeView) WriteBuckets(sm protocol.StateManager) error {
	ids, types, infos := s.cache.Buckets()
	cssm := contractstaking.NewContractStakingStateManager(sm)
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
		if err := cssm.UpsertBucket(s.contractAddr, id, &contractstaking.Bucket{
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
	return cssm.UpdateNumOfBuckets(s.contractAddr, s.cache.TotalBucketCount())
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
	blkCtx := protocol.MustGetBlockCtx(ctx)
	// new event handler for this receipt
	handler := newContractStakingEventHandler(newWrappedCache(s.cache))

	// handle events of receipt
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}
	for _, log := range receipt.Logs() {
		if log.Address != s.contractAddr.String() {
			continue
		}
		if err := handler.HandleEvent(ctx, blkCtx.BlockHeight, log); err != nil {
			return err
		}
	}
	_, delta := handler.Result()
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

	handler := newContractStakingEventHandler(newWrappedCache(s.cache))
	if err := handler.HandleReceipts(ctx, height, receipts, s.contractAddr.String()); err != nil {
		return err
	}
	_, delta := handler.Result()
	s.cache = delta
	s.height = height
	return nil
}
