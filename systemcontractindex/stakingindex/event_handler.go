package stakingindex

import (
	_ "embed"
	"math"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db/batch"
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
	dirty           *cache             // dirty cache, a view for current block
	delta           batch.KVStoreBatch // delta for db to store buckets of current block
	tokenOwner      map[uint64]address.Address
	// context for event handler
	block       *block.Block
	timestamped bool
}

func init() {
	var err error
	StakingContractABI, err = abi.JSON(strings.NewReader(stakingContractABIJSON))
	if err != nil {
		panic(err)
	}
}

func newEventHandler(bucketNS string, dirty *cache, blk *block.Block, timestamped bool) *eventHandler {
	return &eventHandler{
		stakingBucketNS: bucketNS,
		dirty:           dirty,
		delta:           batch.NewBatch(),
		tokenOwner:      make(map[uint64]address.Address),
		block:           blk,
		timestamped:     timestamped,
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
	createdAt := eh.block.Height()
	if eh.timestamped {
		createdAt = uint64(eh.block.Timestamp().Unix())
	}
	bucket := &Bucket{
		Candidate:      delegateParam,
		Owner:          owner,
		StakedAmount:   amountParam,
		StakedDuration: durationParam.Uint64(),
		CreatedAt:      createdAt,
		UnlockedAt:     maxStakingNumber,
		UnstakedAt:     maxStakingNumber,
		Timestamped:    eh.timestamped,
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
	bkt.UnlockedAt = eh.block.Height()
	if eh.timestamped {
		bkt.UnlockedAt = uint64(eh.block.Timestamp().Unix())
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
	bkt.UnstakedAt = eh.block.Height()
	if eh.timestamped {
		bkt.UnstakedAt = uint64(eh.block.Timestamp().Unix())
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
