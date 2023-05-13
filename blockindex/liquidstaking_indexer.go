// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
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
					"internalType": "bytes12",
					"name": "newDelegate",
					"type": "bytes12"
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
					"internalType": "bytes12",
					"name": "delegate",
					"type": "bytes12"
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

		CandidateVotes(ownerAddr string) *big.Int
		Buckets() ([]*Bucket, error)
		Bucket(id uint64) (*Bucket, error)
		BucketsByIndices(indices []uint64) ([]*Bucket, error)
		TotalBucketCount() uint64
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
	// TODO (iip-13): make it concurrent safe
	liquidStakingIndexer struct {
		kvstore             db.KVStore          // persistent storage
		cache               *liquidStakingCache // in-memory index for clean data
		blockInterval       time.Duration
		candNameToOwnerFunc candNameToOwnerFunc
	}
	candNameToOwnerFunc func(name string) (address.Address, error)
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
func NewLiquidStakingIndexer(kvStore db.KVStore, blockInterval time.Duration, candNameToOwnerFunc candNameToOwnerFunc) LiquidStakingIndexer {
	return &liquidStakingIndexer{
		blockInterval:       blockInterval,
		kvstore:             kvStore,
		cache:               newLiquidStakingCache(),
		candNameToOwnerFunc: candNameToOwnerFunc,
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
	return nil
}

// PutBlock puts a block into indexer
func (s *liquidStakingIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	// new dirty cache
	dirty := newLiquidStakingDirty(s.cache)
	dirty.putHeight(blk.Height())
	// make action map
	actionMap := make(map[hash.Hash256]*action.SealedEnvelope)
	for i := range blk.Actions {
		h, err := blk.Actions[i].Hash()
		if err != nil {
			return err
		}
		actionMap[h] = &blk.Actions[i]
	}

	// handle events of block
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		act, ok := actionMap[receipt.ActionHash]
		if !ok {
			return errors.Errorf("action %x not found", receipt.ActionHash)
		}
		for _, log := range receipt.Logs() {
			if log.Address != LiquidStakingContractAddress {
				continue
			}
			if err := s.handleEvent(ctx, dirty, blk, act, log); err != nil {
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
func (s *liquidStakingIndexer) CandidateVotes(ownerAddr string) *big.Int {
	return s.cache.getCandidateVotes(ownerAddr)
}

// Buckets returns the buckets
func (s *liquidStakingIndexer) Buckets() ([]*Bucket, error) {
	vbs := []*Bucket{}
	for id, bi := range s.cache.idBucketMap {
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
	bi, ok := s.cache.idBucketMap[id]
	if !ok {
		return nil, errors.Wrapf(ErrBucketInfoNotExist, "id %d", id)
	}
	bt := s.cache.mustGetBucketType(bi.TypeIndex)
	vb, err := s.convertToVoteBucket(id, bi, bt)
	if err != nil {
		return nil, err
	}
	return vb, nil
}

// BucketsByIndices returns the buckets by indices
func (s *liquidStakingIndexer) BucketsByIndices(indices []uint64) ([]*Bucket, error) {
	vbs := make([]*Bucket, 0, len(indices))
	for _, id := range indices {
		bi, ok := s.cache.idBucketMap[id]
		if !ok {
			return nil, errors.Wrapf(ErrBucketInfoNotExist, "id %d", id)
		}
		bt := s.cache.mustGetBucketType(bi.TypeIndex)
		vb, err := s.convertToVoteBucket(id, bi, bt)
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

func (s *liquidStakingIndexer) handleEvent(ctx context.Context, dirty *liquidStakingDirty, blk *block.Block, act *action.SealedEnvelope, log *action.Log) error {
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
	timestamp := blk.Timestamp()
	switch abiEvent.Name {
	case "BucketTypeActivated":
		return s.handleBucketTypeActivatedEvent(dirty, event, timestamp)
	case "BucketTypeDeactivated":
		return s.handleBucketTypeDeactivatedEvent(dirty, event)
	case "Staked":
		return s.handleStakedEvent(dirty, event, timestamp)
	case "Locked":
		return s.handleLockedEvent(dirty, event)
	case "Unlocked":
		return s.handleUnlockedEvent(dirty, event, timestamp)
	case "Unstaked":
		return s.handleUnstakedEvent(dirty, event, timestamp)
	case "Merged":
		return s.handleMergedEvent(dirty, event)
	case "DurationExtended":
		return s.handleDurationExtendedEvent(dirty, event)
	case "AmountIncreased":
		return s.handleAmountIncreasedEvent(dirty, event)
	case "DelegateChanged":
		return s.handleDelegateChangedEvent(dirty, event)
	case "Withdrawal":
		return s.handleWithdrawalEvent(dirty, event)
	case "Transfer":
		return s.handleTransferEvent(dirty, event)
	default:
		return nil
	}
}

func (s *liquidStakingIndexer) handleTransferEvent(dirty *liquidStakingDirty, event eventParam) error {
	to, err := event.indexedFieldAddress("to")
	if err != nil {
		return err
	}
	tokenID, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	dirty.tokenOwner[tokenID.Uint64()] = to.String()
	return nil
}

func (s *liquidStakingIndexer) handleBucketTypeActivatedEvent(dirty *liquidStakingDirty, event eventParam, timeStamp time.Time) error {
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	bt := BucketType{
		Amount:      amountParam,
		Duration:    s.blockHeightToDuration(durationParam.Uint64()),
		ActivatedAt: timeStamp,
	}
	id, ok := dirty.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		id = dirty.getBucketTypeCount()
		err = dirty.addBucketType(id, &bt)
	} else {
		err = dirty.updateBucketType(id, &bt)
	}

	return err
}

func (s *liquidStakingIndexer) handleBucketTypeDeactivatedEvent(dirty *liquidStakingDirty, event eventParam) error {
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	id, ok := dirty.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	bt, ok := dirty.getBucketType(id)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", id)
	}
	bt.ActivatedAt = time.Time{}
	return dirty.updateBucketType(id, bt)
}

func (s *liquidStakingIndexer) handleStakedEvent(dirty *liquidStakingDirty, event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.fieldBytes12("delegate")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	btIdx, ok := dirty.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	delegateOwner, err := s.candNameToOwnerFunc(delegateParam)
	if err != nil {
		return errors.Wrapf(err, "get delegate owner from %v failed", delegateParam)
	}
	bucket := BucketInfo{
		TypeIndex: btIdx,
		Delegate:  delegateOwner.String(),
		Owner:     dirty.tokenOwner[tokenIDParam.Uint64()],
		CreatedAt: timestamp,
	}
	return dirty.addBucketInfo(tokenIDParam.Uint64(), &bucket)
}

func (s *liquidStakingIndexer) handleLockedEvent(dirty *liquidStakingDirty, event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := dirty.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := dirty.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %v, duration %d", bt.Amount, durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	b.UnlockedAt = time.Time{}
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (s *liquidStakingIndexer) handleUnlockedEvent(dirty *liquidStakingDirty, event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnlockedAt = timestamp
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (s *liquidStakingIndexer) handleUnstakedEvent(dirty *liquidStakingDirty, event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnstakedAt = timestamp
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (s *liquidStakingIndexer) handleMergedEvent(dirty *liquidStakingDirty, event eventParam) error {
	tokenIDsParam, err := event.fieldUint256Slice("tokenIds")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	// merge to the first bucket
	btIdx, ok := dirty.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	b, ok := dirty.getBucketInfo(tokenIDsParam[0].Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDsParam[0].Uint64())
	}
	b.TypeIndex = btIdx
	b.UnlockedAt = time.Time{}
	for i := 1; i < len(tokenIDsParam); i++ {
		if err = dirty.burnBucket(tokenIDsParam[i].Uint64()); err != nil {
			return err
		}
	}
	return dirty.updateBucketInfo(tokenIDsParam[0].Uint64(), b)
}

func (s *liquidStakingIndexer) handleDurationExtendedEvent(dirty *liquidStakingDirty, event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := dirty.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := dirty.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", bt.Amount.Int64(), durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (s *liquidStakingIndexer) handleAmountIncreasedEvent(dirty *liquidStakingDirty, event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := dirty.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := dirty.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), bt.Duration)
	}
	b.TypeIndex = newBtIdx
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (s *liquidStakingIndexer) handleDelegateChangedEvent(dirty *liquidStakingDirty, event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.fieldBytes12("newDelegate")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	delegateOwner, err := s.candNameToOwnerFunc(delegateParam)
	if err != nil {
		return errors.Wrapf(err, "get owner of candidate %s", delegateParam)
	}
	b.Delegate = delegateOwner.String()
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (s *liquidStakingIndexer) handleWithdrawalEvent(dirty *liquidStakingDirty, event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	return dirty.burnBucket(tokenIDParam.Uint64())
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
	if err != nil {
		if !errors.Is(err, db.ErrBucketNotExist) {
			return err
		}
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
	if err != nil {
		if !errors.Is(err, db.ErrBucketNotExist) {
			return err
		}
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
	batch, delta := dirty.finalize()
	if err := s.kvstore.WriteBatch(batch); err != nil {
		return err
	}
	if err := s.cache.merge(delta); err != nil {
		return err
	}
	return nil
}

func (s *liquidStakingIndexer) blockHeightToDuration(height uint64) time.Duration {
	return time.Duration(height) * s.blockInterval
}

func (s *liquidStakingIndexer) convertToVoteBucket(token uint64, bi *BucketInfo, bt *BucketType) (*Bucket, error) {
	var err error
	vb := Bucket{
		Index:            token,
		StakedAmount:     bt.Amount,
		StakedDuration:   bt.Duration,
		CreateTime:       bi.CreatedAt,
		StakeStartTime:   bi.CreatedAt,
		UnstakeStartTime: bi.UnstakedAt,
		AutoStake:        bi.UnlockedAt.IsZero(),
	}
	vb.Candidate, err = address.FromString(bi.Delegate)
	if err != nil {
		return nil, err
	}
	vb.Owner, err = address.FromHex(bi.Owner)
	if err != nil {
		return nil, err
	}
	if !bi.UnlockedAt.IsZero() {
		vb.StakeStartTime = bi.UnlockedAt
	}
	return &vb, nil
}
