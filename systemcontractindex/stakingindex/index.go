package stakingindex

import (
	"context"
	"sync"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
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
	}
	// Indexer is the staking indexer
	Indexer struct {
		common           *systemcontractindex.IndexerCommon
		cache            *base // in-memory cache, used to query index data
		mutex            sync.RWMutex
		blocksToDuration blocksDurationAtFn // function to calculate duration from block range
		bucketNS         string
		ns               string
		muteHeight       uint64
		timestamped      bool
	}
	// IndexerOption is the option to create an indexer
	IndexerOption func(*Indexer)

	blocksDurationFn   func(start uint64, end uint64) time.Duration
	blocksDurationAtFn func(start uint64, end uint64, viewAt uint64) time.Duration
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

// NewIndexer creates a new staking indexer
func NewIndexer(kvstore db.KVStore, contractAddr address.Address, startHeight uint64, blocksToDurationFn blocksDurationAtFn, opts ...IndexerOption) *Indexer {
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
	return idx
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.common.Started() {
		return nil
	}
	if err := s.common.Start(ctx); err != nil {
		return err
	}
	return s.cache.Load(s.common.KVStore(), s.ns, s.bucketNS)
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if !s.common.Started() {
		return nil
	}
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

// LoadStakeView loads the contract stake view from state reader
func (s *Indexer) LoadStakeView(ctx context.Context, sr protocol.StateReader) (staking.ContractStakeView, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if !s.common.Started() {
		return nil, errors.New("indexer not started")
	}
	if protocol.MustGetFeatureCtx(ctx).StoreVoteOfNFTBucketIntoView {
		return &stakeView{
			cache:              s.cache.Clone(),
			height:             s.common.Height(),
			contractAddr:       s.common.ContractAddress(),
			muteHeight:         s.muteHeight,
			timestamped:        s.timestamped,
			startHeight:        s.common.StartHeight(),
			bucketNS:           s.bucketNS,
			genBlockDurationFn: s.genBlockDurationFn,
		}, nil
	}
	contractAddr := s.common.ContractAddress()
	ids, buckets, err := contractstaking.NewStateReader(sr).Buckets(contractAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get buckets for contract %s", contractAddr)
	}
	if len(ids) != len(buckets) {
		return nil, errors.Errorf("length of ids (%d) does not match length of buckets (%d)", len(ids), len(buckets))
	}
	cache := &base{}
	for i, b := range buckets {
		if b == nil {
			return nil, errors.New("bucket is nil")
		}
		b.IsTimestampBased = s.timestamped
		cache.PutBucket(ids[i], b)
	}
	return &stakeView{
		cache:              cache,
		height:             s.common.Height(),
		contractAddr:       s.common.ContractAddress(),
		muteHeight:         s.muteHeight,
		startHeight:        s.common.StartHeight(),
		timestamped:        s.timestamped,
		bucketNS:           s.bucketNS,
		genBlockDurationFn: s.genBlockDurationFn,
	}, nil
}

// StartHeight returns the start height of the indexer
func (s *Indexer) StartHeight() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.common.StartHeight()
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	height := s.common.Height()
	startHeight := s.common.StartHeight()
	if height < startHeight {
		return startHeight - 1, nil
	}
	return height, nil
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
	if blk.Height() < s.common.StartHeight() {
		return nil
	}
	// check block continuity
	expect := s.common.ExpectedHeight()
	if blk.Height() > expect {
		return errors.Errorf("invalid block height %d, expect %d", blk.Height(), expect)
	}
	if blk.Height() < expect {
		return errors.Errorf("block height %d has been indexed, expect %d", blk.Height(), expect)
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
