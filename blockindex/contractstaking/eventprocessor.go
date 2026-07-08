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
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
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

// contractStakingEventProcessor handles events from staking contract
type contractStakingEventProcessor struct {
	dirty        staking.EventHandler
	contractAddr address.Address
	tokenOwner   map[uint64]address.Address
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

func newContractStakingEventProcessor(contractAddr address.Address, dirty staking.EventHandler) *contractStakingEventProcessor {
	return &contractStakingEventProcessor{
		dirty:        dirty,
		contractAddr: contractAddr,
		tokenOwner:   make(map[uint64]address.Address),
	}
}

func (processor *contractStakingEventProcessor) ProcessReceipts(ctx context.Context, receipts ...*action.Receipt) error {
	expectedContractAddr := processor.contractAddr.String()
	for _, receipt := range receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != expectedContractAddr {
				continue
			}
			if err := processor.HandleEvent(ctx, log); err != nil {
				return err
			}
		}
	}
	return nil
}

func (processor *contractStakingEventProcessor) HandleEvent(ctx context.Context, log *action.Log) error {
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
		return processor.handleBucketTypeActivatedEvent(event, log.BlockHeight)
	case "BucketTypeDeactivated":
		return processor.handleBucketTypeDeactivatedEvent(event, log.BlockHeight)
	case "Staked":
		return processor.handleStakedEvent(event, log.BlockHeight)
	case "Locked":
		return processor.handleLockedEvent(event)
	case "Unlocked":
		return processor.handleUnlockedEvent(event, log.BlockHeight)
	case "Unstaked":
		return processor.handleUnstakedEvent(event, log.BlockHeight)
	case "Merged":
		return processor.handleMergedEvent(event)
	case "BucketExpanded":
		return processor.handleBucketExpandedEvent(event)
	case "DelegateChanged":
		return processor.handleDelegateChangedEvent(event)
	case "Withdrawal":
		return processor.handleWithdrawalEvent(event)
	case "Transfer":
		return processor.handleTransferEvent(event)
	case "Approval", "ApprovalForAll", "OwnershipTransferred", "Paused", "Unpaused":
		// not require handling events
		return nil
	default:
		return errors.Errorf("unknown event name %s", abiEvent.Name)
	}
}

func (processor *contractStakingEventProcessor) handleTransferEvent(event eventParam) error {
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
	processor.tokenOwner[tokenID] = to
	// update bucket owner if token exists
	bucket, err := processor.dirty.DeductBucket(processor.contractAddr, tokenID)
	switch errors.Cause(err) {
	case nil:
		bucket.Owner = to
		return processor.dirty.PutBucket(processor.contractAddr, tokenID, bucket)
	case contractstaking.ErrBucketNotExist:
		return nil
	default:
		return err
	}
}

func (processor *contractStakingEventProcessor) handleBucketTypeActivatedEvent(event eventParam, height uint64) error {
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
	return processor.dirty.PutBucketType(processor.contractAddr, &bt)
}

func (processor *contractStakingEventProcessor) handleBucketTypeDeactivatedEvent(event eventParam, height uint64) error {
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
		ActivatedAt: maxBlockNumber,
	}

	return processor.dirty.PutBucketType(processor.contractAddr, &bt)
}

func (processor *contractStakingEventProcessor) handleStakedEvent(event eventParam, height uint64) error {
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

	owner, ok := processor.tokenOwner[tokenIDParam.Uint64()]
	if !ok {
		return errors.Errorf("no owner for token id %d", tokenIDParam.Uint64())
	}

	return processor.dirty.PutBucket(processor.contractAddr, tokenIDParam.Uint64(), &contractstaking.Bucket{
		Candidate:      delegateParam,
		Owner:          owner,
		StakedAmount:   amountParam,
		StakedDuration: durationParam.Uint64(),
		CreatedAt:      height,
		UnstakedAt:     maxBlockNumber,
		UnlockedAt:     maxBlockNumber,
	})
}

func (processor *contractStakingEventProcessor) handleLockedEvent(event eventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.FieldUint256("duration")
	if err != nil {
		return err
	}

	bucket, err := processor.dirty.DeductBucket(processor.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}

	bucket.StakedDuration = durationParam.Uint64()
	bucket.UnlockedAt = maxBlockNumber

	return processor.dirty.PutBucket(processor.contractAddr, tokenIDParam.Uint64(), bucket)
}

func (processor *contractStakingEventProcessor) handleUnlockedEvent(event eventParam, height uint64) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	bucket, err := processor.dirty.DeductBucket(processor.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bucket.UnlockedAt = height
	return processor.dirty.PutBucket(processor.contractAddr, tokenIDParam.Uint64(), bucket)
}

func (processor *contractStakingEventProcessor) handleUnstakedEvent(event eventParam, height uint64) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	bucket, err := processor.dirty.DeductBucket(processor.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bucket.UnstakedAt = height
	return processor.dirty.PutBucket(processor.contractAddr, tokenIDParam.Uint64(), bucket)
}

func (processor *contractStakingEventProcessor) handleMergedEvent(event eventParam) error {
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
	bucket, err := processor.dirty.DeductBucket(processor.contractAddr, tokenIDsParam[0].Uint64())
	if err != nil {
		return err
	}
	bucket.StakedAmount = amountParam
	bucket.StakedDuration = durationParam.Uint64()
	bucket.UnlockedAt = maxBlockNumber
	for i := 1; i < len(tokenIDsParam); i++ {
		if err := processor.dirty.DeleteBucket(processor.contractAddr, tokenIDsParam[i].Uint64()); err != nil {
			return err
		}
	}
	return processor.dirty.PutBucket(processor.contractAddr, tokenIDsParam[0].Uint64(), bucket)
}

func (processor *contractStakingEventProcessor) handleBucketExpandedEvent(event eventParam) error {
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

	bucket, err := processor.dirty.DeductBucket(processor.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bucket.StakedAmount = amountParam
	bucket.StakedDuration = durationParam.Uint64()

	return processor.dirty.PutBucket(processor.contractAddr, tokenIDParam.Uint64(), bucket)
}

func (processor *contractStakingEventProcessor) handleDelegateChangedEvent(event eventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.FieldAddress("newDelegate")
	if err != nil {
		return err
	}

	bucket, err := processor.dirty.DeductBucket(processor.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return errors.Wrapf(err, "token id %d", tokenIDParam.Uint64())
	}
	bucket.Candidate = delegateParam

	return processor.dirty.PutBucket(processor.contractAddr, tokenIDParam.Uint64(), bucket)
}

func (processor *contractStakingEventProcessor) handleWithdrawalEvent(event eventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	return processor.dirty.DeleteBucket(processor.contractAddr, tokenIDParam.Uint64())
}
