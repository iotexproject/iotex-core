package stakingindex

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/abiutil"
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
		ContractAddress() string
		Buckets(height uint64) ([]*VoteBucket, error)
		Bucket(id uint64, height uint64) (*VoteBucket, bool, error)
		BucketsByIndices(indices []uint64, height uint64) ([]*VoteBucket, error)
		BucketsByCandidate(candidate address.Address, height uint64) ([]*VoteBucket, error)
		TotalBucketCount(height uint64) (uint64, error)
		PutBlock(ctx context.Context, blk *block.Block) error
	}
	stakingEventHandler interface {
		HandleStakedEvent(event *abiutil.EventParam) error
		HandleLockedEvent(event *abiutil.EventParam) error
		HandleUnlockedEvent(event *abiutil.EventParam) error
		HandleUnstakedEvent(event *abiutil.EventParam) error
		HandleDelegateChangedEvent(event *abiutil.EventParam) error
		HandleWithdrawalEvent(event *abiutil.EventParam) error
		HandleTransferEvent(event *abiutil.EventParam) error
		HandleMergedEvent(event *abiutil.EventParam) error
		HandleBucketExpandedEvent(event *abiutil.EventParam) error
		HandleDonatedEvent(event *abiutil.EventParam) error
		Finalize() (batch.KVStoreBatch, *cache)
	}
	// Indexer is the staking indexer
	Indexer struct {
		common           *systemcontractindex.IndexerCommon
		cache            *cache // in-memory cache, used to query index data
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
func NewIndexer(kvstore db.KVStore, contractAddr string, startHeight uint64, blocksToDurationFn blocksDurationAtFn, opts ...IndexerOption) *Indexer {
	bucketNS := contractAddr + "#" + stakingBucketNS
	ns := contractAddr + "#" + stakingNS
	idx := &Indexer{
		common:           systemcontractindex.NewIndexerCommon(kvstore, ns, stakingHeightKey, contractAddr, startHeight),
		cache:            newCache(ns, bucketNS),
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

// StartHeight returns the start height of the indexer
func (s *Indexer) StartHeight() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.common.StartHeight()
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
	vbs := batchAssembleVoteBucket(idxs, bkts, s.common.ContractAddress(), s.genBlockDurationFn(height))
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
	vbs := assembleVoteBucket(id, bkt, s.common.ContractAddress(), s.genBlockDurationFn(height))
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
	vbs := batchAssembleVoteBucket(indices, bkts, s.common.ContractAddress(), s.genBlockDurationFn(height))
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
	vbs := batchAssembleVoteBucket(idxsFiltered, bktsFiltered, s.common.ContractAddress(), s.genBlockDurationFn(height))
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
	var handler stakingEventHandler
	eventHandler := newEventHandler(s.bucketNS, s.cache.Copy(), blk, s.timestamped)
	if s.muteHeight > 0 && blk.Height() >= s.muteHeight {
		handler = newEventMuteHandler(eventHandler)
	} else {
		handler = eventHandler
	}
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != s.common.ContractAddress() {
				continue
			}
			if err := s.handleEvent(ctx, handler, blk, log); err != nil {
				return err
			}
		}
	}
	// commit
	return s.commit(handler, blk.Height())
}

func (s *Indexer) handleEvent(ctx context.Context, eh stakingEventHandler, blk *block.Block, actLog *action.Log) error {
	// get event abi
	abiEvent, err := StakingContractABI.EventByID(common.Hash(actLog.Topics[0]))
	if err != nil {
		return errors.Wrapf(err, "get event abi from topic %v failed", actLog.Topics[0])
	}

	// unpack event data
	event, err := abiutil.UnpackEventParam(abiEvent, actLog)
	if err != nil {
		return err
	}
	log.L().Debug("handle staking event", zap.String("event", abiEvent.Name), zap.Any("event", event))
	// handle different kinds of event
	switch abiEvent.Name {
	case "Staked":
		return eh.HandleStakedEvent(event)
	case "Locked":
		return eh.HandleLockedEvent(event)
	case "Unlocked":
		return eh.HandleUnlockedEvent(event)
	case "Unstaked":
		return eh.HandleUnstakedEvent(event)
	case "Merged":
		return eh.HandleMergedEvent(event)
	case "BucketExpanded":
		return eh.HandleBucketExpandedEvent(event)
	case "DelegateChanged":
		return eh.HandleDelegateChangedEvent(event)
	case "Withdrawal":
		return eh.HandleWithdrawalEvent(event)
	case "Donated":
		return eh.HandleDonatedEvent(event)
	case "Transfer":
		return eh.HandleTransferEvent(event)
	case "Approval", "ApprovalForAll", "OwnershipTransferred", "Paused", "Unpaused", "BeneficiaryChanged",
		"Migrated":
		// not require handling events
		return nil
	default:
		return errors.Errorf("unknown event name %s", abiEvent.Name)
	}
}

func (s *Indexer) commit(handler stakingEventHandler, height uint64) error {
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

func (s *Indexer) genBlockDurationFn(view uint64) blocksDurationFn {
	return func(start uint64, end uint64) time.Duration {
		return s.blocksToDuration(start, end, view)
	}
}
