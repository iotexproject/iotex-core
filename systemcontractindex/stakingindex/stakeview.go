package stakingindex

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
	cache              indexerCache
	height             uint64
	startHeight        uint64
	contractAddr       address.Address
	muteHeight         uint64
	timestamped        bool
	bucketNS           string
	genBlockDurationFn func(view uint64) blocksDurationFn
}

func (s *stakeView) Wrap() staking.ContractStakeView {
	return &stakeView{
		cache:              newWrappedCache(s.cache),
		height:             s.height,
		startHeight:        s.startHeight,
		contractAddr:       s.contractAddr,
		muteHeight:         s.muteHeight,
		timestamped:        s.timestamped,
		bucketNS:           s.bucketNS,
		genBlockDurationFn: s.genBlockDurationFn,
	}
}

func (s *stakeView) Fork() staking.ContractStakeView {
	return &stakeView{
		cache:              newWrappedCacheWithCloneInCommit(s.cache),
		height:             s.height,
		startHeight:        s.startHeight,
		contractAddr:       s.contractAddr,
		muteHeight:         s.muteHeight,
		timestamped:        s.timestamped,
		bucketNS:           s.bucketNS,
		genBlockDurationFn: s.genBlockDurationFn,
	}
}

func (s *stakeView) WriteBuckets(sm protocol.StateManager) error {
	ids := s.cache.BucketIdxs()
	slices.Sort(ids)
	buckets := s.cache.Buckets(ids)
	cssm := contractstaking.NewContractStakingStateManager(sm)
	for _, id := range ids {
		if err := cssm.UpsertBucket(s.contractAddr, id, buckets[id]); err != nil {
			return err
		}
	}
	return cssm.UpdateNumOfBuckets(s.contractAddr, s.cache.TotalBucketCount())
}

func (s *stakeView) BucketsByCandidate(candidate address.Address) ([]*VoteBucket, error) {
	idxs := s.cache.BucketIdsByCandidate(candidate)
	bkts := s.cache.Buckets(idxs)
	// filter out muted buckets
	idxsFiltered := make([]uint64, 0, len(bkts))
	bktsFiltered := make([]*Bucket, 0, len(bkts))
	for i := range bkts {
		if !bkts[i].Muted {
			idxsFiltered = append(idxsFiltered, idxs[i])
			bktsFiltered = append(bktsFiltered, bkts[i])
		}
	}
	vbs := batchAssembleVoteBucket(idxsFiltered, bktsFiltered, s.contractAddr.String(), s.genBlockDurationFn(s.height))
	return vbs, nil
}

func (s *stakeView) CreatePreStates(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	return nil
}

func (s *stakeView) Handle(ctx context.Context, receipt *action.Receipt) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	muted := s.muteHeight > 0 && blkCtx.BlockHeight >= s.muteHeight
	handler := newEventHandler(s.bucketNS, s.cache, blkCtx, s.timestamped, muted)
	return handler.handleReceipt(ctx, s.contractAddr.String(), receipt)
}

func (s *stakeView) AddBlockReceipts(ctx context.Context, receipts []*action.Receipt) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	height := blkCtx.BlockHeight
	if height < s.startHeight {
		return nil
	}
	if height != s.height+1 && height != s.startHeight {
		return errors.Errorf("block height %d does not match stake view height %d", height, s.height+1)
	}
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	muted := s.muteHeight > 0 && height >= s.muteHeight
	handler := newEventHandler(s.bucketNS, s.cache, blkCtx, s.timestamped, muted)
	for _, receipt := range receipts {
		if err := handler.handleReceipt(ctx, s.contractAddr.String(), receipt); err != nil {
			return errors.Wrapf(err, "failed to handle receipt at height %d", height)
		}
	}
	s.height = height
	return nil
}

func (s *stakeView) Commit(ctx context.Context, sm protocol.StateManager) error {
	cache, err := s.cache.Commit(ctx, s.contractAddr, s.timestamped, sm)
	if err != nil {
		return err
	}
	s.cache = cache

	return nil
}
