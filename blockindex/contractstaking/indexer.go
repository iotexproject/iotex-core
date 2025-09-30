// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"math/big"
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
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/systemcontractindex/stakingindex"
)

const (
	maxBlockNumber uint64 = staking.MaxDurationNumber
)

type (
	// Indexer is the contract staking indexer
	// Main functions:
	// 		1. handle contract staking contract events when new block comes to generate index data
	// 		2. provide query interface for contract staking index data
	Indexer struct {
		kvstore      db.KVStore            // persistent storage, used to initialize index cache at startup
		cache        *contractStakingCache // in-memory index for clean data, used to query index data
		config       Config                // indexer config
		height       uint64
		mu           sync.RWMutex
		contractAddr address.Address
		lifecycle.Readiness
	}

	// Config is the config for contract staking indexer
	Config struct {
		ContractAddress      string // stake contract ContractAddress
		ContractDeployHeight uint64 // height of the contract deployment
		// TODO: move calculateVoteWeightFunc out of config
		CalculateVoteWeight calculateVoteWeightFunc // calculate vote weight function
		BlocksToDuration    blocksDurationAtFn      // function to calculate duration from block range
	}

	calculateVoteWeightFunc func(v *Bucket) *big.Int
	blocksDurationFn        func(start uint64, end uint64) time.Duration
	blocksDurationAtFn      func(start uint64, end uint64, viewAt uint64) time.Duration
)

// NewContractStakingIndexer creates a new contract staking indexer
func NewContractStakingIndexer(kvStore db.KVStore, config Config) (*Indexer, error) {
	if kvStore == nil {
		return nil, errors.New("kv store is nil")
	}
	contractAddr, err := address.FromString(config.ContractAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid contract address %s", config.ContractAddress)
	}
	if config.CalculateVoteWeight == nil {
		return nil, errors.New("calculate vote weight function is nil")
	}
	return &Indexer{
		kvstore:      kvStore,
		cache:        newContractStakingCache(),
		config:       config,
		contractAddr: contractAddr,
	}, nil
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	if s.IsReady() {
		return nil
	}
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadFromDB(); err != nil {
		return err
	}
	s.TurnOn()
	return nil
}

// CreateEventProcessor creates a new event processor for contract staking
func (s *Indexer) CreateEventProcessor(ctx context.Context, handler staking.EventHandler) staking.EventProcessor {
	return newContractStakingEventProcessor(
		s.contractAddr,
		handler,
	)
}

// LoadStakeView loads the contract stake view
func (s *Indexer) LoadStakeView(ctx context.Context, sr protocol.StateReader) (staking.ContractStakeView, error) {
	if !s.IsReady() {
		return nil, errors.New("indexer not started")
	}
	featureCtx, ok := protocol.GetFeatureCtx(ctx)
	if ok && !featureCtx.StoreVoteOfNFTBucketIntoView {
		return nil, nil
	}
	srHeight, err := sr.Height()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get state reader height")
	}
	if s.config.ContractDeployHeight <= srHeight && srHeight != s.height {
		return nil, errors.New("state reader height does not match indexer height")
	}
	ids, typs, infos := s.cache.Buckets()
	buckets := make(map[uint64]*contractstaking.Bucket)
	for i, id := range ids {
		buckets[id] = assembleContractBucket(infos[i], typs[i])
	}
	cur := stakingindex.AggregateCandidateVotes(buckets, func(b *contractstaking.Bucket) *big.Int {
		return s.calculateUnmutedVoteWeightAt(b, s.height)
	})
	processorBuilder := newEventProcessorBuilder(s.contractAddr)
	cfg := &stakingindex.VoteViewConfig{ContractAddr: s.contractAddr}
	mgr := stakingindex.NewCandidateVotesManager(s.ContractAddress())
	return stakingindex.NewVoteView(cfg, s.height, cur, processorBuilder, mgr, s.calculateUnmutedVoteWeightAt), nil
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	if !s.IsReady() {
		return nil
	}
	if err := s.kvstore.Stop(ctx); err != nil {
		return err
	}
	s.cache = newContractStakingCache()
	s.TurnOff()
	return nil
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.height < s.config.ContractDeployHeight {
		return s.config.ContractDeployHeight - 1, nil
	}
	return s.height, nil
}

// StartHeight returns the start height of the indexer
func (s *Indexer) StartHeight() uint64 {
	return s.config.ContractDeployHeight
}

// ContractAddress returns the contract address
func (s *Indexer) ContractAddress() address.Address {
	return s.contractAddr
}

// CandidateVotes returns the candidate votes
func (s *Indexer) CandidateVotes(ctx context.Context, candidate address.Address, height uint64) (*big.Int, error) {
	if s.isIgnored(height) {
		return big.NewInt(0), nil
	}
	if err := s.validateHeight(height); err != nil {
		return nil, err
	}
	fn := s.genBlockDurationFn(height)
	s.mu.RLock()
	ids, types, infos := s.cache.BucketsByCandidate(candidate)
	s.mu.RUnlock()
	if len(types) != len(infos) || len(types) != len(ids) {
		return nil, errors.New("inconsistent bucket data")
	}
	if len(ids) == 0 {
		return big.NewInt(0), nil
	}
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	votes := big.NewInt(0)
	for i, id := range ids {
		bi := infos[i]
		if bi == nil || bi.UnstakedAt != maxBlockNumber {
			continue
		}
		if featureCtx.FixContractStakingWeightedVotes {
			votes.Add(votes, s.config.CalculateVoteWeight(assembleBucket(id, bi, types[i], s.config.ContractAddress, fn)))
		} else {
			votes.Add(votes, types[i].Amount)
		}
	}

	return votes, nil
}

func (s *Indexer) genBlockDurationFn(height uint64) blocksDurationFn {
	return func(start, end uint64) time.Duration {
		return s.config.BlocksToDuration(start, end, height)
	}
}

// DeductBucket deducts the bucket by address and id
func (s *Indexer) DeductBucket(addr address.Address, id uint64) (*contractstaking.Bucket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.contractAddr.String() != addr.String() {
		return nil, errors.Wrapf(contractstaking.ErrBucketNotExist, "contract address not match: %s vs %s", s.contractAddr.String(), addr.String())
	}
	bt, bi := s.cache.Bucket(id)
	if bt == nil || bi == nil {
		return nil, errors.Wrapf(contractstaking.ErrBucketNotExist, "bucket %d not found", id)
	}
	return assembleContractBucket(bi, bt), nil
}

// Buckets returns the buckets
func (s *Indexer) Buckets(height uint64) ([]*Bucket, error) {
	if s.isIgnored(height) {
		return []*Bucket{}, nil
	}
	if err := s.validateHeight(height); err != nil {
		return nil, err
	}
	fn := s.genBlockDurationFn(height)
	s.mu.RLock()
	ids, types, infos := s.cache.Buckets()
	s.mu.RUnlock()
	if len(types) != len(infos) || len(types) != len(ids) {
		return nil, errors.New("inconsistent bucket data")
	}
	if len(ids) == 0 {
		return []*Bucket{}, nil
	}

	buckets := make([]*Bucket, 0, len(ids))
	for i, id := range ids {
		bucket := assembleBucket(id, infos[i], types[i], s.config.ContractAddress, fn)
		if bucket != nil {
			buckets = append(buckets, bucket)
		}
	}

	return buckets, nil
}

// Bucket returns the bucket
func (s *Indexer) Bucket(id uint64, height uint64) (*Bucket, bool, error) {
	if s.isIgnored(height) {
		return nil, false, nil
	}
	if err := s.validateHeight(height); err != nil {
		return nil, false, err
	}
	fn := s.genBlockDurationFn(height)
	s.mu.RLock()
	bt, bi := s.cache.Bucket(id)
	s.mu.RUnlock()
	if bt == nil || bi == nil {
		return nil, false, nil
	}

	return assembleBucket(id, bi, bt, s.config.ContractAddress, fn), true, nil
}

// BucketsByIndices returns the buckets by indices
func (s *Indexer) BucketsByIndices(indices []uint64, height uint64) ([]*Bucket, error) {
	if s.isIgnored(height) {
		return []*Bucket{}, nil
	}
	if err := s.validateHeight(height); err != nil {
		return nil, err
	}
	fn := s.genBlockDurationFn(height)
	s.mu.RLock()
	ts, infos := s.cache.BucketsByIndices(indices)
	s.mu.RUnlock()
	if len(ts) != len(infos) || len(ts) != len(indices) {
		return nil, errors.New("inconsistent bucket data")
	}
	buckets := make([]*Bucket, 0, len(ts))
	for i, id := range indices {
		if ts[i] == nil || infos[i] == nil {
			continue
		}
		bucket := assembleBucket(id, infos[i], ts[i], s.config.ContractAddress, fn)
		if bucket != nil {
			buckets = append(buckets, bucket)
		}
	}

	return buckets, nil
}

// BucketsByCandidate returns the buckets by candidate
func (s *Indexer) BucketsByCandidate(candidate address.Address, height uint64) ([]*Bucket, error) {
	if s.isIgnored(height) {
		return []*Bucket{}, nil
	}
	if err := s.validateHeight(height); err != nil {
		return nil, err
	}
	fn := s.genBlockDurationFn(height)
	s.mu.RLock()
	ids, types, infos := s.cache.BucketsByCandidate(candidate)
	s.mu.RUnlock()
	buckets := make([]*Bucket, 0, len(infos))
	for i, id := range ids {
		info := infos[i]
		bucket := assembleBucket(id, info, types[i], s.config.ContractAddress, fn)
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

// TotalBucketCount returns the total bucket count including active and burnt buckets
func (s *Indexer) TotalBucketCount(height uint64) (uint64, error) {
	if s.isIgnored(height) {
		return 0, nil
	}
	if err := s.validateHeight(height); err != nil {
		return 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache.TotalBucketCount(), nil
}

// BucketTypes returns the active bucket types
func (s *Indexer) BucketTypes(height uint64) ([]*BucketType, error) {
	if s.isIgnored(height) {
		return []*BucketType{}, nil
	}
	if err := s.validateHeight(height); err != nil {
		return nil, err
	}
	s.mu.RLock()
	btMap := s.cache.ActiveBucketTypes()
	s.mu.RUnlock()
	bts := make([]*BucketType, 0, len(btMap))
	for _, bt := range btMap {
		bts = append(bts, bt)
	}
	return bts, nil
}

// ContractStakingBuckets returns all contract staking buckets
func (s *Indexer) ContractStakingBuckets() (uint64, map[uint64]*contractstaking.Bucket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, typs, infos := s.cache.Buckets()
	res := make(map[uint64]*contractstaking.Bucket)
	for i, id := range ids {
		res[id] = assembleContractBucket(infos[i], typs[i])
	}
	return s.height, res, nil
}

// PutBlock puts a block into indexer
func (s *Indexer) PutBlock(ctx context.Context, blk *block.Block) error {
	if blk.Height() < s.config.ContractDeployHeight {
		return nil
	}
	s.mu.RLock()
	expectHeight := s.height + 1
	cache := newWrappedCache(s.cache)
	s.mu.RUnlock()
	if expectHeight < s.config.ContractDeployHeight {
		expectHeight = s.config.ContractDeployHeight
	}
	if blk.Height() < expectHeight {
		return errors.Errorf("block height %d has been indexed, expect %d", blk.Height(), expectHeight)
	}
	if blk.Height() > expectHeight {
		return errors.Errorf("invalid block height %d, expect %d", blk.Height(), expectHeight)
	}
	handler := newContractStakingDirty(cache)
	processor := newContractStakingEventProcessor(s.contractAddr, handler)
	if err := processor.ProcessReceipts(ctx, blk.Receipts...); err != nil {
		return errors.Wrapf(err, "failed to handle receipts at height %d", blk.Height())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// commit the result
	if err := s.commit(ctx, handler, blk.Height()); err != nil {
		return errors.Wrapf(err, "failed to commit block %d", blk.Height())
	}
	return nil
}

// IndexerAt returns the contract staking indexer at a specific height
func (s *Indexer) IndexerAt(sr protocol.StateReader) staking.ContractStakingIndexer {
	epb := newEventProcessorBuilder(s.contractAddr)
	return stakingindex.NewHistoryIndexer(sr, s.contractAddr, s.config.ContractDeployHeight, epb, s.calculateUnmutedVoteWeightAt)
}

func (s *Indexer) commit(ctx context.Context, handler *contractStakingDirty, height uint64) error {
	batch, delta := handler.Finalize()
	cache, err := delta.Commit(ctx, s.contractAddr, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to commit delta")
	}
	base, ok := cache.(*contractStakingCache)
	if !ok {
		return errors.New("invalid cache type of base")
	}
	// update db
	batch.Put(_StakingNS, _stakingHeightKey, byteutil.Uint64ToBytesBigEndian(height), "failed to put height")
	if err := s.kvstore.WriteBatch(batch); err != nil {
		s.cache = newContractStakingCache()
		return s.loadFromDB()
	}
	s.height = height
	s.cache = base
	return nil
}

func (s *Indexer) loadFromDB() error {
	// load height
	var height uint64
	h, err := s.kvstore.Get(_StakingNS, _stakingHeightKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
		height = 0
	} else {
		height = byteutil.BytesToUint64BigEndian(h)

	}
	s.height = height
	return s.cache.LoadFromDB(s.kvstore)
}

// isIgnored returns true if before cotractDeployHeight.
// it aims to be compatible with blocks between feature hard-fork and contract deployed
// read interface should return empty result instead of invalid height error if it returns true
func (s *Indexer) isIgnored(height uint64) bool {
	return height < s.config.ContractDeployHeight
}

func (s *Indexer) validateHeight(height uint64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// means latest height
	if height == 0 {
		return nil
	}
	// Currently, historical block data query is not supported.
	// However, the latest data is actually returned when querying historical block data, for the following reasons:
	//	1. to maintain compatibility with the current code's invocation of ActiveCandidate
	//	2. to cause consensus errors when the indexer is lagging behind
	if height > s.height {
		return errors.Wrapf(ErrInvalidHeight, "expected %d, actual %d", s.height, height)
	}
	return nil
}

func (s *Indexer) calculateUnmutedVoteWeightAt(b *contractstaking.Bucket, height uint64) *big.Int {
	vb := contractBucketToVoteBucket(0, b, s.contractAddr.String(), s.genBlockDurationFn(height))
	return s.config.CalculateVoteWeight(vb)
}
