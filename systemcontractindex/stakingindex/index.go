package stakingindex

import (
	"context"
	"sync"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
		ContractAddress() string
		Buckets(height uint64) ([]*VoteBucket, error)
		Bucket(id uint64, height uint64) (*VoteBucket, bool, error)
		BucketsByIndices(indices []uint64, height uint64) ([]*VoteBucket, error)
		BucketsByCandidate(candidate address.Address, height uint64) ([]*VoteBucket, error)
		TotalBucketCount(height uint64) (uint64, error)
		PutBlock(ctx context.Context, blk *block.Block) error
		DeleteTipBlock(ctx context.Context, blk *block.Block) error
	}
	// Indexer is the staking indexer
	Indexer struct {
		common        *systemcontractindex.IndexerCommon
		cache         *cache // in-memory cache, used to query index data
		mutex         sync.RWMutex
		blockInterval time.Duration
		bucketNS      string
		ns            string
	}
)

// NewIndexer creates a new staking indexer
func NewIndexer(kvstore db.KVStore, contractAddr string, startHeight uint64, blockInterval time.Duration) *Indexer {
	bucketNS := contractAddr + "#" + stakingBucketNS
	ns := contractAddr + "#" + stakingNS
	return &Indexer{
		common:        systemcontractindex.NewIndexerCommon(kvstore, ns, stakingHeightKey, contractAddr, startHeight),
		cache:         newCache(ns, bucketNS),
		blockInterval: blockInterval,
		bucketNS:      bucketNS,
		ns:            ns,
	}
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := s.common.Start(ctx); err != nil {
		return err
	}
	return s.cache.Load(s.common.KVStore())
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.common.Stop(ctx)
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.common.Height(), nil
}

// ContractAddress returns the contract address
func (s *Indexer) ContractAddress() string {
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
	vbs := batchAssembleVoteBucket(idxs, bkts, s.common.ContractAddress(), s.blockInterval)
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
	vbs := assembleVoteBucket(id, bkt, s.common.ContractAddress(), s.blockInterval)
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
	vbs := batchAssembleVoteBucket(indices, bkts, s.common.ContractAddress(), s.blockInterval)
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
	vbs := batchAssembleVoteBucket(idxs, bkts, s.common.ContractAddress(), s.blockInterval)
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
	handler := newEventHandler(s.bucketNS, s.cache.Copy())
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != s.common.ContractAddress() {
				continue
			}
			if err := handler.HandleEvent(ctx, blk, log); err != nil {
				return err
			}
		}
	}
	// commit
	return s.commit(handler, blk.Height())
}

// DeleteTipBlock deletes the tip block from indexer
func (s *Indexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

func (s *Indexer) commit(handler *eventHandler, height uint64) error {
	delta, dirty := handler.Finalize()
	// update db
	if err := s.common.Commit(height, delta); err != nil {
		return err
	}
	// update cache
	s.cache = dirty
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
