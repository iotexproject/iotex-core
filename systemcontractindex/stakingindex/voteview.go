package stakingindex

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

type (
	// BucketCache is the interface to get all buckets from the underlying storage
	BucketCache interface {
		ContractStakingBuckets() (uint64, map[uint64]*Bucket, error)
	}
	BucketStore staking.EventHandler
	// VoteViewConfig is the configuration for the vote view
	VoteViewConfig struct {
		MuteHeight, StartHeight uint64
		ContractAddr            address.Address
		Timestamped             bool
	}
	EventHandlerFactory interface {
		NewEventHandler(CandidateVotes) (BucketStore, error)
		NewEventHandlerWithStore(BucketStore, CandidateVotes) (BucketStore, error)
	}

	voteView struct {
		config *VoteViewConfig

		height uint64
		// current candidate votes
		cur CandidateVotes

		// bucketCache is used to migrate buckets from the underlying storage
		bucketCache BucketCache
		// handler is used as staging bucket storage for handling events
		handler BucketStore

		store CandidateVotesManager

		handlerBuilder   EventHandlerFactory
		processorBuilder EventProcessorBuilder
	}
)

func NewVoteView(cfg *VoteViewConfig,
	height uint64,
	cur CandidateVotes,
	handlerBuilder EventHandlerFactory,
	processorBuilder EventProcessorBuilder,
	bucketCache BucketCache,
	store CandidateVotesManager,
) staking.ContractStakeView {
	return &voteView{
		config:           cfg,
		height:           height,
		cur:              cur,
		handlerBuilder:   handlerBuilder,
		processorBuilder: processorBuilder,
		bucketCache:      bucketCache,
		store:            store,
	}
}

func (s *voteView) Height() uint64 {
	return s.height
}

func (s *voteView) Wrap() staking.ContractStakeView {
	cur := newCandidateVotesWrapper(s.cur)
	handler, err := s.handlerBuilder.NewEventHandlerWithStore(s.handler, cur)
	if err != nil {
		panic(errors.Wrap(err, "failed to wrap vote view event handler"))
	}
	return &voteView{
		config:      s.config,
		height:      s.height,
		cur:         cur,
		handler:     handler,
		bucketCache: s.bucketCache,
	}
}

func (s *voteView) Fork() staking.ContractStakeView {
	cur := newCandidateVotesWrapperCommitInClone(s.cur)
	handler, err := s.handlerBuilder.NewEventHandlerWithStore(s.handler, cur)
	if err != nil {
		panic(errors.Wrap(err, "failed to fork vote view event handler"))
	}
	return &voteView{
		config:      s.config,
		height:      s.height,
		cur:         cur,
		handler:     handler,
		bucketCache: s.bucketCache,
	}
}

func (s *voteView) IsDirty() bool {
	return s.cur.IsDirty()
}

func (s *voteView) Migrate(handler staking.EventHandler) error {
	height, buckets, err := s.bucketCache.ContractStakingBuckets()
	if err != nil {
		return err
	}
	if height != s.height-1 {
		return errors.Errorf("bucket cache height %d does not match vote view height %d", height, s.height)
	}
	for id := range buckets {
		if err := handler.PutBucket(s.config.ContractAddr, id, buckets[id]); err != nil {
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
	handler, err := s.handlerBuilder.NewEventHandler(s.cur)
	if err != nil {
		return err
	}
	s.handler = handler
	return nil
}

func (s *voteView) Handle(ctx context.Context, receipt *action.Receipt) error {
	return s.processorBuilder.Build(ctx, s.handler).ProcessReceipts(ctx, receipt)
}

func (s *voteView) AddBlockReceipts(ctx context.Context, receipts []*action.Receipt, handler staking.EventHandler) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	height := blkCtx.BlockHeight
	if height < s.config.StartHeight {
		return nil
	}
	if height != s.height+1 && height != s.config.StartHeight {
		return errors.Errorf("block height %d does not match stake view height %d", height, s.height+1)
	}
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	if err := s.processorBuilder.Build(ctx, s.handler).ProcessReceipts(ctx, receipts...); err != nil {
		return errors.Wrapf(err, "failed to handle receipts at height %d", height)
	}
	s.height = height
	return nil
}

func (s *voteView) Commit(ctx context.Context, sm protocol.StateManager) error {
	s.cur.Commit()

	return s.store.Store(ctx, sm, s.cur)
}
