// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"encoding/binary"
	"encoding/json"
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
	"github.com/iotexproject/iotex-core/db/batch"
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

	_liquidStakingBucketInfoNS      = "lsdBucketInfo"
	_liquidStakingBucketTypeNS      = "lsdBucketType"
	_liquidStakingBucketTypeMapNS   = "lsdBucketTypeMap"
	_liquidStakingBucketTypeCountNS = "lsdBucketTypeCount"
)

type (
	// LiquidStakingIndexer is the interface of liquid staking indexer
	LiquidStakingIndexer interface {
		blockdao.BlockIndexer

		GetCandidateVotes(candidate string) (*big.Int, error)
		GetBucket(bucketIndex uint64) (*staking.VoteBucket, error)
	}

	liquidStakingIndexer struct {
		data *liquidStakingData

		blockInterval time.Duration
	}

	liquidStakingData struct {
		// dirty data in memory
		batch batch.CachedBatch
		// clean data in db
		kvStore db.KVStore
		// clean data cache in memory
		// bucketMap     map[uint64]*BucketInfo // map[token]bucketInfo
		// bucketTypes   []*BucketType
		// bucketTypeMap map[int64]map[int64]uint64 // map[amount][duration]index
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

	errInvlidEventParam = errors.New("invalid event param")
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
	s.data.newBatch()
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
	return s.data.commit()
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
	event := make(eventParam)
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
	id, err := s.data.getBucketTypeIndex(amountParam, bt.Duration)
	if err != nil {
		if !errors.Is(err, batch.ErrNotExist) {
			return err
		}
		id, err = s.data.getBucketTypeCount()
		if err != nil {
			return err
		}
	}
	s.data.putBucketType(id, &bt)
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

	id, err := s.data.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if err != nil {
		return err
	}
	bt, err := s.data.getBucketType(id)
	if err != nil {
		return err
	}
	bt.ActivatedAt = nil
	s.data.putBucketType(id, bt)
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

	btIdx, err := s.data.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if err != nil {
		return err
	}
	bucket := BucketInfo{
		TypeIndex: btIdx,
		Delegate:  string(delegateParam[:]),
	}
	s.data.putBucketInfo(tokenIDParam.Uint64(), &bucket)
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

	b, err := s.data.getBucketInfo(tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bt, err := s.data.getBucketType(b.TypeIndex)
	if err != nil {
		return err
	}
	newBtIdx, err := s.data.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if err != nil {
		return err
	}
	b.TypeIndex = newBtIdx
	b.UnlockedAt = nil
	s.data.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleUnlockedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, err := s.data.getBucketInfo(tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	b.UnlockedAt = &timestamp
	s.data.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleUnstakedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, err := s.data.getBucketInfo(tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	b.UnstakedAt = &timestamp
	s.data.putBucketInfo(tokenIDParam.Uint64(), b)
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
	btIdx, err := s.data.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if err != nil {
		return err
	}
	b, err := s.data.getBucketInfo(tokenIDsParam[0].Uint64())
	if err != nil {
		return err
	}
	b.TypeIndex = btIdx
	b.UnlockedAt = nil
	for i := 1; i < len(tokenIDsParam); i++ {
		s.data.burnBucket(tokenIDsParam[i].Uint64())
	}
	s.data.putBucketInfo(tokenIDsParam[0].Uint64(), b)
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

	b, err := s.data.getBucketInfo(tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bt, err := s.data.getBucketType(b.TypeIndex)
	if err != nil {
		return err
	}
	newBtIdx, err := s.data.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if err != nil {
		return err
	}
	b.TypeIndex = newBtIdx
	s.data.putBucketInfo(tokenIDParam.Uint64(), b)
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

	b, err := s.data.getBucketInfo(tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bt, err := s.data.getBucketType(b.TypeIndex)
	if err != nil {
		return err
	}
	newBtIdx, err := s.data.getBucketTypeIndex(amountParam, bt.Duration)
	if err != nil {
		return err
	}
	b.TypeIndex = newBtIdx
	s.data.putBucketInfo(tokenIDParam.Uint64(), b)
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

	b, err := s.data.getBucketInfo(tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	b.Delegate = string(delegateParam[:])
	s.data.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleWithdrawalEvent(event eventParam) error {
	tokenIDParam, err := event.fieldUint256("tokenId")
	if err != nil {
		return err
	}

	s.data.burnBucket(tokenIDParam.Uint64())
	return nil
}

func (s *liquidStakingIndexer) blockHeightToDuration(height uint64) time.Duration {
	return time.Duration(height) * s.blockInterval
}

func eventField[T any](e eventParam, name string) (T, error) {
	field, ok := e[name].(T)
	if !ok {
		return field, errors.Wrapf(errInvlidEventParam, "field %s got %#v, expect %T", name, e[name], field)
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

func newLiquidStakingData(kvStore db.KVStore) *liquidStakingData {
	return &liquidStakingData{
		kvStore: kvStore,
		batch:   batch.NewCachedBatch(),
	}
}

// newBatch
func (s *liquidStakingData) newBatch() {
	s.batch.Clear()
}

// getBucketTypeIndex
func (s *liquidStakingData) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, error) {
	d := make([]byte, 8)
	binary.LittleEndian.PutUint64(d, uint64(duration))
	amountKey := []byte("amount")
	durationKey := []byte("duration")
	key := append(amountKey, amount.Bytes()...)
	key = append(key, durationKey...)
	key = append(key, d...)

	v, err := s.get(_liquidStakingBucketTypeMapNS, key)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(v), nil
}

// get bucket type count
func (s *liquidStakingData) getBucketTypeCount() (uint64, error) {
	v, err := s.get(_liquidStakingBucketTypeCountNS, []byte("count"))
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(v), nil
}

// get bucket type
func (s *liquidStakingData) getBucketType(id uint64) (*BucketType, error) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, id)
	v, err := s.get(_liquidStakingBucketTypeNS, key)
	if err != nil {
		return nil, err
	}
	var bt BucketType
	if err := json.Unmarshal(v, &bt); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal bucket type")
	}
	return &bt, nil
}

func (s *liquidStakingData) get(ns string, key []byte) ([]byte, error) {
	v, err := s.batch.Get(ns, key)
	if err != nil {
		if !errors.Is(err, batch.ErrNotExist) {
			return nil, err
		}
		v, err = s.kvStore.Get(ns, key)
	}
	return v, err
}

// putBucketType
func (s *liquidStakingData) putBucketType(id uint64, bt *BucketType) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, id)
	s.batch.Put(_liquidStakingBucketTypeNS, key, bt.serialize(), "failed to put bucket type")
}

// putBucketInfo
func (s *liquidStakingData) putBucketInfo(id uint64, bi *BucketInfo) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, id)
	s.batch.Put(_liquidStakingBucketInfoNS, key, bi.serialize(), "failed to put bucket info")
}

// getBucketInfo
func (s *liquidStakingData) getBucketInfo(id uint64) (*BucketInfo, error) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, id)
	v, err := s.get(_liquidStakingBucketInfoNS, key)
	if err != nil {
		return nil, err
	}
	var bi BucketInfo
	if err := json.Unmarshal(v, &bi); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal bucket info")
	}
	return &bi, nil
}

// burnBucket
func (s *liquidStakingData) burnBucket(id uint64) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, id)
	s.batch.Delete(_liquidStakingBucketInfoNS, key, "failed to delete bucket info")
}

// commit()
func (s *liquidStakingData) commit() error {
	return s.kvStore.WriteBatch(s.batch)
}

func (bt *BucketType) serialize() []byte {
	b, err := json.Marshal(bt)
	if err != nil {
		log.S().Panic("marshal bucket type", zap.Error(err))
	}
	return b
}

func (bi *BucketInfo) serialize() []byte {
	b, err := json.Marshal(bi)
	if err != nil {
		log.S().Panic("marshal bucket info", zap.Error(err))
	}
	return b
}
