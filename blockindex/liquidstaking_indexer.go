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

	// bucket related namespace in db
	_liquidStakingBucketInfoNS      = "lsbInfo"
	_liquidStakingBucketTypeNS      = "lsbType"
	_liquidStakingBucketTypeMapNS   = "lsbTypeMap"
	_liquidStakingBucketTypeCountNS = "lsbTypeCount"

	// bucket cache size
	_liquidStakingBucketCacheSize = 10000
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
		dirty      batch.CachedBatch // im-memory dirty data
		dirtyCache *liquidStakingCache
		clean      db.KVStore          // clean data in db
		cleanCache *liquidStakingCache // in-memory index for clean data
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

	// eventParam is a struct to hold smart contract event parameters, which can easily convert a param to go type
	// TODO: this is general enough to be moved to a common package
	eventParam map[string]any
)

var (
	_liquidStakingInterface abi.ABI

	errInvlidEventParam   = errors.New("invalid event param")
	errBucketTypeNotExist = errors.New("bucket type does not exist")
	errBucketInfoNotExist = errors.New("bucket info does not exist")
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
	id, ok := s.data.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		id = s.data.getBucketTypeCount()
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

	id, ok := s.data.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	bt, ok := s.data.getBucketType(id)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", id)
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

	btIdx, ok := s.data.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
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

	b, ok := s.data.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.data.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.data.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", bt.Amount.Int64(), durationParam.Uint64())
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

	b, ok := s.data.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
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

	b, ok := s.data.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
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
	btIdx, ok := s.data.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	b, ok := s.data.getBucketInfo(tokenIDsParam[0].Uint64())
	if !ok {
		return errors.Wrapf(errBucketInfoNotExist, "token id %d", tokenIDsParam[0].Uint64())
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

	b, ok := s.data.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.data.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.data.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", bt.Amount.Int64(), durationParam.Uint64())
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

	b, ok := s.data.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.data.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.data.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), bt.Duration)
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

	b, ok := s.data.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
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

func newLiquidStakingData(kvStore db.KVStore) (*liquidStakingData, error) {
	data := liquidStakingData{
		dirty:      batch.NewCachedBatch(),
		clean:      kvStore,
		cleanCache: newLiquidStakingCache(),
		dirtyCache: newLiquidStakingCache(),
	}
	return &data, nil
}

func (s *liquidStakingData) loadCache() error {
	ks, vs, err := s.clean.Filter(_liquidStakingBucketInfoNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
	}
	for i := range vs {
		var b BucketInfo
		if err := json.Unmarshal(vs[i], &b); err != nil {
			return err
		}
		s.cleanCache.putBucketInfo(binary.LittleEndian.Uint64(ks[i]), &b)
	}

	ks, vs, err = s.clean.Filter(_liquidStakingBucketTypeNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
	}
	for i := range vs {
		var b BucketType
		if err := json.Unmarshal(vs[i], &b); err != nil {
			return err
		}
		s.cleanCache.putBucketType(binary.LittleEndian.Uint64(ks[i]), &b)
	}
	return nil
}

func (s *liquidStakingData) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	id, ok := s.dirtyCache.getBucketTypeIndex(amount, duration)
	if ok {
		return id, true
	}
	id, ok = s.cleanCache.getBucketTypeIndex(amount, duration)
	return id, ok
}

func (s *liquidStakingData) getBucketTypeCount() uint64 {
	base := len(s.cleanCache.bucketTypes)
	add := 0
	for k, dbt := range s.dirtyCache.bucketTypes {
		_, ok := s.cleanCache.bucketTypes[k]
		if dbt != nil && !ok {
			add++
		} else if dbt == nil && ok {
			add--
		}
	}
	return uint64(base + add)
}

func (s *liquidStakingData) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.dirtyCache.getBucketType(id)
	if ok {
		return bt, true
	}
	bt, ok = s.cleanCache.getBucketType(id)
	return bt, ok
}

func (s *liquidStakingData) putBucketType(id uint64, bt *BucketType) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, id)
	s.dirty.Put(_liquidStakingBucketTypeNS, key, bt.serialize(), "failed to put bucket type")
	s.dirtyCache.putBucketType(id, bt)
}

func (s *liquidStakingData) putBucketInfo(id uint64, bi *BucketInfo) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, id)
	s.dirty.Put(_liquidStakingBucketInfoNS, key, bi.serialize(), "failed to put bucket info")
	s.dirtyCache.putBucketInfo(id, bi)
}

func (s *liquidStakingData) getBucketInfo(id uint64) (*BucketInfo, bool) {
	bi, ok := s.dirtyCache.getBucketInfo(id)
	if ok {
		return bi, bi != nil
	}
	bi, ok = s.cleanCache.getBucketInfo(id)
	return bi, ok
}

func (s *liquidStakingData) burnBucket(id uint64) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, id)
	s.dirty.Delete(_liquidStakingBucketInfoNS, key, "failed to delete bucket info")
	s.dirtyCache.putBucketInfo(id, nil)
}

// GetBuckets(height uint64, offset, limit uint32)
// BucketsByVoter(voterAddr string, offset, limit uint32)
// BucketsByCandidate(candidateAddr string, offset, limit uint32)
// BucketByIndices(indecis []uint64)
// BucketCount()
// TotalStakingAmount()

func (s *liquidStakingData) commit() error {
	if err := s.cleanCache.writeBatch(s.dirty); err != nil {
		return err
	}
	if err := s.clean.WriteBatch(s.dirty); err != nil {
		return err
	}
	s.dirty.Lock()
	s.dirty.ClearAndUnlock()
	s.dirtyCache = newLiquidStakingCache()
	return nil
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
