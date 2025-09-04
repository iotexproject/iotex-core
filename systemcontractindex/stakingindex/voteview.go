package stakingindex

import (
	"context"
	"math/big"
	"slices"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

type voteView struct {
	// read only fields
	muteHeight, startHeight    uint64
	contractAddr               address.Address
	timestamped                bool
	ns, bucketNS               string
	calculateUnmutedVoteWeight calculateUnmutedVoteWeightFn
	indexer                    *Indexer

	// height of the view
	height uint64
	// current candidate votes
	cur CandidateVotes

	handler staking.EventHandler
}

func (s *voteView) Height() uint64 {
	return s.height
}

func (s *voteView) Wrap() staking.ContractStakeView {
	cur := newCandidateVotesWrapper(s.cur)
	var handler staking.EventHandler
	if s.handler != nil {
		handler = newVoteViewEventHandlerWrapper(s.handler, cur, s.calculateUnmutedVoteWeight)
	}
	return &voteView{
		muteHeight:                 s.muteHeight,
		contractAddr:               s.contractAddr,
		timestamped:                s.timestamped,
		height:                     s.height,
		cur:                        cur,
		handler:                    handler,
		calculateUnmutedVoteWeight: s.calculateUnmutedVoteWeight,
		indexer:                    s.indexer,
	}
}

func (s *voteView) Fork() staking.ContractStakeView {
	cur := newCandidateVotesWrapperCommitInClone(s.cur)
	var handler staking.EventHandler
	if s.handler != nil {
		handler = newVoteViewEventHandlerWrapper(s.handler, cur, s.calculateUnmutedVoteWeight)
	}
	return &voteView{
		muteHeight:                 s.muteHeight,
		contractAddr:               s.contractAddr,
		timestamped:                s.timestamped,
		height:                     s.height,
		cur:                        cur,
		handler:                    handler,
		calculateUnmutedVoteWeight: s.calculateUnmutedVoteWeight,
		indexer:                    s.indexer,
	}
}

func (s *voteView) IsDirty() bool {
	return s.cur.IsDirty()
}

func (s *voteView) Migrate(handler staking.EventHandler) error {
	var cache *base
	if s.indexer != nil {
		indexerHeight, err := s.indexer.Height()
		if err != nil {
			return err
		}
		if indexerHeight == s.height-1 {
			cache = s.indexer.cache
		}
	}
	if cache == nil {
		return errors.Errorf("cannot migrate vote view at height %d without indexer cache at height %d", s.height, s.height-1)
	}
	ids := cache.BucketIdxs()
	slices.Sort(ids)
	buckets := cache.Buckets(ids)
	for _, id := range ids {
		if err := handler.PutBucket(s.contractAddr, id, buckets[id]); err != nil {
			return err
		}
	}
	return nil
}

func (s *voteView) CandidateStakeVotes(ctx context.Context, candidate address.Address) *big.Int {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if !featureCtx.CreatePostActionStates {
		return s.cur.Base().Votes(featureCtx, candidate.String())
	}
	return s.cur.Votes(featureCtx, candidate.String())
}

func (s *voteView) CreatePreStates(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	return nil
}

func (s *voteView) Handle(ctx context.Context, receipt *action.Receipt) error {
	if s.handler == nil {
		return nil
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	muted := s.muteHeight > 0 && blkCtx.BlockHeight >= s.muteHeight
	return newEventProcessor(
		s.contractAddr, blkCtx, s.handler, s.timestamped, muted,
	).ProcessReceipts(ctx, receipt)
}

func (s *voteView) AddBlockReceipts(ctx context.Context, receipts []*action.Receipt, handler staking.EventHandler) error {
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
	if err := newEventProcessor(
		s.contractAddr, blkCtx, handler, s.timestamped, muted,
	).ProcessReceipts(ctx, receipts...); err != nil {
		return errors.Wrapf(err, "failed to handle receipts at height %d", height)
	}
	s.height = height
	return nil
}

var (
	voteViewKey      = []byte("voteview")
	voteViewNSPrefix = "voterview"
)

func (s *voteView) Commit(ctx context.Context, sm protocol.StateManager) error {
	s.cur.Commit()

	return s.storeVotes(ctx, sm)
}

func (s *voteView) storeVotes(ctx context.Context, sm protocol.StateManager) error {
	if _, err := sm.PutState(s.cur,
		protocol.KeyOption(voteViewKey),
		protocol.NamespaceOption(s.namespace()),
		protocol.SecondaryOnlyOption(),
	); err != nil {
		return errors.Wrap(err, "failed to put candidate votes state")
	}
	return nil
}

func (s *voteView) loadVotes(ctx context.Context, sr protocol.StateReader) error {
	_, err := sr.State(s.cur, protocol.KeyOption(voteViewKey), protocol.NamespaceOption(s.namespace()))
	if err != nil {
		return errors.Wrap(err, "failed to get candidate votes state")
	}
	return nil
}

func (s *voteView) namespace() string {
	return voteViewNSPrefix + s.contractAddr.String()
}
