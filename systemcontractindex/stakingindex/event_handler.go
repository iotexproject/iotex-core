package stakingindex

import (
	"context"
	_ "embed"
	"math"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/abiutil"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	maxStakingNumber uint64 = math.MaxUint64
)

var (
	//go:embed staking_contract_abi_v3.json
	stakingContractABIJSON string
	// StakingContractABI is the abi of staking contract
	StakingContractABI abi.ABI

	// ErrBucketNotExist is the error when bucket does not exist
	ErrBucketNotExist = errors.New("bucket does not exist")
)

type eventHandler struct {
	stakingBucketNS string
	dirty           indexerCache       // dirty cache, a view for current block
	delta           batch.KVStoreBatch // delta for db to store buckets of current block
	tokenOwner      map[uint64]address.Address
	// context for event handler
	blockCtx    protocol.BlockCtx
	timestamped bool
	muted       bool
}

func init() {
	var err error
	StakingContractABI, err = abi.JSON(strings.NewReader(stakingContractABIJSON))
	if err != nil {
		panic(err)
	}
}

func newEventHandler(bucketNS string, dirty indexerCache, blkCtx protocol.BlockCtx, timestamped, muted bool) *eventHandler {
	return &eventHandler{
		stakingBucketNS: bucketNS,
		dirty:           dirty,
		delta:           batch.NewBatch(),
		tokenOwner:      make(map[uint64]address.Address),
		blockCtx:        blkCtx,
		timestamped:     timestamped,
		muted:           muted,
	}
}

func (eh *eventHandler) HandleStakedEvent(event *abiutil.EventParam) error {
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
	createdAt := eh.blockCtx.BlockHeight
	if eh.timestamped {
		createdAt = uint64(eh.blockCtx.BlockTimeStamp.Unix())
	}
	bucket := &Bucket{
		Candidate:        delegateParam,
		Owner:            owner,
		StakedAmount:     amountParam,
		StakedDuration:   durationParam.Uint64(),
		CreatedAt:        createdAt,
		UnlockedAt:       maxStakingNumber,
		UnstakedAt:       maxStakingNumber,
		IsTimestampBased: eh.timestamped,
		Muted:            eh.muted,
	}
	eh.putBucket(tokenIDParam.Uint64(), bucket)
	return nil
}

func (eh *eventHandler) HandleLockedEvent(event *abiutil.EventParam) error {
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
	bkt.StakedDuration = durationParam.Uint64()
	bkt.UnlockedAt = maxStakingNumber
	bkt.Muted = eh.muted
	eh.putBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) HandleUnlockedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.UnlockedAt = eh.blockCtx.BlockHeight
	if eh.timestamped {
		bkt.UnlockedAt = uint64(eh.blockCtx.BlockTimeStamp.Unix())
	}
	eh.putBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) HandleUnstakedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.UnstakedAt = eh.blockCtx.BlockHeight
	if eh.timestamped {
		bkt.UnstakedAt = uint64(eh.blockCtx.BlockTimeStamp.Unix())
	}
	eh.putBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) HandleDelegateChangedEvent(event *abiutil.EventParam) error {
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

func (eh *eventHandler) HandleWithdrawalEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	eh.delBucket(tokenIDParam.Uint64())
	return nil
}

func (eh *eventHandler) HandleTransferEvent(event *abiutil.EventParam) error {
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

func (eh *eventHandler) HandleMergedEvent(event *abiutil.EventParam) error {
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
	b.StakedDuration = durationParam.Uint64()
	b.UnlockedAt = maxStakingNumber
	b.Muted = eh.muted
	for i := 1; i < len(tokenIDsParam); i++ {
		eh.delBucket(tokenIDsParam[i].Uint64())
	}
	eh.putBucket(tokenIDsParam[0].Uint64(), b)
	return nil
}

func (eh *eventHandler) HandleBucketExpandedEvent(event *abiutil.EventParam) error {
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
	b.StakedDuration = durationParam.Uint64()
	b.Muted = eh.muted
	eh.putBucket(tokenIDParam.Uint64(), b)
	return nil
}

func (eh *eventHandler) HandleDonatedEvent(event *abiutil.EventParam) error {
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
	b.Muted = eh.muted
	eh.putBucket(tokenIDParam.Uint64(), b)
	return nil
}

func (eh *eventHandler) Finalize() (batch.KVStoreBatch, indexerCache) {
	delta, dirty := eh.delta, eh.dirty
	eh.delta, eh.dirty = nil, nil
	return delta, dirty
}

func (eh *eventHandler) putBucket(id uint64, bkt *Bucket) {
	data, err := bkt.Serialize()
	if err != nil {
		panic(errors.Wrap(err, "failed to serialize bucket"))
	}
	eh.dirty.PutBucket(id, bkt)
	eh.delta.Put(eh.stakingBucketNS, byteutil.Uint64ToBytesBigEndian(id), data, "failed to put bucket")
}

func (eh *eventHandler) delBucket(id uint64) {
	eh.dirty.DeleteBucket(id)
	eh.delta.Delete(eh.stakingBucketNS, byteutil.Uint64ToBytesBigEndian(id), "failed to delete bucket")
}

func (eh *eventHandler) handleReceipt(ctx context.Context, contractAddr string, receipt *action.Receipt) error {
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil
	}
	for _, log := range receipt.Logs() {
		if log.Address != contractAddr {
			continue
		}
		if err := eh.handleEvent(ctx, log); err != nil {
			return err
		}
	}
	return nil
}

func (eh *eventHandler) handleEvent(ctx context.Context, actLog *action.Log) error {
	// get event abi
	abiEvent, err := StakingContractABI.EventByID(common.Hash(actLog.Topics[0]))
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
		return eh.HandleStakedEvent(event)
	case "Locked":
		return eh.HandleLockedEvent(event)
	case "Unlocked":
		return eh.HandleUnlockedEvent(event)
	case "Unstaked":
		return eh.HandleUnstakedEvent(event)
	case "Merged":
		return eh.HandleMergedEvent(event)
	case "BucketExpanded":
		return eh.HandleBucketExpandedEvent(event)
	case "DelegateChanged":
		return eh.HandleDelegateChangedEvent(event)
	case "Withdrawal":
		return eh.HandleWithdrawalEvent(event)
	case "Donated":
		return eh.HandleDonatedEvent(event)
	case "Transfer":
		return eh.HandleTransferEvent(event)
	case "Approval", "ApprovalForAll", "OwnershipTransferred", "Paused", "Unpaused", "BeneficiaryChanged",
		"Migrated":
		// not require handling events
		return nil
	default:
		return errors.Errorf("unknown event name %s", abiEvent.Name)
	}
}
