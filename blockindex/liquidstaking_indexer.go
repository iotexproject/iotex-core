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
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	// LiquidStakingContractAddress  is the address of liquid staking contract
	// TODO (iip-13): replace with the real liquid staking contract address
	LiquidStakingContractAddress = "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
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
	_liquidStakingHeightNS     = "lsHeight"
)

type (
	// LiquidStakingIndexer is the interface of liquid staking indexer
	LiquidStakingIndexer interface {
		blockdao.BlockIndexer

		CandidateVotes(candidate string) *big.Int
		Buckets() ([]*Bucket, error)
		Bucket(id uint64) (*Bucket, error)
	}

	liquidStakingIndexer struct {
		dirty      batch.CachedBatch   // batch for dirty data
		kvstore    db.KVStore          // persistent storage
		dirtyCache *liquidStakingCache // in-memory index for dirty data
		cleanCache *liquidStakingCache // in-memory index for clean data

		tokenOwner    map[uint64]string // token id -> owner
		blockInterval time.Duration
	}
)

var (
	_liquidStakingInterface abi.ABI
	_liquidStakingHeightKey = []byte("lsHeight")

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
func NewLiquidStakingIndexer(kvStore db.KVStore, blockInterval time.Duration) LiquidStakingIndexer {
	return &liquidStakingIndexer{
		blockInterval: blockInterval,
		dirty:         batch.NewCachedBatch(),
		dirtyCache:    newLiquidStakingCache(),
		kvstore:       kvStore,
		cleanCache:    newLiquidStakingCache(),
		tokenOwner:    make(map[uint64]string),
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
	actionMap := make(map[hash.Hash256]*action.SealedEnvelope)
	for _, act := range blk.Actions {
		h, err := act.Hash()
		if err != nil {
			return err
		}
		actionMap[h] = &act
	}

	s.dirtyCache.putHeight(blk.Height())
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
			if err := s.handleEvent(ctx, blk, act, log); err != nil {
				return err
			}
		}
	}
	return s.commit()
}

// DeleteTipBlock deletes the tip block from indexer
func (s *liquidStakingIndexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

// Height returns the tip block height
func (s *liquidStakingIndexer) Height() (uint64, error) {
	return s.cleanCache.getHeight(), nil
}

// CandidateVotes returns the candidate votes
func (s *liquidStakingIndexer) CandidateVotes(candidate string) *big.Int {
	return s.cleanCache.getCandidateVotes(candidate)
}

// Buckets returns the buckets
func (s *liquidStakingIndexer) Buckets() ([]*Bucket, error) {
	vbs := []*Bucket{}
	for id, bi := range s.cleanCache.idBucketMap {
		bt := s.cleanCache.mustGetBucketType(bi.TypeIndex)
		vb, err := convertToVoteBucket(id, bi, bt)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

// Bucket returns the bucket
func (s *liquidStakingIndexer) Bucket(id uint64) (*Bucket, error) {
	bi, ok := s.cleanCache.idBucketMap[id]
	if !ok {
		return nil, errors.Wrapf(ErrBucketInfoNotExist, "id %d", id)
	}
	bt := s.cleanCache.mustGetBucketType(bi.TypeIndex)
	vb, err := convertToVoteBucket(id, bi, bt)
	if err != nil {
		return nil, err
	}
	return vb, nil
}

func (s *liquidStakingIndexer) handleEvent(ctx context.Context, blk *block.Block, act *action.SealedEnvelope, log *action.Log) error {
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
		return s.handleBucketTypeActivatedEvent(event, timestamp)
	case "BucketTypeDeactivated":
		return s.handleBucketTypeDeactivatedEvent(event)
	case "Staked":
		return s.handleStakedEvent(event, timestamp)
	case "Locked":
		return s.handleLockedEvent(event)
	case "Unlocked":
		return s.handleUnlockedEvent(event, timestamp)
	case "Unstaked":
		return s.handleUnstakedEvent(event, timestamp)
	case "Merged":
		return s.handleMergedEvent(event)
	case "DurationExtended":
		return s.handleDurationExtendedEvent(event)
	case "AmountIncreased":
		return s.handleAmountIncreasedEvent(event)
	case "DelegateChanged":
		return s.handleDelegateChangedEvent(event)
	case "Withdrawal":
		return s.handleWithdrawalEvent(event)
	case "Transfer":
		return s.handleTransferEvent(event)
	default:
		return nil
	}
}

func (s *liquidStakingIndexer) handleTransferEvent(event eventParam) error {
	to, err := event.indexedFieldAddress("to")
	if err != nil {
		return err
	}
	tokenID, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	s.tokenOwner[tokenID.Uint64()] = to.String()
	return nil
}

func (s *liquidStakingIndexer) handleBucketTypeActivatedEvent(event eventParam, timeStamp time.Time) error {
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
		ActivatedAt: &timeStamp,
	}
	id, ok := s.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		id = s.getBucketTypeCount()
	}
	s.putBucketType(id, &bt)
	return nil
}

func (s *liquidStakingIndexer) handleBucketTypeDeactivatedEvent(event eventParam) error {
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	id, ok := s.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	bt, ok := s.getBucketType(id)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", id)
	}
	bt.ActivatedAt = nil
	s.putBucketType(id, bt)
	return nil
}

func (s *liquidStakingIndexer) handleStakedEvent(event eventParam, timestamp time.Time) error {
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

	btIdx, ok := s.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}

	bucket := BucketInfo{
		TypeIndex: btIdx,
		Delegate:  delegateParam,
		Owner:     s.tokenOwner[tokenIDParam.Uint64()],
		CreatedAt: timestamp,
	}
	s.putBucketInfo(tokenIDParam.Uint64(), &bucket)
	return nil
}

func (s *liquidStakingIndexer) handleLockedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %v, duration %d", bt.Amount, durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	b.UnlockedAt = nil
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleUnlockedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnlockedAt = &timestamp
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleUnstakedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnstakedAt = &timestamp
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleMergedEvent(event eventParam) error {
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
	btIdx, ok := s.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	b, ok := s.getBucketInfo(tokenIDsParam[0].Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDsParam[0].Uint64())
	}
	b.TypeIndex = btIdx
	b.UnlockedAt = nil
	for i := 1; i < len(tokenIDsParam); i++ {
		s.burnBucket(tokenIDsParam[i].Uint64())
	}
	s.putBucketInfo(tokenIDsParam[0].Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleDurationExtendedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", bt.Amount.Int64(), durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleAmountIncreasedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), bt.Duration)
	}
	b.TypeIndex = newBtIdx
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleDelegateChangedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.fieldBytes12("newDelegate")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.Delegate = string(delegateParam[:])
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleWithdrawalEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	s.burnBucket(tokenIDParam.Uint64())
	return nil
}

func (s *liquidStakingIndexer) loadCache() error {
	// load height
	var height uint64
	h, err := s.kvstore.Get(_liquidStakingHeightNS, _liquidStakingHeightKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
		height = 0
	} else {
		height = deserializeUint64(h)
	}
	s.cleanCache.putHeight(height)

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
		s.cleanCache.putBucketInfo(deserializeUint64(ks[i]), &b)
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
		s.cleanCache.putBucketType(deserializeUint64(ks[i]), &b)
	}
	return nil
}

func (s *liquidStakingIndexer) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	id, ok := s.dirtyCache.getBucketTypeIndex(amount, duration)
	if ok {
		return id, true
	}
	id, ok = s.cleanCache.getBucketTypeIndex(amount, duration)
	return id, ok
}

func (s *liquidStakingIndexer) getBucketTypeCount() uint64 {
	base := len(s.cleanCache.idBucketTypeMap)
	add := 0
	for k, dbt := range s.dirtyCache.idBucketTypeMap {
		_, ok := s.cleanCache.idBucketTypeMap[k]
		if dbt != nil && !ok {
			add++
		} else if dbt == nil && ok {
			add--
		}
	}
	return uint64(base + add)
}

func (s *liquidStakingIndexer) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.dirtyCache.getBucketType(id)
	if ok {
		return bt, true
	}
	bt, ok = s.cleanCache.getBucketType(id)
	return bt, ok
}

func (s *liquidStakingIndexer) putHeight(h uint64) {
	s.dirty.Put(_liquidStakingHeightNS, _liquidStakingHeightKey, serializeUint64(h), "failed to put height")
	s.dirtyCache.putHeight(h)
}

func (s *liquidStakingIndexer) putBucketType(id uint64, bt *BucketType) {
	s.dirty.Put(_liquidStakingBucketTypeNS, serializeUint64(id), bt.serialize(), "failed to put bucket type")
	s.dirtyCache.putBucketType(id, bt)
}

func (s *liquidStakingIndexer) putBucketInfo(id uint64, bi *BucketInfo) {
	s.dirty.Put(_liquidStakingBucketInfoNS, serializeUint64(id), bi.serialize(), "failed to put bucket info")
	s.dirtyCache.putBucketInfo(id, bi)
}

func (s *liquidStakingIndexer) getBucketInfo(id uint64) (*BucketInfo, bool) {
	bi, ok := s.dirtyCache.getBucketInfo(id)
	if ok {
		return bi, bi != nil
	}
	bi, ok = s.cleanCache.getBucketInfo(id)
	return bi, ok
}

func (s *liquidStakingIndexer) burnBucket(id uint64) {
	s.dirty.Delete(_liquidStakingBucketInfoNS, serializeUint64(id), "failed to delete bucket info")
	s.dirtyCache.markDeleteBucketInfo(id)
}

func (s *liquidStakingIndexer) commit() error {
	if err := s.cleanCache.writeBatch(s.dirty); err != nil {
		return err
	}
	if err := s.kvstore.WriteBatch(s.dirty); err != nil {
		return err
	}
	s.dirty.Lock()
	s.dirty.ClearAndUnlock()
	s.dirtyCache = newLiquidStakingCache()
	return nil
}

func (s *liquidStakingIndexer) blockHeightToDuration(height uint64) time.Duration {
	return time.Duration(height) * s.blockInterval
}
