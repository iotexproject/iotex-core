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
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	// TODO (iip-13): replace with the real liquid staking contract address
	LiquidStakingContractAddress = ""

	// TODO (iip-13): replace with the real liquid staking contract ABI
	_liquidStakingContractABI = `[
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "x",
					"type": "uint256"
				}
			],
			"name": "Set",
			"type": "event"
		},
		{
			"inputs": [],
			"name": "get",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "x",
					"type": "uint256"
				}
			],
			"name": "set",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

type (
	// LiquidStakingIndexer is the interface of liquid staking indexer
	LiquidStakingIndexer interface {
		blockdao.BlockIndexer

		GetCandidateVotes(candidate string) (*big.Int, error)
		GetBucket(bucketIndex uint64) (*staking.VoteBucket, error)
	}

	liquidStakingIndexer struct {
		kvStore db.KVStore

		bucketMap     map[uint64]*BucketInfo // map[token]bucketInfo
		bucketTypes   []*BucketType
		bucketTypeMap map[int64]map[int64]uint64 // map[amount][duration]index

		blockInterval time.Duration
	}

	// BucketInfo is the bucket information
	BucketInfo struct {
		TypeIndex  uint64
		UnlockedAt *time.Time
		UnstakedAt *time.Time
		Delegate   string
	}

	// BucketType is the bucket type
	BucketType struct {
		Amount      *big.Int
		Duration    time.Duration
		ActivatedAt *time.Time
	}

	eventParam map[string]any
)

var (
	_liquidStakingInterface abi.ABI

	errUnpackEvent = errors.New("failed to unpack event")
)

func init() {
	var err error
	_liquidStakingInterface, err = abi.JSON(strings.NewReader(_liquidStakingContractABI))
	if err != nil {
		panic(err)
	}
}

// NewLiquidStakingIndexer creates a new liquid staking indexer
func NewLiquidStakingIndexer() *liquidStakingIndexer {
	return &liquidStakingIndexer{}
}

func (s *liquidStakingIndexer) Start(ctx context.Context) error {
	return nil
}

func (s *liquidStakingIndexer) Stop(ctx context.Context) error {
	return nil
}

func (s *liquidStakingIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != LiquidStakingContractAddress {
				continue
			}
			if err := s.handleEvent(ctx, blk, log); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *liquidStakingIndexer) DeleteTipBlock(context.Context, *block.Block) error {
	return nil
}

func (s *liquidStakingIndexer) Height() (uint64, error) {
	return 0, nil
}

func (s *liquidStakingIndexer) GetCandidateVotes(candidate string) (*big.Int, error) {
	return nil, nil
}

func (s *liquidStakingIndexer) GetBucket(bucketIndex uint64) (*staking.VoteBucket, error) {
	return nil, nil
}

func (s *liquidStakingIndexer) handleEvent(ctx context.Context, blk *block.Block, log *action.Log) error {
	// get event abi
	abiEvent, err := _liquidStakingInterface.EventByID(common.Hash(log.Topics[0]))
	if err != nil {
		return errors.Wrapf(err, "get event abi from topic %v failed", log.Topics[0])
	}
	// unpack event data
	event := make(map[string]any)
	if err = abiEvent.Inputs.UnpackIntoMap(event, log.Data); err != nil {
		return errors.Wrap(err, "unpack event data failed")
	}
	// handle different kinds of event
	switch abiEvent.Name {
	case "BucketTypeActivated":
		err = s.handleBucketTypeActivatedEvent(event, blk.Timestamp())
	case "BucketTypeDeactivated":
		err = s.handleBucketTypeDeactivatedEvent(event)
	case "Staked":
		err = s.handleStakedEvent(event)
	case "Locked":
		err = s.handleLockedEvent(event)
	case "Unlocked":
		err = s.handleUnlockedEvent(event, blk.Timestamp())
	case "Unstaked":
		err = s.handleUnstakedEvent(event, blk.Timestamp())
	case "Merged":
		err = s.handleMergedEvent(event)
	case "DurationExtended":
		err = s.handleDurationExtendedEvent(event)
	case "AmountIncreased":
		err = s.handleAmountIncreasedEvent(event)
	case "DelegateChanged":
		err = s.handleDelegateChangedEvent(event)
	case "Withdrawal":
		err = s.handleWithdrawalEvent(event)
	default:
		err = nil
	}
	return err
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
	if ok {
		s.bucketTypes[id] = &bt
	} else {
		s.bucketTypes = append(s.bucketTypes, &bt)
	}
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

	id := s.mustGetBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	s.bucketTypes[id].ActivatedAt = nil
	return nil
}

func (s *liquidStakingIndexer) handleStakedEvent(event eventParam) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
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

	btIdx := s.mustGetBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	bucket := BucketInfo{
		TypeIndex: btIdx,
		Delegate:  string(delegateParam[:]),
	}
	s.bucketMap[tokenIDParam.Uint64()] = &bucket
	return nil
}

func (s *liquidStakingIndexer) handleLockedEvent(event eventParam) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b := s.mustGetBucket(tokenIDParam.Uint64())
	bt := s.bucketTypes[b.TypeIndex]
	newBtIdx := s.mustGetBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	b.TypeIndex = newBtIdx
	b.UnlockedAt = nil
	return nil
}

func (s *liquidStakingIndexer) handleUnlockedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}

	b := s.mustGetBucket(tokenIDParam.Uint64())
	b.UnlockedAt = &timestamp
	return nil
}

func (s *liquidStakingIndexer) handleUnstakedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}

	b := s.mustGetBucket(tokenIDParam.Uint64())
	b.UnstakedAt = &timestamp
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
	btIdx := s.mustGetBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	b := s.mustGetBucket(tokenIDsParam[0].Uint64())
	b.TypeIndex = btIdx
	b.UnlockedAt = nil
	for i := 1; i < len(tokenIDsParam); i++ {
		s.burnBucket(tokenIDsParam[i].Uint64())
	}
	return nil
}

func (s *liquidStakingIndexer) handleDurationExtendedEvent(event eventParam) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b := s.mustGetBucket(tokenIDParam.Uint64())
	bt := s.bucketTypes[b.TypeIndex]
	newBtIdx := s.mustGetBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	b.TypeIndex = newBtIdx
	return nil
}

func (s *liquidStakingIndexer) handleAmountIncreasedEvent(event eventParam) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}

	b := s.mustGetBucket(tokenIDParam.Uint64())
	bt := s.bucketTypes[b.TypeIndex]
	newBtIdx := s.mustGetBucketTypeIndex(amountParam, bt.Duration)
	b.TypeIndex = newBtIdx
	return nil
}

func (s *liquidStakingIndexer) handleDelegateChangedEvent(event eventParam) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.fieldBytes12("newDelegate")
	if err != nil {
		return err
	}

	b := s.mustGetBucket(tokenIDParam.Uint64())
	b.Delegate = string(delegateParam[:])
	return nil
}

func (s *liquidStakingIndexer) handleWithdrawalEvent(event eventParam) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}

	s.burnBucket(tokenIDParam.Uint64())
	return nil
}

func (s *liquidStakingIndexer) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	if m, ok := s.bucketTypeMap[amount.Int64()]; ok {
		if index, ok := m[int64(duration)]; ok {
			return index, true
		}
	}
	return 0, false
}

func (s *liquidStakingIndexer) mustGetBucketTypeIndex(amount *big.Int, duration time.Duration) uint64 {
	idx, ok := s.getBucketTypeIndex(amount, duration)
	if !ok {
		log.S().Panic("bucket type not found", zap.Uint64("amount", amount.Uint64()), zap.Uint64("duration", uint64(duration)))
	}
	return idx
}

func (s *liquidStakingIndexer) mustGetBucket(token uint64) *BucketInfo {
	b, ok := s.bucketMap[token]
	if !ok {
		log.S().Panic("bucket not found", zap.Uint64("tokenID", token))
	}
	return b
}

func (s *liquidStakingIndexer) burnBucket(token uint64) {
	delete(s.bucketMap, token)
}

func (s *liquidStakingIndexer) blockHeightToDuration(height uint64) time.Duration {
	return time.Duration(height) * s.blockInterval
}

func eventField[T any](e eventParam, name string) (T, error) {
	field, ok := e[name].(T)
	if !ok {
		return field, errors.Wrapf(errUnpackEvent, "invalid %s %v", name, e[name])
	}
	return field, nil
}

func (e eventParam) fieldUint256(name string) (*big.Int, error) {
	return eventField[*big.Int](e, name)
}

func (e eventParam) fieldBytes12(name string) ([12]byte, error) {
	return eventField[[12]byte](e, name)
}

func (e eventParam) fieldUint256Slice(name string) ([]*big.Int, error) {
	return eventField[[]*big.Int](e, name)
}

func (e eventParam) fieldAddress(name string) (common.Address, error) {
	return eventField[common.Address](e, name)
}
