// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db/batch"
)

const (
	// StakingContractABI is the ABI of system staking contract
	StakingContractABI = `[
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
			"name": "BucketExpanded",
			"type": "event"
		}
	]`
)

// contractStakingEventHandler handles events from staking contract
type contractStakingEventHandler struct {
	dirty      *contractStakingDirty
	tokenOwner map[uint64]address.Address
}

var (
	_stakingInterface abi.ABI
)

func init() {
	var err error
	_stakingInterface, err = abi.JSON(strings.NewReader(StakingContractABI))
	if err != nil {
		panic(err)
	}
}

func newContractStakingEventHandler(cache *contractStakingCache) *contractStakingEventHandler {
	dirty := newContractStakingDirty(cache)
	return &contractStakingEventHandler{
		dirty:      dirty,
		tokenOwner: make(map[uint64]address.Address),
	}
}

func (eh *contractStakingEventHandler) HandleEvent(ctx context.Context, blk *block.Block, log *action.Log) error {
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
		return eh.handleBucketTypeActivatedEvent(event, blk.Height())
	case "BucketTypeDeactivated":
		return eh.handleBucketTypeDeactivatedEvent(event, blk.Height())
	case "Staked":
		return eh.handleStakedEvent(event, blk.Height())
	case "Locked":
		return eh.handleLockedEvent(event)
	case "Unlocked":
		return eh.handleUnlockedEvent(event, blk.Height())
	case "Unstaked":
		return eh.handleUnstakedEvent(event, blk.Height())
	case "Merged":
		return eh.handleMergedEvent(event)
	case "BucketExpanded":
		return eh.handleBucketExpandedEvent(event)
	case "DelegateChanged":
		return eh.handleDelegateChangedEvent(event)
	case "Withdrawal":
		return eh.handleWithdrawalEvent(event)
	case "Transfer":
		return eh.handleTransferEvent(event)
	case "Approval", "ApprovalForAll", "OwnershipTransferred", "Paused", "Unpaused":
		// not require handling events
		return nil
	default:
		return errors.Errorf("unknown event name %s", abiEvent.Name)
	}
}

func (eh *contractStakingEventHandler) Result() (batch.KVStoreBatch, *contractStakingDelta) {
	return eh.dirty.finalize()
}

func (eh *contractStakingEventHandler) handleTransferEvent(event eventParam) error {
	to, err := event.IndexedFieldAddress("to")
	if err != nil {
		return err
	}
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	tokenID := tokenIDParam.Uint64()
	// cache token owner for stake event
	eh.tokenOwner[tokenID] = to
	// update bucket owner if token exists
	if bi, ok := eh.dirty.getBucketInfo(tokenID); ok {
		bi.Owner = to
		return eh.dirty.updateBucketInfo(tokenID, bi)
	}

	return nil
}

func (eh *contractStakingEventHandler) handleBucketTypeActivatedEvent(event eventParam, height uint64) error {
	amountParam, err := event.FieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.FieldUint256("duration")
	if err != nil {
		return err
	}

	bt := BucketType{
		Amount:      amountParam,
		Duration:    durationParam.Uint64(),
		ActivatedAt: height,
	}
	return eh.dirty.putBucketType(&bt)
}

func (eh *contractStakingEventHandler) handleBucketTypeDeactivatedEvent(event eventParam, height uint64) error {
	amountParam, err := event.FieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.FieldUint256("duration")
	if err != nil {
		return err
	}

	id, bt, ok := eh.dirty.matchBucketType(amountParam, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	bt.ActivatedAt = maxBlockNumber
	return eh.dirty.updateBucketType(id, bt)
}

func (eh *contractStakingEventHandler) handleStakedEvent(event eventParam, height uint64) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.FieldAddress("delegate")
	if err != nil {
		return err
	}
	amountParam, err := event.FieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.FieldUint256("duration")
	if err != nil {
		return err
	}

	btIdx, _, ok := eh.dirty.matchBucketType(amountParam, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	owner, ok := eh.tokenOwner[tokenIDParam.Uint64()]
	if !ok {
		return errors.Errorf("no owner for token id %d", tokenIDParam.Uint64())
	}
	bucket := bucketInfo{
		TypeIndex:  btIdx,
		Delegate:   delegateParam,
		Owner:      owner,
		CreatedAt:  height,
		UnlockedAt: maxBlockNumber,
		UnstakedAt: maxBlockNumber,
	}
	return eh.dirty.addBucketInfo(tokenIDParam.Uint64(), &bucket)
}

func (eh *contractStakingEventHandler) handleLockedEvent(event eventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.FieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := eh.dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := eh.dirty.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, _, ok := eh.dirty.matchBucketType(bt.Amount, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %v, duration %d", bt.Amount, durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	b.UnlockedAt = maxBlockNumber
	return eh.dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (eh *contractStakingEventHandler) handleUnlockedEvent(event eventParam, height uint64) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := eh.dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnlockedAt = height
	return eh.dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (eh *contractStakingEventHandler) handleUnstakedEvent(event eventParam, height uint64) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := eh.dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnstakedAt = height
	return eh.dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (eh *contractStakingEventHandler) handleMergedEvent(event eventParam) error {
	tokenIDsParam, err := event.FieldUint256Slice("tokenIds")
	if err != nil {
		return err
	}
	amountParam, err := event.FieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.FieldUint256("duration")
	if err != nil {
		return err
	}

	// merge to the first bucket
	btIdx, _, ok := eh.dirty.matchBucketType(amountParam, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	b, ok := eh.dirty.getBucketInfo(tokenIDsParam[0].Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDsParam[0].Uint64())
	}
	b.TypeIndex = btIdx
	b.UnlockedAt = maxBlockNumber
	for i := 1; i < len(tokenIDsParam); i++ {
		if err = eh.dirty.deleteBucketInfo(tokenIDsParam[i].Uint64()); err != nil {
			return err
		}
	}
	return eh.dirty.updateBucketInfo(tokenIDsParam[0].Uint64(), b)
}

func (eh *contractStakingEventHandler) handleBucketExpandedEvent(event eventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	amountParam, err := event.FieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.FieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := eh.dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	newBtIdx, _, ok := eh.dirty.matchBucketType(amountParam, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	return eh.dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (eh *contractStakingEventHandler) handleDelegateChangedEvent(event eventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.FieldAddress("newDelegate")
	if err != nil {
		return err
	}

	b, ok := eh.dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.Delegate = delegateParam
	return eh.dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (eh *contractStakingEventHandler) handleWithdrawalEvent(event eventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	return eh.dirty.deleteBucketInfo(tokenIDParam.Uint64())
}
