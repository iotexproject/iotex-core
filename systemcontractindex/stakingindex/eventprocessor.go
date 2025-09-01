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
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/abiutil"
)

const (
	maxStakingNumber uint64 = math.MaxUint64
)

var (
	//go:embed staking_contract_abi_v3.json
	stakingContractABIJSON string
	// StakingContractABI is the abi of staking contract
	StakingContractABI abi.ABI
)

type (
	eventProcessor struct {
		contractAddr address.Address
		handler      staking.EventHandler
		tokenOwner   map[uint64]address.Address
		// context for event handler
		blockCtx    protocol.BlockCtx
		timestamped bool
		muted       bool
	}
)

func init() {
	var err error
	StakingContractABI, err = abi.JSON(strings.NewReader(stakingContractABIJSON))
	if err != nil {
		panic(err)
	}
}

func newEventProcessor(contractAddr address.Address, blkCtx protocol.BlockCtx, handler staking.EventHandler, timestamped, muted bool) *eventProcessor {
	return &eventProcessor{
		contractAddr: contractAddr,
		tokenOwner:   make(map[uint64]address.Address),
		blockCtx:     blkCtx,
		timestamped:  timestamped,
		muted:        muted,
		handler:      handler,
	}
}

func (ep *eventProcessor) handleStakedEvent(event *abiutil.EventParam) error {
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
	owner, ok := ep.tokenOwner[tokenIDParam.Uint64()]
	if !ok {
		return errors.Errorf("no owner for token id %d", tokenIDParam.Uint64())
	}
	createdAt := ep.blockCtx.BlockHeight
	if ep.timestamped {
		createdAt = uint64(ep.blockCtx.BlockTimeStamp.Unix())
	}
	bucket := &Bucket{
		Candidate:        delegateParam,
		Owner:            owner,
		StakedAmount:     amountParam,
		StakedDuration:   durationParam.Uint64(),
		CreatedAt:        createdAt,
		UnlockedAt:       maxStakingNumber,
		UnstakedAt:       maxStakingNumber,
		IsTimestampBased: ep.timestamped,
		Muted:            ep.muted,
	}
	return ep.handler.PutBucket(ep.contractAddr, tokenIDParam.Uint64(), bucket)
}

func (ep *eventProcessor) handleLockedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	durationParam, err := event.FieldByIDUint256(1)
	if err != nil {
		return err
	}

	bkt, err := ep.handler.DeductBucket(ep.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bkt.StakedDuration = durationParam.Uint64()
	bkt.UnlockedAt = maxStakingNumber
	bkt.Muted = ep.muted
	return ep.handler.PutBucket(ep.contractAddr, tokenIDParam.Uint64(), bkt)
}

func (ep *eventProcessor) handleUnlockedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	bkt, err := ep.handler.DeductBucket(ep.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bkt.UnlockedAt = ep.blockCtx.BlockHeight
	if ep.timestamped {
		bkt.UnlockedAt = uint64(ep.blockCtx.BlockTimeStamp.Unix())
	}
	return ep.handler.PutBucket(ep.contractAddr, tokenIDParam.Uint64(), bkt)
}

func (ep *eventProcessor) handleUnstakedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	bkt, err := ep.handler.DeductBucket(ep.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bkt.UnstakedAt = ep.blockCtx.BlockHeight
	if ep.timestamped {
		bkt.UnstakedAt = uint64(ep.blockCtx.BlockTimeStamp.Unix())
	}
	return ep.handler.PutBucket(ep.contractAddr, tokenIDParam.Uint64(), bkt)
}

func (ep *eventProcessor) handleDelegateChangedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	delegateParam, err := event.FieldByIDAddress(1)
	if err != nil {
		return err
	}

	bkt, err := ep.handler.DeductBucket(ep.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	bkt.Candidate = delegateParam
	return ep.handler.PutBucket(ep.contractAddr, tokenIDParam.Uint64(), bkt)
}

func (ep *eventProcessor) handleWithdrawalEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}

	return ep.handler.DeleteBucket(ep.contractAddr, tokenIDParam.Uint64())
}

func (ep *eventProcessor) handleTransferEvent(event *abiutil.EventParam) error {
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
	ep.tokenOwner[tokenID] = to
	// update bucket owner if token exists
	bkt, err := ep.handler.DeductBucket(ep.contractAddr, tokenID)
	switch errors.Cause(err) {
	case nil:
		bkt.Owner = to
		return ep.handler.PutBucket(ep.contractAddr, tokenID, bkt)
	case contractstaking.ErrBucketNotExist:
		return nil
	default:
		return err
	}
}

func (ep *eventProcessor) handleMergedEvent(event *abiutil.EventParam) error {
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
	b, err := ep.handler.DeductBucket(ep.contractAddr, tokenIDsParam[0].Uint64())
	if err != nil {
		return err
	}
	b.StakedAmount = amountParam
	b.StakedDuration = durationParam.Uint64()
	b.UnlockedAt = maxStakingNumber
	b.Muted = ep.muted
	for i := 1; i < len(tokenIDsParam); i++ {
		if err := ep.handler.DeleteBucket(ep.contractAddr, tokenIDsParam[i].Uint64()); err != nil {
			return err
		}
	}
	return ep.handler.PutBucket(ep.contractAddr, tokenIDsParam[0].Uint64(), b)
}

func (ep *eventProcessor) handleBucketExpandedEvent(event *abiutil.EventParam) error {
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

	b, err := ep.handler.DeductBucket(ep.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	b.StakedAmount = amountParam
	b.StakedDuration = durationParam.Uint64()
	b.Muted = ep.muted
	return ep.handler.PutBucket(ep.contractAddr, tokenIDParam.Uint64(), b)
}

func (ep *eventProcessor) handleDonatedEvent(event *abiutil.EventParam) error {
	tokenIDParam, err := event.FieldByIDUint256(0)
	if err != nil {
		return err
	}
	amountParam, err := event.FieldByIDUint256(2)
	if err != nil {
		return err
	}

	b, err := ep.handler.DeductBucket(ep.contractAddr, tokenIDParam.Uint64())
	if err != nil {
		return err
	}
	b.StakedAmount.Sub(b.StakedAmount, amountParam)
	b.Muted = ep.muted
	return ep.handler.PutBucket(ep.contractAddr, tokenIDParam.Uint64(), b)
}

func (ep *eventProcessor) ProcessReceipts(ctx context.Context, receipts ...*action.Receipt) error {
	expectedContractAddr := ep.contractAddr.String()
	for _, receipt := range receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != expectedContractAddr {
				continue
			}
			if err := ep.processEvent(ctx, log); err != nil {
				return errors.Wrapf(err, "handle %d log in receipt %x failed", log.Index, receipt.ActionHash)
			}
		}
	}
	return nil
}

func (ep *eventProcessor) processEvent(ctx context.Context, actLog *action.Log) error {
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
		return ep.handleStakedEvent(event)
	case "Locked":
		return ep.handleLockedEvent(event)
	case "Unlocked":
		return ep.handleUnlockedEvent(event)
	case "Unstaked":
		return ep.handleUnstakedEvent(event)
	case "Merged":
		return ep.handleMergedEvent(event)
	case "BucketExpanded":
		return ep.handleBucketExpandedEvent(event)
	case "DelegateChanged":
		return ep.handleDelegateChangedEvent(event)
	case "Withdrawal":
		return ep.handleWithdrawalEvent(event)
	case "Donated":
		return ep.handleDonatedEvent(event)
	case "Transfer":
		return ep.handleTransferEvent(event)
	case "Approval", "ApprovalForAll", "OwnershipTransferred", "Paused", "Unpaused", "BeneficiaryChanged",
		"Migrated":
		// not require handling events
		return nil
	default:
		return errors.Errorf("unknown event name %s", abiEvent.Name)
	}
}
