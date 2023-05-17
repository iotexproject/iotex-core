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
	// LiquidStakingContractAddress  is the address of liquid staking contract
	// TODO (iip-13): replace with the real liquid staking contract address
	LiquidStakingContractAddress = "io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd"
	// LiquidStakingContractABI is the ABI of liquid staking contract
	LiquidStakingContractABI = `[
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
	_liquidStakingBucketInfoNS = "lsbInfo"
	_liquidStakingBucketTypeNS = "lsbType"
	_liquidStakingNS           = "lsns"
)

type (
	// LiquidStakingIndexer is the interface of liquid staking indexer
	LiquidStakingIndexer interface {
		blockdao.BlockIndexer

		CandidateVotes(candidate address.Address) *big.Int
		Buckets() ([]*Bucket, error)
		Bucket(id uint64) (*Bucket, error)
		BucketsByIndices(indices []uint64) ([]*Bucket, error)
		TotalBucketCount() uint64
		ActiveBucketTypes() (map[uint64]*BucketType, error)
	}

	// liquidStakingIndexer is the implementation of LiquidStakingIndexer
	// Main functions:
	// 		1. handle liquid staking contract events when new block comes to generate index data
	// 		2. provide query interface for liquid staking index data
	// Generate index data flow:
	// 		block comes -> new dirty cache -> handle contract events -> update dirty cache -> merge dirty to clean cache
	// Main Object:
	// 		kvstore: persistent storage, used to initialize index cache at startup
	// 		cache: in-memory index for clean data, used to query index data
	//      dirty: the cache to update during event processing, will be merged to clean cache after all events are processed. If errors occur during event processing, dirty cache will be discarded.
	liquidStakingIndexer struct {
		kvstore db.KVStore                      // persistent storage
		cache   *liquidStakingCacheThreadSafety // in-memory index for clean data
		mutex   sync.RWMutex                    // mutex for multiple reading to cache
	}
)

var (
	_liquidStakingInterface           abi.ABI
	_liquidStakingHeightKey           = []byte("lsHeight")
	_liquidStakingTotalBucketCountKey = []byte("lsTotalBucketCount")

	errBucketTypeNotExist = errors.New("bucket type does not exist")

	// ErrBucketInfoNotExist is the error when bucket does not exist
	ErrBucketInfoNotExist = errors.New("bucket info does not exist")
)

func init() {
	var err error
	_liquidStakingInterface, err = abi.JSON(strings.NewReader(LiquidStakingContractABI))
	if err != nil {
		panic(err)
	}
}

// NewLiquidStakingIndexer creates a new liquid staking indexer
func NewLiquidStakingIndexer(kvStore db.KVStore) LiquidStakingIndexer {
	return &liquidStakingIndexer{
		kvstore: kvStore,
		cache:   newLiquidStakingCache(),
	}
}

// Start starts the indexer
func (s *liquidStakingIndexer) Start(ctx context.Context) error {
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	return s.loadCache()
}

// Stop stops the indexer
func (s *liquidStakingIndexer) Stop(ctx context.Context) error {
	if err := s.kvstore.Stop(ctx); err != nil {
		return err
	}
	s.cache = newLiquidStakingCache()
	return nil
}

// PutBlock puts a block into indexer
func (s *liquidStakingIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	// new dirty cache for this block
	// it's not necessary to use thread safe cache here, because only one thread will call this function
	// and no update to cache will happen before dirty merge to clean
	dirty := newLiquidStakingDirty(s.cache.unsafe())
	dirty.putHeight(blk.Height())

	// handle events of block
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != LiquidStakingContractAddress {
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
func (s *liquidStakingIndexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

// Height returns the tip block height
func (s *liquidStakingIndexer) Height() (uint64, error) {
	return s.cache.getHeight(), nil
}

// CandidateVotes returns the candidate votes
func (s *liquidStakingIndexer) CandidateVotes(candidate address.Address) *big.Int {
	return s.cache.getCandidateVotes(candidate)
}

// Buckets returns the buckets
func (s *liquidStakingIndexer) Buckets() ([]*Bucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vbs := []*Bucket{}
	for id, bi := range s.cache.getAllBucketInfo() {
		bt := s.cache.mustGetBucketType(bi.TypeIndex)
		vb, err := s.convertToVoteBucket(id, bi, bt)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

// Bucket returns the bucket
func (s *liquidStakingIndexer) Bucket(id uint64) (*Bucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.generateBucket(id)
}

// BucketsByIndices returns the buckets by indices
func (s *liquidStakingIndexer) BucketsByIndices(indices []uint64) ([]*Bucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vbs := make([]*Bucket, 0, len(indices))
	for _, id := range indices {
		vb, err := s.generateBucket(id)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

func (s *liquidStakingIndexer) TotalBucketCount() uint64 {
	return s.cache.getTotalBucketCount()
}

func (s *liquidStakingIndexer) ActiveBucketTypes() (map[uint64]*BucketType, error) {
	return s.cache.getActiveBucketType(), nil
}

func (s *liquidStakingIndexer) generateBucket(id uint64) (*Bucket, error) {
	bi, ok := s.cache.getBucketInfo(id)
	if !ok {
		return nil, errors.Wrapf(ErrBucketInfoNotExist, "id %d", id)
	}
	bt := s.cache.mustGetBucketType(bi.TypeIndex)
	return s.convertToVoteBucket(id, bi, bt)
}

func (s *liquidStakingIndexer) handleEvent(ctx context.Context, dirty *liquidStakingDirty, blk *block.Block, log *action.Log) error {
	// get event abi
	abiEvent, err := _liquidStakingInterface.EventByID(common.Hash(log.Topics[0]))
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
	default:
		return nil
	}
}

func (s *liquidStakingIndexer) loadCache() error {
	delta := newLiquidStakingDelta()
	// load height
	var height uint64
	h, err := s.kvstore.Get(_liquidStakingNS, _liquidStakingHeightKey)
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
	tbc, err := s.kvstore.Get(_liquidStakingNS, _liquidStakingTotalBucketCountKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
	} else {
		totalBucketCount = byteutil.BytesToUint64BigEndian(tbc)
	}
	delta.putTotalBucketCount(totalBucketCount)

	// load bucket info
	ks, vs, err := s.kvstore.Filter(_liquidStakingBucketInfoNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b BucketInfo
		if err := b.deserialize(vs[i]); err != nil {
			return err
		}
		delta.putBucketInfo(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}

	// load bucket type
	ks, vs, err = s.kvstore.Filter(_liquidStakingBucketTypeNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b BucketType
		if err := b.deserialize(vs[i]); err != nil {
			return err
		}
		delta.putBucketType(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}
	return s.cache.merge(delta)
}

func (s *liquidStakingIndexer) commit(dirty *liquidStakingDirty) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	batch, delta := dirty.finalize()
	if err := s.kvstore.WriteBatch(batch); err != nil {
		return err
	}
	if err := s.cache.merge(delta); err != nil {
		return err
	}
	return nil
}

func (s *liquidStakingIndexer) convertToVoteBucket(token uint64, bi *BucketInfo, bt *BucketType) (*Bucket, error) {
	vb := Bucket{
		Index:            token,
		StakedAmount:     bt.Amount,
		StakedDuration:   bt.Duration,
		CreateTime:       bi.CreatedAt,
		StakeStartTime:   bi.CreatedAt,
		UnstakeStartTime: bi.UnstakedAt,
		AutoStake:        bi.UnlockedAt == maxBlockNumber,
		Candidate:        bi.Delegate,
		Owner:            bi.Owner,
	}
	if bi.UnlockedAt != maxBlockNumber {
		vb.StakeStartTime = bi.UnlockedAt
	}
	return &vb, nil
}
