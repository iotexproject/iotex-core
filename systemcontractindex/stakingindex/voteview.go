package stakingindex

import (
	"context"
	"math/big"

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
		config                *VoteViewConfig
		height                uint64
		cur                   CandidateVotes
		store                 BucketStore
		processorBuilder      EventProcessorBuilder
		calculateVoteWeightFn CalculateUnmutedVoteWeightAtFn
	}
)

// NewVoteView creates a new vote view
func NewVoteView(cfg *VoteViewConfig,
	height uint64,
	cur CandidateVotes,
	processorBuilder EventProcessorBuilder,
	fn CalculateUnmutedVoteWeightAtFn,
) staking.ContractStakeView {
	return &voteView{
		config:                cfg,
		height:                height,
		cur:                   cur,
		processorBuilder:      processorBuilder,
		calculateVoteWeightFn: fn,
	}
}

func (s *voteView) Height() uint64 {
	return s.height
}

func (s *voteView) Wrap() staking.ContractStakeView {
	cur := newCandidateVotesWrapper(s.cur)
	var store BucketStore
	if s.store != nil {
		store = newBucketStore(s.store)
	}
	return &voteView{
		config:                s.config,
		height:                s.height,
		cur:                   cur,
		store:                 store,
		processorBuilder:      s.processorBuilder,
		calculateVoteWeightFn: s.calculateVoteWeightFn,
	}
}

func (s *voteView) Fork() staking.ContractStakeView {
	cur := newCandidateVotesWrapperCommitInClone(s.cur)
	var store BucketStore
	if s.store != nil {
		store = newBucketStore(s.store)
	}
	return &voteView{
		config:                s.config,
		height:                s.height,
		cur:                   cur,
		store:                 store,
		processorBuilder:      s.processorBuilder,
		calculateVoteWeightFn: s.calculateVoteWeightFn,
	}
}

func (s *voteView) IsDirty() bool {
	return s.cur.IsDirty()
}

func (s *voteView) Migrate(handler staking.EventHandler, buckets map[uint64]*contractstaking.Bucket) error {
	for id := range buckets {
		if err := handler.PutBucket(s.config.ContractAddr, id, buckets[id]); err != nil {
			return err
		}
	}
	return nil
}

func (s *voteView) Revise(buckets map[uint64]*contractstaking.Bucket) {
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

func (s *voteView) CreatePreStates(ctx context.Context, br BucketReader) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	s.height = blkCtx.BlockHeight
	s.store = newBucketStore(br)
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
	s.cur = s.cur.Commit()
	return nil
}
