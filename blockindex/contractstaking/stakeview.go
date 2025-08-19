package contractstaking

import (
	"context"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

type stakeView struct {
	helper *Indexer
	cache  stakingCache
	height uint64
}

func (s *stakeView) Wrap() staking.ContractStakeView {
	return &stakeView{
		helper: s.helper,
		cache:  newWrappedCache(s.cache),
		height: s.height,
	}
}

func (s *stakeView) Fork() staking.ContractStakeView {
	return &stakeView{
		helper: s.helper,
		cache:  newWrappedCacheWithCloneInCommit(s.cache),
		height: s.height,
	}
}

func (s *stakeView) BucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	ids, types, infos := s.cache.BucketsByCandidate(candidate)
	vbs := make([]*Bucket, 0, len(ids))
	for i, id := range ids {
		bt := types[i]
		info := infos[i]
		if bt != nil && info != nil {
			vbs = append(vbs, s.assembleBucket(id, info, bt))
		}
	}
	return vbs, nil
}

func (s *stakeView) assembleBucket(token uint64, bi *bucketInfo, bt *BucketType) *Bucket {
	return assembleBucket(token, bi, bt, s.helper.config.ContractAddress, s.genBlockDurationFn(s.height))
}

func (s *stakeView) genBlockDurationFn(view uint64) blocksDurationFn {
	return func(start, end uint64) time.Duration {
		return s.helper.config.BlocksToDuration(start, end, view)
	}
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
		if log.Address != s.helper.config.ContractAddress {
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

func (s *stakeView) Commit() {
	s.cache = s.cache.Commit()
}

func (s *stakeView) AddBlockReceipts(ctx context.Context, receipts []*action.Receipt) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	height := blkCtx.BlockHeight
	expectHeight := s.height + 1
	if expectHeight < s.helper.config.ContractDeployHeight {
		expectHeight = s.helper.config.ContractDeployHeight
	}
	if height < expectHeight {
		return nil
	}
	if height > expectHeight {
		return errors.Errorf("invalid block height %d, expect %d", height, expectHeight)
	}

	handler, err := handleReceipts(ctx, height, receipts, &s.helper.config, s.cache)
	if err != nil {
		return err
	}
	_, delta := handler.Result()
	s.cache = delta.Commit()
	return nil
}
