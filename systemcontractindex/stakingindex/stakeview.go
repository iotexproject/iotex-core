package stakingindex

import (
	"context"
	"sync"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

type stakeView struct {
	helper *Indexer
	cache  bucketCache
	height uint64
	mu     sync.RWMutex
}

func (s *stakeView) Clone() staking.ContractStakeView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &stakeView{
		helper: s.helper,
		cache:  s.cache.Copy(),
		height: s.height,
	}
}
func (s *stakeView) BucketsByCandidate(candidate address.Address) ([]*VoteBucket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	return nil
}

func (s *stakeView) Handle(ctx context.Context, receipt *action.Receipt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	blkCtx := protocol.MustGetBlockCtx(ctx)
	muted := s.helper.muteHeight > 0 && blkCtx.BlockHeight >= s.helper.muteHeight
	handler := newEventHandler(s.helper.bucketNS, s.cache, blkCtx, s.helper.timestamped, muted)
	return s.helper.handleReceipt(ctx, handler, receipt)
}

func (s *stakeView) Commit() {}

func (s *stakeView) BuildWithBlock(ctx context.Context, blk *block.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if blk.Height() != s.height+1 {
		return errors.Errorf("block height %d does not match stake view height %d", blk.Height(), s.height+1)
	}
	g, ok := genesis.ExtractGenesisContext(ctx)
	if !ok {
		return errors.New("failed to extract genesis context")
	}
	blkCtx := protocol.BlockCtx{
		BlockHeight:    blk.Height(),
		BlockTimeStamp: blk.Timestamp(),
		GasLimit:       g.BlockGasLimitByHeight(blk.Height()),
		Producer:       blk.PublicKey().Address(),
		BaseFee:        blk.BaseFee(),
		ExcessBlobGas:  blk.BlobGasUsed(),
	}
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	muted := s.helper.muteHeight > 0 && blk.Height() >= s.helper.muteHeight
	handler := newEventHandler(s.helper.bucketNS, s.cache, blkCtx, s.helper.timestamped, muted)
	for _, receipt := range blk.Receipts {
		if err := s.helper.handleReceipt(ctx, handler, receipt); err != nil {
			return errors.Wrapf(err, "failed to handle receipt at height %d", blk.Height())
		}
	}
	s.height = blk.Height()
	return nil
}
