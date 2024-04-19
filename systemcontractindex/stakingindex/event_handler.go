package stakingindex

import (
	"context"
	_ "embed"
	"math"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/abiutil"
)

const (
	maxBlockNumber uint64 = math.MaxUint64
)

type eventHandler struct {
	dirty      *cache             // dirty cache, a view for current block
	delta      batch.KVStoreBatch // delta for db to store buckets of current block
	tokenOwner map[uint64]address.Address
}

var (
	// TODO: fill in the ABI of staking contract
	//go:embed staking.json
	stakingContractJSONABI string
	stakingContractABI     abi.ABI

	// ErrBucketNotExist is the error when bucket does not exist
	ErrBucketNotExist = errors.New("bucket does not exist")
)

func init() {
	var err error
	stakingContractABI, err = abi.JSON(strings.NewReader(stakingContractJSONABI))
	if err != nil {
		panic(err)
	}
}

func newEventHandler(dirty *cache) *eventHandler {
	return &eventHandler{
		dirty: dirty,
		delta: batch.NewBatch(),
	}
}

func (eh *eventHandler) HandleEvent(ctx context.Context, blk *block.Block, log *action.Log) error {
	// get event abi
	abiEvent, err := stakingContractABI.EventByID(common.Hash(log.Topics[0]))
	if err != nil {
		return errors.Wrapf(err, "get event abi from topic %v failed", log.Topics[0])
	}

	// unpack event data
	event, err := abiutil.UnpackEventParam(abiEvent, log)
	if err != nil {
		return err
	}

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
	case "Approval", "ApprovalForAll", "OwnershipTransferred", "Paused", "Unpaused":
		// not require handling events
		return nil
	default:
		return errors.Errorf("unknown event name %s", abiEvent.Name)
	}
}

func (eh *eventHandler) handleStakedEvent(event abiutil.EventParam, height uint64) error {
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
	eh.dirty.PutBucket(tokenIDParam.Uint64(), bucket)
	return nil
}

func (eh *eventHandler) handleLockedEvent(event abiutil.EventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.FieldUint256("duration")
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.StakedDurationBlockNumber = durationParam.Uint64()
	bkt.UnlockedAt = maxBlockNumber
	eh.dirty.PutBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) handleUnlockedEvent(event abiutil.EventParam, height uint64) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.UnlockedAt = height
	eh.dirty.PutBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) handleUnstakedEvent(event abiutil.EventParam, height uint64) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.UnstakedAt = height
	eh.dirty.PutBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) handleDelegateChangedEvent(event abiutil.EventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.FieldAddress("newDelegate")
	if err != nil {
		return err
	}

	bkt := eh.dirty.Bucket(tokenIDParam.Uint64())
	if bkt == nil {
		return errors.Errorf("no bucket for token id %d", tokenIDParam.Uint64())
	}
	bkt.Candidate = delegateParam
	eh.dirty.PutBucket(tokenIDParam.Uint64(), bkt)
	return nil
}

func (eh *eventHandler) handleWithdrawalEvent(event abiutil.EventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	eh.dirty.DeleteBucket(tokenIDParam.Uint64())
	return nil
}

func (eh *eventHandler) handleTransferEvent(event abiutil.EventParam) error {
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
	bkt := eh.dirty.Bucket(tokenID)
	if bkt != nil {
		bkt.Owner = to
		eh.dirty.PutBucket(tokenID, bkt)
	}
	return nil
}

func (eh *eventHandler) handleMergedEvent(event abiutil.EventParam) error {
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
	b := eh.dirty.Bucket(tokenIDsParam[0].Uint64())
	if b == nil {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDsParam[0].Uint64())
	}
	b.StakedAmount = amountParam
	b.StakedDurationBlockNumber = durationParam.Uint64()
	b.UnlockedAt = maxBlockNumber
	for i := 1; i < len(tokenIDsParam); i++ {
		eh.dirty.DeleteBucket(tokenIDsParam[i].Uint64())
	}
	eh.dirty.PutBucket(tokenIDsParam[0].Uint64(), b)
	return nil
}

func (eh *eventHandler) handleBucketExpandedEvent(event abiutil.EventParam) error {
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

	b := eh.dirty.Bucket(tokenIDParam.Uint64())
	if b == nil {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.StakedAmount = amountParam
	b.StakedDurationBlockNumber = durationParam.Uint64()
	eh.dirty.PutBucket(tokenIDParam.Uint64(), b)
	return nil
}

func (eh *eventHandler) handleDonatedEvent(event abiutil.EventParam) error {
	tokenIDParam, err := event.IndexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	amountParam, err := event.FieldUint256("amount")
	if err != nil {
		return err
	}

	b := eh.dirty.Bucket(tokenIDParam.Uint64())
	if b == nil {
		return errors.Wrapf(ErrBucketNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.StakedAmount.Sub(b.StakedAmount, amountParam)
	eh.dirty.PutBucket(tokenIDParam.Uint64(), b)
	return nil
}

func (eh *eventHandler) Finalize() (batch.KVStoreBatch, *cache) {
	delta, dirty := eh.delta, eh.dirty
	eh.delta, eh.dirty = nil, nil
	return delta, dirty
}
