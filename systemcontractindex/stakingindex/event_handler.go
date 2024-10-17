package stakingindex

import (
	"context"
	_ "embed"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/abiutil"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	maxBlockNumber uint64 = math.MaxUint64
)

var (
	stakingContractABI = staking.StakingContractABI

	// ErrBucketNotExist is the error when bucket does not exist
	ErrBucketNotExist = errors.New("bucket does not exist")
)

type eventHandler struct {
	stakingBucketNS string
	dirty           *cache             // dirty cache, a view for current block
	delta           batch.KVStoreBatch // delta for db to store buckets of current block
	tokenOwner      map[uint64]address.Address
}

func newEventHandler(bucketNS string, dirty *cache) *eventHandler {
	return &eventHandler{
		stakingBucketNS: bucketNS,
		dirty:           dirty,
		delta:           batch.NewBatch(),
		tokenOwner:      make(map[uint64]address.Address),
	}
}

func (eh *eventHandler) HandleEvent(ctx context.Context, blk *block.Block, actLog *action.Log) error {
	// get event abi
	abiEvent, err := stakingContractABI.EventByID(common.Hash(actLog.Topics[0]))
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
	case "Donated":
		return eh.handleDonatedEvent(event)
	case "Transfer":
		return eh.handleTransferEvent(event)
	case "Approval", "ApprovalForAll", "OwnershipTransferred", "Paused", "Unpaused", "BeneficiaryChanged":
		// not require handling events
		return nil
	default:
		return errors.Errorf("unknown event name %s", abiEvent.Name)
	}
}

func (eh *eventHandler) handleStakedEvent(event *abiutil.EventParam, height uint64) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	delegateParam, err := event.FieldByIDAddress(1)
	if err != nil {
		return err
	}
	amountParam, err := event.FieldByIDUint256(2)
	if err != nil {
		return err
	}
	durationParam, err := event.FieldByIDUint256(3)
	if err != nil {
		return err
	}
	owner, ok := eh.tokenOwner[tokenIDParam.Uint64()]
	if !ok {
		return errors.Errorf("no owner for token id %d", tokenIDParam.Uint64())
	}
	bucket := &Bucket{
		Candidate:                 delegateParam,
		Owner:                     owner,
		StakedAmount:              amountParam,
		StakedDurationBlockNumber: durationParam.Uint64(),
		CreatedAt:                 height,
		UnlockedAt:                maxBlockNumber,
		UnstakedAt:                maxBlockNumber,
	}
	eh.putBucket(tokenIDParam.Uint64(), bucket)
	return nil
}

func (eh *eventHandler) handleLockedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	durationParam, err := event.FieldByIDUint256(1)
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.StakedDurationBlockNumber = durationParam.Uint64()
	bkt.UnlockedAt = maxBlockNumber
	eh.putBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) handleUnlockedEvent(event *abiutil.EventParam, height uint64) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.UnlockedAt = height
	eh.putBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) handleUnstakedEvent(event *abiutil.EventParam, height uint64) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.UnstakedAt = height
	eh.putBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) handleDelegateChangedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	delegateParam, err := event.FieldByIDAddress(1)
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.Candidate = delegateParam
	eh.putBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) handleWithdrawalEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	eh.delBucket(tokenIDParam.Uint64())
	return nil
}

func (eh *eventHandler) handleTransferEvent(event *abiutil.EventParam) error {
	to, err := event.FieldByIDAddress(1)
	if err != nil {
		return err
	}
	tokenIDParam, err := event.FieldByIDUint256(2)
	if err != nil {
		return err
	}

	tokenID := tokenIDParam.Uint64()
	// cache token owner for stake event
	eh.tokenOwner[tokenID] = to
	// update bucket owner if token exists
	bkt := eh.dirty.Bucket(tokenID)
	if bkt != nil {
		bkt.Owner = to
		eh.putBucket(tokenID, bkt)
	}
	return nil
}

func (eh *eventHandler) handleMergedEvent(event *abiutil.EventParam) error {
	tokenIDsParam, err := event.FieldByIDUint256Slice(0)
	if err != nil {
		return err
	}
	amountParam, err := event.FieldByIDUint256(1)
	if err != nil {
		return err
	}
	durationParam, err := event.FieldByIDUint256(2)
	if err != nil {
		return err
	}

	// merge to the first bucket
	b := eh.dirty.Bucket(tokenIDsParam[0].Uint64())
	if b == nil {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDsParam[0].Uint64())
	}
	b.StakedAmount = amountParam
	b.StakedDurationBlockNumber = durationParam.Uint64()
	b.UnlockedAt = maxBlockNumber
	for i := 1; i < len(tokenIDsParam); i++ {
		eh.delBucket(tokenIDsParam[i].Uint64())
	}
	eh.putBucket(tokenIDsParam[0].Uint64(), b)
	return nil
}

func (eh *eventHandler) handleBucketExpandedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	amountParam, err := event.FieldByIDUint256(1)
	if err != nil {
		return err
	}
	durationParam, err := event.FieldByIDUint256(2)
	if err != nil {
		return err
	}

	b := eh.dirty.Bucket(tokenIDParam.Uint64())
	if b == nil {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.StakedAmount = amountParam
	b.StakedDurationBlockNumber = durationParam.Uint64()
	eh.putBucket(tokenIDParam.Uint64(), b)
	return nil
}

func (eh *eventHandler) handleDonatedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	amountParam, err := event.FieldByIDUint256(2)
	if err != nil {
		return err
	}

	b := eh.dirty.Bucket(tokenIDParam.Uint64())
	if b == nil {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.StakedAmount.Sub(b.StakedAmount, amountParam)
	eh.putBucket(tokenIDParam.Uint64(), b)
	return nil
}

func (eh *eventHandler) Finalize() (batch.KVStoreBatch, *cache) {
	delta, dirty := eh.delta, eh.dirty
	eh.delta, eh.dirty = nil, nil
	return delta, dirty
}

func (eh *eventHandler) putBucket(id uint64, bkt *Bucket) {
	eh.dirty.PutBucket(id, bkt)
	eh.delta.Put(eh.stakingBucketNS, byteutil.Uint64ToBytesBigEndian(id), bkt.Serialize(), "failed to put bucket")
}

func (eh *eventHandler) delBucket(id uint64) {
	eh.dirty.DeleteBucket(id)
	eh.delta.Delete(eh.stakingBucketNS, byteutil.Uint64ToBytesBigEndian(id), "failed to delete bucket")
}
