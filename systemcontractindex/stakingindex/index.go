package stakingindex

import (
	"context"
	"sync"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/systemcontractindex"
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
	// Indexer is the staking indexer
	Indexer struct {
		common        *systemcontractindex.IndexerCommon
		cache         *cache // in-memory cache, used to query index data
		mutex         sync.RWMutex
		blockInterval time.Duration
	}
)

// NewIndexer creates a new staking indexer
func NewIndexer(kvstore db.KVStore, contractAddr string, startHeight uint64, blockInterval time.Duration) *Indexer {
	return &Indexer{
		common:        systemcontractindex.NewIndexerCommon(kvstore, stakingNS, stakingHeightKey, contractAddr, startHeight),
		cache:         newCache(),
		blockInterval: blockInterval,
	}
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	if err := s.common.KVStore().Start(ctx); err != nil {
		return err
	}
	return s.cache.Load(s.common.KVStore())
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	return s.common.KVStore().Stop(ctx)
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.common.Height()
}

// StartHeight returns the start height of the indexer
func (s *Indexer) StartHeight() uint64 {
	return s.common.StartHeight()
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
	bkts := s.cache.BucketsByIndices(idxs)
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
	bkts := s.cache.BucketsByIndices(indices)
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
	idxs := s.cache.BucketIdxsByCandidate(candidate)
	bkts := s.cache.BucketsByIndices(idxs)
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
	// check block continuity
	if existed, err := s.common.BlockContinuity(blk.Height()); err != nil {
		return err
	} else if existed {
		return nil
	}
	// handle events of block
	handler := newEventHandler(s.cache.Copy())
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

func (s *Indexer) commit(handler *eventHandler, height uint64) error {
	delta, dirty := handler.Finalize()
	s.mutex.Lock()
	defer s.mutex.Unlock()
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
	tipHeight, err := s.common.Height()
	if err != nil {
		return false, err
	}
	if height > tipHeight {
		return false, errors.Errorf("invalid block height %d, expect %d", height, tipHeight)
	}
	return false, nil
}
