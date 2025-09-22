package stakingindex

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontractindex"
)

const (
	stakingNS       = "sns"
	stakingBucketNS = "sbn"
)

var (
	stakingHeightKey           = []byte("shk")
	stakingTotalBucketCountKey = []byte("stbck")
)

type (
	// StakingIndexer defines the interface of staking indexer
	StakingIndexer interface {
		lifecycle.StartStopper
		Height() (uint64, error)
		StartHeight() uint64
		ContractAddress() address.Address
		Buckets(height uint64) ([]*VoteBucket, error)
		Bucket(id uint64, height uint64) (*VoteBucket, bool, error)
		BucketsByIndices(indices []uint64, height uint64) ([]*VoteBucket, error)
		BucketsByCandidate(candidate address.Address, height uint64) ([]*VoteBucket, error)
		TotalBucketCount(height uint64) (uint64, error)
		PutBlock(ctx context.Context, blk *block.Block) error
		LoadStakeView(context.Context, protocol.StateReader) (staking.ContractStakeView, error)
		CreateEventProcessor(context.Context, staking.EventHandler) staking.EventProcessor
		CreateMemoryEventHandler(context.Context) staking.CachedEventHandler
	}
	// Indexer is the staking indexer
	Indexer struct {
		common              *systemcontractindex.IndexerCommon
		cache               *base // in-memory cache, used to query index data
		mutex               sync.RWMutex
		blocksToDuration    blocksDurationAtFn // function to calculate duration from block range
		bucketNS            string
		ns                  string
		muteHeight          uint64
		timestamped         bool
		calculateVoteWeight CalculateVoteWeightFunc
	}
	// IndexerOption is the option to create an indexer
	IndexerOption func(*Indexer)

	blocksDurationFn        func(start uint64, end uint64) time.Duration
	blocksDurationAtFn      func(start uint64, end uint64, viewAt uint64) time.Duration
	CalculateVoteWeightFunc func(v *VoteBucket) *big.Int
)

// WithMuteHeight sets the mute height
func WithMuteHeight(height uint64) IndexerOption {
	return func(s *Indexer) {
		s.muteHeight = height
	}
}

// EnableTimestamped enables timestamped
func EnableTimestamped() IndexerOption {
	return func(s *Indexer) {
		s.timestamped = true
	}
}

// WithCalculateUnmutedVoteWeightFn sets the function to calculate unmuted vote weight
func WithCalculateUnmutedVoteWeightFn(f CalculateVoteWeightFunc) IndexerOption {
	return func(s *Indexer) {
		s.calculateVoteWeight = f
	}
}

// NewIndexer creates a new staking indexer
func NewIndexer(kvstore db.KVStore, contractAddr address.Address, startHeight uint64, blocksToDurationFn blocksDurationAtFn, opts ...IndexerOption) (*Indexer, error) {
	bucketNS := contractAddr.String() + "#" + stakingBucketNS
	ns := contractAddr.String() + "#" + stakingNS
	idx := &Indexer{
		common:           systemcontractindex.NewIndexerCommon(kvstore, ns, stakingHeightKey, contractAddr, startHeight),
		cache:            newCache(),
		blocksToDuration: blocksToDurationFn,
		bucketNS:         bucketNS,
		ns:               ns,
	}
	for _, opt := range opts {
		opt(idx)
	}
	if idx.calculateVoteWeight == nil {
		return nil, errors.New("calculateVoteWeight function is not set")
	}
	return idx, nil
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.common.Started() {
		return nil
	}
	return s.start(ctx)
}

func (s *Indexer) start(ctx context.Context) error {
	if err := s.common.Start(ctx); err != nil {
		return err
	}
	return s.cache.Load(s.common.KVStore(), s.ns, s.bucketNS)
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.common.Stop(ctx)
}

// CreateEventProcessor creates a new event processor
func (s *Indexer) CreateEventProcessor(ctx context.Context, handler staking.EventHandler) staking.EventProcessor {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	return newEventProcessor(
		s.common.ContractAddress(),
		blkCtx,
		handler,
		s.timestamped,
		s.muteHeight > 0 && blkCtx.BlockHeight >= s.muteHeight,
	)
}

// CreateMemoryEventHandler creates a new memory event handler
func (s *Indexer) CreateMemoryEventHandler(ctx context.Context) staking.CachedEventHandler {
	return newCachedEventHandler(newEventHandler(s.bucketNS, newWrappedCache(s.cache)), s.common.Height())
}

// LoadStakeView loads the contract stake view from state reader
func (s *Indexer) LoadStakeView(ctx context.Context, sr protocol.StateReader) (staking.ContractStakeView, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if !s.common.Started() {
		if err := s.start(ctx); err != nil {
			return nil, err
		}
	}
	srHeight, err := sr.Height()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get state reader height")
	}
	contractAddr := s.common.ContractAddress()
	csr := contractstaking.NewStateReader(sr)
	hasContractState := false
	mgr := NewCandidateVotesManager(s.ContractAddress())
	cur, err := mgr.Load(ctx, sr)
	switch errors.Cause(err) {
	case nil:
		hasContractState = true
	case state.ErrStateNotExist:
	default:
		return nil, errors.Wrapf(err, "failed to get num of buckets for contract %s", contractAddr)
	}
	cfg := &VoteViewConfig{
		ContractAddr: s.common.ContractAddress(),
		StartHeight:  s.common.StartHeight(),
		MuteHeight:   s.muteHeight,
		Timestamped:  s.timestamped,
	}
	// contract staking state have not been initialized in state reader, we need to read from index
	if !hasContractState && srHeight >= s.common.Height() {
		builder := NewEventHandlerFactory(s.bucketNS, s.common.KVStore(), s.calculateContractVoteWeight)
		processorBuilder := newEventProcessorBuilder(s.common.ContractAddress(), s.timestamped, s.muteHeight)
		mgr := NewCandidateVotesManager(s.ContractAddress())
		return NewVoteView(cfg, s.common.Height(), s.createCandidateVotes(s.cache.buckets), builder, processorBuilder, s, mgr, s.calculateContractVoteWeight), nil
	}
	// otherwise, we need to read from state reader
	cache := NewContractBucketCache(s.common.ContractAddress(), csr)
	builder := NewContractEventHandlerFactory(csr, s.calculateContractVoteWeight)
	processorBuilder := newEventProcessorBuilder(s.common.ContractAddress(), s.timestamped, s.muteHeight)
	vv := NewVoteView(cfg, srHeight, cur, builder, processorBuilder, cache, mgr, s.calculateContractVoteWeight)
	return vv, nil
}

// ContractStakingBuckets returns all the contract staking buckets
func (s *Indexer) ContractStakingBuckets() (uint64, map[uint64]*Bucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	idxs := s.cache.BucketIdxs()
	bkts := s.cache.Buckets(idxs)
	res := make(map[uint64]*Bucket)
	for i, id := range idxs {
		res[id] = bkts[i]
	}
	return s.common.Height(), res, nil
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.common.Height(), nil
}

// StartHeight returns the start height of the indexer
func (s *Indexer) StartHeight() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.common.StartHeight()
}

// ContractAddress returns the contract address
func (s *Indexer) ContractAddress() address.Address {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.common.ContractAddress()
}

// Buckets returns the buckets
func (s *Indexer) Buckets(height uint64) ([]*VoteBucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if unstart, err := s.checkHeight(height); err != nil {
		return nil, err
	} else if unstart {
		return nil, nil
	}
	idxs := s.cache.BucketIdxs()
	bkts := s.cache.Buckets(idxs)
	vbs := batchAssembleVoteBucket(idxs, bkts, s.common.ContractAddress().String(), s.genBlockDurationFn(height))
	return vbs, nil
}

// Bucket returns the bucket
func (s *Indexer) Bucket(id uint64, height uint64) (*VoteBucket, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if unstart, err := s.checkHeight(height); err != nil {
		return nil, false, err
	} else if unstart {
		return nil, false, nil
	}
	bkt := s.cache.Bucket(id)
	if bkt == nil {
		return nil, false, nil
	}
	vbs := assembleVoteBucket(id, bkt, s.common.ContractAddress().String(), s.genBlockDurationFn(height))
	return vbs, true, nil
}

// BucketsByIndices returns the buckets by indices
func (s *Indexer) BucketsByIndices(indices []uint64, height uint64) ([]*VoteBucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if unstart, err := s.checkHeight(height); err != nil {
		return nil, err
	} else if unstart {
		return nil, nil
	}
	bkts := s.cache.Buckets(indices)
	vbs := batchAssembleVoteBucket(indices, bkts, s.common.ContractAddress().String(), s.genBlockDurationFn(height))
	return vbs, nil
}

// BucketsByCandidate returns the buckets by candidate
func (s *Indexer) BucketsByCandidate(candidate address.Address, height uint64) ([]*VoteBucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if unstart, err := s.checkHeight(height); err != nil {
		return nil, err
	} else if unstart {
		return nil, nil
	}
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
	vbs := batchAssembleVoteBucket(idxsFiltered, bktsFiltered, s.common.ContractAddress().String(), s.genBlockDurationFn(height))
	return vbs, nil
}

// TotalBucketCount returns the total bucket count including active and burnt buckets
func (s *Indexer) TotalBucketCount(height uint64) (uint64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if unstart, err := s.checkHeight(height); err != nil {
		return 0, err
	} else if unstart {
		return 0, nil
	}
	return s.cache.TotalBucketCount(), nil
}

// PutBlock puts a block into indexer
func (s *Indexer) PutBlock(ctx context.Context, blk *block.Block) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// check block continuity
	expect := s.common.ExpectedHeight()
	if blk.Height() > expect {
		return errors.Errorf("invalid block height %d, expect %d", blk.Height(), expect)
	} else if blk.Height() < expect {
		log.L().Debug("indexer skip block", zap.Uint64("height", blk.Height()), zap.Uint64("expect", expect))
		return nil
	}
	// handle events of block
	muted := s.muteHeight > 0 && blk.Height() >= s.muteHeight
	handler := newEventHandler(s.bucketNS, newWrappedCache(s.cache))
	processor := newEventProcessor(
		s.common.ContractAddress(),
		protocol.MustGetBlockCtx(ctx),
		handler,
		s.timestamped,
		muted,
	)
	if err := processor.ProcessReceipts(ctx, blk.Receipts...); err != nil {
		return err
	}
	// commit
	return s.commit(ctx, handler, blk.Height())
}

func (s *Indexer) commit(ctx context.Context, handler *eventHandler, height uint64) error {
	delta, dirty := handler.Finalize()
	// update db
	if err := s.common.Commit(height, delta); err != nil {
		return err
	}
	cache, err := dirty.Commit(ctx, s.common.ContractAddress(), s.timestamped, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to commit dirty cache at height %d", height)
	}
	base, ok := cache.(*base)
	if !ok {
		return errors.Errorf("unexpected cache type %T, expect *base", dirty)
	}
	// update cache
	s.cache = base
	return nil
}

func (s *Indexer) checkHeight(height uint64) (unstart bool, err error) {
	if height < s.common.StartHeight() {
		return true, nil
	}
	// means latest height
	if height == 0 {
		return false, nil
	}
	tipHeight := s.common.Height()
	if height > tipHeight {
		return false, errors.Errorf("invalid block height %d, expect %d", height, tipHeight)
	}
	return false, nil
}

func (s *Indexer) genBlockDurationFn(view uint64) blocksDurationFn {
	return func(start uint64, end uint64) time.Duration {
		return s.blocksToDuration(start, end, view)
	}
}

func (s *Indexer) createCandidateVotes(bkts map[uint64]*Bucket) CandidateVotes {
	return AggregateCandidateVotes(bkts, func(b *contractstaking.Bucket) *big.Int {
		return s.calculateContractVoteWeight(b, s.common.Height())
	})
}

func (s *Indexer) calculateContractVoteWeight(b *Bucket, height uint64) *big.Int {
	vb := assembleVoteBucket(0, b, s.common.ContractAddress().String(), s.genBlockDurationFn(height))
	return s.calculateVoteWeight(vb)
}

// AggregateCandidateVotes aggregates the votes for each candidate from the given buckets
func AggregateCandidateVotes(bkts map[uint64]*Bucket, calculateUnmutedVoteWeight CalculateUnmutedVoteWeightFn) CandidateVotes {
	res := newCandidateVotes()
	for _, bkt := range bkts {
		if bkt.Muted || bkt.UnstakedAt < maxStakingNumber {
			continue
		}
		votes := calculateUnmutedVoteWeight(bkt)
		res.Add(bkt.Candidate.String(), bkt.StakedAmount, votes)
	}
	return res
}
