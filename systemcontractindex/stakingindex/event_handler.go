package stakingindex

import (
	_ "embed"
	"math"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/db/batch"
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
	// context for event handler
	height uint64
}

func newEventHandler(bucketNS string, dirty *cache, height uint64) *eventHandler {
	return &eventHandler{
		stakingBucketNS: bucketNS,
		dirty:           dirty,
		delta:           batch.NewBatch(),
		tokenOwner:      make(map[uint64]address.Address),
		height:          height,
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
	bucket := &Bucket{
		Candidate:                 delegateParam,
		Owner:                     owner,
		StakedAmount:              amountParam,
		StakedDurationBlockNumber: durationParam.Uint64(),
		CreatedAt:                 eh.height,
		UnlockedAt:                maxBlockNumber,
		UnstakedAt:                maxBlockNumber,
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
	bkt.StakedDurationBlockNumber = durationParam.Uint64()
	bkt.UnlockedAt = maxBlockNumber
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
	bkt.UnlockedAt = eh.height
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
	bkt.UnstakedAt = eh.height
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
	b.StakedDurationBlockNumber = durationParam.Uint64()
	b.UnlockedAt = maxBlockNumber
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
	b.StakedDurationBlockNumber = durationParam.Uint64()
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
