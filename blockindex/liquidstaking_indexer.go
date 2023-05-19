// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	// StakingContractAddress  is the address of system staking contract
	// TODO (iip-13): replace with the real system staking contract address
	StakingContractAddress = "io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd"
	// StakingContractABI is the ABI of system staking contract
	StakingContractABI = `[
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				}
			],
			"name": "AmountIncreased",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "owner",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "approved",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Approval",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "owner",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "operator",
					"type": "address"
				},
				{
					"indexed": false,
					"internalType": "bool",
					"name": "approved",
					"type": "bool"
				}
			],
			"name": "ApprovalForAll",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "BucketTypeActivated",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "BucketTypeDeactivated",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "address",
					"name": "newDelegate",
					"type": "address"
				}
			],
			"name": "DelegateChanged",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "DurationExtended",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "Locked",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "uint256[]",
					"name": "tokenIds",
					"type": "uint256[]"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "Merged",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "previousOwner",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "newOwner",
					"type": "address"
				}
			],
			"name": "OwnershipTransferred",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "address",
					"name": "account",
					"type": "address"
				}
			],
			"name": "Paused",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "address",
					"name": "delegate",
					"type": "address"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "Staked",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "from",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "to",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Transfer",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Unlocked",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "address",
					"name": "account",
					"type": "address"
				}
			],
			"name": "Unpaused",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Unstaked",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "recipient",
					"type": "address"
				}
			],
			"name": "Withdrawal",
			"type": "event"
		}
	]`

	// bucket related namespace in db
	_StakingBucketInfoNS = "sbi"
	_StakingBucketTypeNS = "sbt"
	_StakingNS           = "s"
)

type (
	// ContractStakingIndexer is the interface of contract staking indexer
	ContractStakingIndexer interface {
		blockdao.BlockIndexer

		// CandidateVotes returns the total votes of a candidate
		CandidateVotes(candidate address.Address) *big.Int
		// Buckets returns all existed buckets
		Buckets() ([]*ContractStakingBucket, error)
		// Bucket returns the bucket by id
		Bucket(id uint64) (*ContractStakingBucket, error)
		// BucketsByIndices returns buckets by indices
		BucketsByIndices(indices []uint64) ([]*ContractStakingBucket, error)
		// BucketsByCandidate returns buckets by candidate
		BucketsByCandidate(candidate address.Address) ([]*ContractStakingBucket, error)
		// TotalBucketCount returns the total bucket count including burned buckets
		TotalBucketCount() uint64
		// BucketTypes returns all active bucket types
		BucketTypes() ([]*ContractStakingBucketType, error)
	}

	// contractStakingIndexer is the implementation of ContractStakingIndexer
	// Main functions:
	// 		1. handle contract staking contract events when new block comes to generate index data
	// 		2. provide query interface for contract staking index data
	// Generate index data flow:
	// 		block comes -> new dirty cache -> handle contract events -> update dirty cache -> merge dirty to clean cache
	// Main Object:
	// 		kvstore: persistent storage, used to initialize index cache at startup
	// 		cache: in-memory index for clean data, used to query index data
	//      dirty: the cache to update during event processing, will be merged to clean cache after all events are processed. If errors occur during event processing, dirty cache will be discarded.
	contractStakingIndexer struct {
		kvstore db.KVStore                  // persistent storage
		cache   contractStakingCacheManager // in-memory index for clean data
		mutex   sync.RWMutex                // mutex for multiple reading to cache
	}
)

var (
	_stakingInterface           abi.ABI
	_stakingHeightKey           = []byte("shk")
	_stakingTotalBucketCountKey = []byte("stbck")

	errBucketTypeNotExist = errors.New("bucket type does not exist")

	// ErrBucketInfoNotExist is the error when bucket does not exist
	ErrBucketInfoNotExist = errors.New("bucket info does not exist")
)

func init() {
	var err error
	_stakingInterface, err = abi.JSON(strings.NewReader(StakingContractABI))
	if err != nil {
		panic(err)
	}
}

// NewContractStakingIndexer creates a new contract staking indexer
func NewContractStakingIndexer(kvStore db.KVStore) ContractStakingIndexer {
	return &contractStakingIndexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(),
	}
}

// Start starts the indexer
func (s *contractStakingIndexer) Start(ctx context.Context) error {
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	return s.loadCache()
}

// Stop stops the indexer
func (s *contractStakingIndexer) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.kvstore.Stop(ctx); err != nil {
		return err
	}
	s.cache = newContractStakingCache()
	return nil
}

// PutBlock puts a block into indexer
func (s *contractStakingIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	// new dirty cache for this block
	// it's not necessary to use thread safe cache here, because only one thread will call this function
	// and no update to cache will happen before dirty merge to clean
	dirty := newContractStakingDirty(s.cache)
	dirty.putHeight(blk.Height())

	// handle events of block
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != StakingContractAddress {
				continue
			}
			if err := s.handleEvent(ctx, dirty, blk, log); err != nil {
				return err
			}
		}
	}

	// commit dirty cache
	return s.commit(dirty)
}

// DeleteTipBlock deletes the tip block from indexer
func (s *contractStakingIndexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

// Height returns the tip block height
func (s *contractStakingIndexer) Height() (uint64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.cache.getHeight(), nil
}

// CandidateVotes returns the candidate votes
func (s *contractStakingIndexer) CandidateVotes(candidate address.Address) *big.Int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.cache.getCandidateVotes(candidate)
}

// Buckets returns the buckets
func (s *contractStakingIndexer) Buckets() ([]*ContractStakingBucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vbs := []*ContractStakingBucket{}
	for id, bi := range s.cache.getAllBucketInfo() {
		bt := s.cache.mustGetBucketType(bi.TypeIndex)
		vb, err := s.assembleContractStakingBucket(id, bi, bt)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

// Bucket returns the bucket
func (s *contractStakingIndexer) Bucket(id uint64) (*ContractStakingBucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.generateBucket(id)
}

// BucketsByIndices returns the buckets by indices
func (s *contractStakingIndexer) BucketsByIndices(indices []uint64) ([]*ContractStakingBucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vbs := make([]*ContractStakingBucket, 0, len(indices))
	for _, id := range indices {
		vb, err := s.generateBucket(id)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

func (s *contractStakingIndexer) BucketsByCandidate(candidate address.Address) ([]*ContractStakingBucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vbs := make([]*ContractStakingBucket, 0)
	for id := range s.cache.getBucketInfoByCandidate(candidate) {
		vb, err := s.generateBucket(id)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

func (s *contractStakingIndexer) TotalBucketCount() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.cache.getTotalBucketCount()
}

func (s *contractStakingIndexer) BucketTypes() ([]*ContractStakingBucketType, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	btMap := s.cache.getActiveBucketType()
	bts := make([]*ContractStakingBucketType, 0, len(btMap))
	for _, bt := range btMap {
		bts = append(bts, bt)
	}
	return bts, nil
}

func (s *contractStakingIndexer) generateBucket(id uint64) (*ContractStakingBucket, error) {
	bi, ok := s.cache.getBucketInfo(id)
	if !ok {
		return nil, errors.Wrapf(ErrBucketInfoNotExist, "id %d", id)
	}
	bt := s.cache.mustGetBucketType(bi.TypeIndex)
	return s.assembleContractStakingBucket(id, bi, bt)
}

func (s *contractStakingIndexer) handleEvent(ctx context.Context, dirty *contractStakingDirty, blk *block.Block, log *action.Log) error {
	// get event abi
	abiEvent, err := _stakingInterface.EventByID(common.Hash(log.Topics[0]))
	if err != nil {
		return errors.Wrapf(err, "get event abi from topic %v failed", log.Topics[0])
	}

	// unpack event data
	event, err := unpackEventParam(abiEvent, log)
	if err != nil {
		return err
	}

	// handle different kinds of event
	switch abiEvent.Name {
	case "BucketTypeActivated":
		return dirty.handleBucketTypeActivatedEvent(event, blk.Height())
	case "BucketTypeDeactivated":
		return dirty.handleBucketTypeDeactivatedEvent(event, blk.Height())
	case "Staked":
		return dirty.handleStakedEvent(event, blk.Height())
	case "Locked":
		return dirty.handleLockedEvent(event)
	case "Unlocked":
		return dirty.handleUnlockedEvent(event, blk.Height())
	case "Unstaked":
		return dirty.handleUnstakedEvent(event, blk.Height())
	case "Merged":
		return dirty.handleMergedEvent(event)
	case "DurationExtended":
		return dirty.handleDurationExtendedEvent(event)
	case "AmountIncreased":
		return dirty.handleAmountIncreasedEvent(event)
	case "DelegateChanged":
		return dirty.handleDelegateChangedEvent(event)
	case "Withdrawal":
		return dirty.handleWithdrawalEvent(event)
	case "Transfer":
		return dirty.handleTransferEvent(event)
	case "Approval", "ApprovalForAll", "OwnershipTransferred", "Paused", "Unpaused":
		// not require handling events
		return nil
	default:
		return errors.Errorf("unknown event name %s", abiEvent.Name)
	}
}

func (s *contractStakingIndexer) reloadCache() error {
	s.cache = newContractStakingCache()
	return s.loadCache()
}

func (s *contractStakingIndexer) loadCache() error {
	delta := newContractStakingDelta()
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
	delta.putHeight(height)

	// load total bucket count
	var totalBucketCount uint64
	tbc, err := s.kvstore.Get(_StakingNS, _stakingTotalBucketCountKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
	} else {
		totalBucketCount = byteutil.BytesToUint64BigEndian(tbc)
	}
	delta.putTotalBucketCount(totalBucketCount)

	// load bucket info
	ks, vs, err := s.kvstore.Filter(_StakingBucketInfoNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b ContractStakingBucketInfo
		if err := b.Deserialize(vs[i]); err != nil {
			return err
		}
		delta.addBucketInfo(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}

	// load bucket type
	ks, vs, err = s.kvstore.Filter(_StakingBucketTypeNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b ContractStakingBucketType
		if err := b.Deserialize(vs[i]); err != nil {
			return err
		}
		delta.addBucketType(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}
	return s.cache.merge(delta)
}

func (s *contractStakingIndexer) commit(dirty *contractStakingDirty) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	batch, delta := dirty.finalize()
	if err := s.cache.merge(delta); err != nil {
		s.reloadCache()
		return err
	}
	if err := s.kvstore.WriteBatch(batch); err != nil {
		s.reloadCache()
		return err
	}
	return nil
}

func (s *contractStakingIndexer) assembleContractStakingBucket(token uint64, bi *ContractStakingBucketInfo, bt *ContractStakingBucketType) (*ContractStakingBucket, error) {
	vb := ContractStakingBucket{
		Index:                     token,
		StakedAmount:              bt.Amount,
		StakedDurationBlockNumber: bt.Duration,
		CreateBlockHeight:         bi.CreatedAt,
		StakeStartBlockHeight:     bi.CreatedAt,
		UnstakeStartBlockHeight:   bi.UnstakedAt,
		AutoStake:                 bi.UnlockedAt == maxBlockNumber,
		Candidate:                 bi.Delegate,
		Owner:                     bi.Owner,
		ContractAddress:           StakingContractAddress,
	}
	if bi.UnlockedAt != maxBlockNumber {
		vb.StakeStartBlockHeight = bi.UnlockedAt
	}
	return &vb, nil
}
