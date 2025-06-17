package stakingindex

import (
	"context"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

type stakeView struct {
	helper *Indexer
	cache  indexerCache
	height uint64
}

func (s *stakeView) Clone() staking.ContractStakeView {
	return &stakeView{
		helper: s.helper,
		cache:  newWrappedCache(s.cache),
		height: s.height,
	}
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
	vbs := batchAssembleVoteBucket(idxsFiltered, bktsFiltered, s.helper.common.ContractAddress(), s.helper.genBlockDurationFn(s.height))
	return vbs, nil
}

func (s *stakeView) CreatePreStates(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	return nil
}

func (s *stakeView) Handle(ctx context.Context, receipt *action.Receipt) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	muted := s.helper.muteHeight > 0 && blkCtx.BlockHeight >= s.helper.muteHeight
	handler := newEventHandler(s.helper.bucketNS, s.cache, blkCtx, s.helper.timestamped, muted)
	return s.helper.handleReceipt(ctx, handler, receipt)
}
