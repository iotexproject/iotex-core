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
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
)

type (
	// BucketStore is the interface to manage buckets in the event handler
	BucketStore staking.EventHandler
	// VoteViewConfig is the configuration for the vote view
	VoteViewConfig struct {
		ContractAddr address.Address
	}
	// EventProcessorBuilder is the interface to build event processor
	EventProcessorBuilder interface {
		Build(context.Context, staking.EventHandler) staking.EventProcessor
	}
	voteView struct {
		indexer               staking.ContractStakingIndexer
		config                *VoteViewConfig
		height                uint64
		cur                   CandidateVotes
		store                 BucketStore
		cvm                   CandidateVotesManager
		processorBuilder      EventProcessorBuilder
		calculateVoteWeightFn CalculateUnmutedVoteWeightAtFn
	}
)

// NewVoteView creates a new vote view
func NewVoteView(
	indexer staking.ContractStakingIndexer,
	cfg *VoteViewConfig,
	height uint64,
	cur CandidateVotes,
	processorBuilder EventProcessorBuilder,
	cvm CandidateVotesManager,
	fn CalculateUnmutedVoteWeightAtFn,
) staking.ContractStakeView {
	return &voteView{
		indexer:               indexer,
		config:                cfg,
		height:                height,
		cur:                   cur,
		processorBuilder:      processorBuilder,
		cvm:                   cvm,
		calculateVoteWeightFn: fn,
	}
}

func (s *voteView) Height() uint64 {
	return s.height
}

// preDeployment reports whether the staking contract behind this view cannot
// have emitted an event yet by the block the returned view will serve. The only
// thing that ever mutates cur is voteViewEventHandler, and it is reached solely
// from eventProcessor.ProcessReceipts, which drops every log whose address is
// not config.ContractAddr; before the contract exists there is no such log, so
// the layer Wrap/Fork would push can never hold a change and has nothing to
// isolate.
//
// It still has a cost: every layer is another link that IsDirty walks, and the
// chain is only collapsed by Commit, which the staking protocol runs only on
// blocks where the whole viewData is dirty. Replaying mainnet from genesis that
// is almost never (the first system staking contract lands at 24486464), so
// without this guard the chain grows roughly with the block height and the
// recursive IsDirty turns replay quadratic (measured 185 blk/s at height 10k
// vs 3.2 blk/s at 355k, with IsDirty at 60% of CPU).
//
// s.height is the height this view last served (set in CreatePreStates), so the
// next block is s.height+1; requiring that one to be below the start height too
// means the view is already layering normally a block before the contract can
// be touched, and no view that shares cur with a sibling can ever commit a
// change into it.
func (s *voteView) preDeployment() bool {
	return s.height+1 < s.indexer.StartHeight()
}

// shallowCopy returns a view backed by the same cur and store. Only safe while
// preDeployment holds, i.e. while neither can be mutated.
func (s *voteView) shallowCopy() staking.ContractStakeView {
	cp := *s
	return &cp
}

func (s *voteView) Wrap() staking.ContractStakeView {
	if s.preDeployment() {
		return s.shallowCopy()
	}
	cur := newCandidateVotesWrapper(s.cur)
	var store BucketStore
	if s.store != nil {
		store = newBucketStore(s.store)
	}
	return &voteView{
		indexer:               s.indexer,
		config:                s.config,
		height:                s.height,
		cur:                   cur,
		store:                 store,
		processorBuilder:      s.processorBuilder,
		cvm:                   s.cvm,
		calculateVoteWeightFn: s.calculateVoteWeightFn,
	}
}

func (s *voteView) Fork() staking.ContractStakeView {
	if s.preDeployment() {
		return s.shallowCopy()
	}
	cur := newCandidateVotesWrapperCommitInClone(s.cur)
	var store BucketStore
	if s.store != nil {
		store = newBucketStore(s.store)
	}
	return &voteView{
		indexer:               s.indexer,
		config:                s.config,
		height:                s.height,
		cur:                   cur,
		store:                 store,
		processorBuilder:      s.processorBuilder,
		cvm:                   s.cvm,
		calculateVoteWeightFn: s.calculateVoteWeightFn,
	}
}

func (s *voteView) IsDirty() bool {
	return s.cur.IsDirty()
}

func (s *voteView) buckets(ctx context.Context) (map[uint64]*contractstaking.Bucket, error) {
	h, buckets, err := s.indexer.ContractStakingBuckets()
	if err != nil {
		return nil, err
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if s.indexer.StartHeight() <= blkCtx.BlockHeight && h != blkCtx.BlockHeight-1 {
		return nil, errors.Errorf("bucket cache height %d does not match current height %d", h, blkCtx.BlockHeight-1)
	}
	return buckets, nil
}

func (s *voteView) Migrate(ctx context.Context, handler staking.EventHandler) error {
	h, buckets, err := s.indexer.ContractStakingBuckets()
	if err != nil {
		return err
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if s.indexer.StartHeight() <= blkCtx.BlockHeight && h != blkCtx.BlockHeight-1 {
		return errors.Errorf("bucket cache height %d does not match current height %d", h, blkCtx.BlockHeight-1)
	}
	sortedIDs := make([]uint64, 0, len(buckets))
	for id := range buckets {
		sortedIDs = append(sortedIDs, id)
	}
	slices.Sort(sortedIDs)
	for _, id := range sortedIDs {
		if err := handler.PutBucket(s.config.ContractAddr, id, buckets[id]); err != nil {
			return err
		}
	}
	return nil
}

func (s *voteView) Revise(ctx context.Context) {
	buckets, err := s.buckets(ctx)
	if err != nil {
		return
	}
	s.cur = AggregateCandidateVotes(buckets, func(b *contractstaking.Bucket) *big.Int {
		return s.calculateVoteWeightFn(b, s.height)
	})
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
	s.store = newBucketStore(s.indexer)
	return nil
}

func (s *voteView) Handle(ctx context.Context, receipt *action.Receipt) error {
	handler, err := newVoteViewEventHandler(s.store, s.cur, func(b *contractstaking.Bucket) *big.Int {
		return s.calculateVoteWeightFn(b, s.height)
	})
	if err != nil {
		return errors.Wrap(err, "failed to create event handler")
	}
	return s.processorBuilder.Build(ctx, handler).ProcessReceipts(ctx, receipt)
}

func (s *voteView) AddBlockReceipts(ctx context.Context, receipts []*action.Receipt) error {
	return errors.New("not supported")
}

func (s *voteView) Commit(ctx context.Context, sm protocol.StateManager) error {
	isDirty := s.cur.IsDirty()
	s.cur = s.cur.Commit()
	if sm == nil || !isDirty {
		return nil
	}
	return s.cvm.Store(ctx, sm, s.cur)
}
