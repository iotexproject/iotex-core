package staking

import (
	"context"
	"math"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	handleCandidateActivate = "candidateActivate"

	candidateNoSelfStakeBucketIndex        = math.MaxUint64
	candidateExitRequested          uint64 = math.MaxUint64
)

func (p *Protocol) handleCandidateActivate(ctx context.Context, act *action.CandidateActivate, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	log := newReceiptLog(p.addr.String(), handleCandidateActivate, featureCtx.NewStakingReceiptFormat)

	bucket, rErr := p.fetchBucket(csm, act.BucketID())
	if rErr != nil {
		return log, nil, rErr
	}
	// caller must be the owner of a candidate
	cand := csm.GetByOwner(actCtx.Caller)
	if cand == nil {
		return log, nil, errCandNotExist
	}

	if err := p.validateBucketSelfStake(ctx, csm, NewEndorsementStateManager(csm.SM()), bucket, cand); err != nil {
		return log, nil, err
	}

	log.AddTopics(byteutil.Uint64ToBytesBigEndian(bucket.Index), bucket.Candidate.Bytes())
	// convert previous self-stake bucket to vote bucket
	if cand.SelfStake.Sign() > 0 {
		prevBucket, err := p.fetchBucket(csm, cand.SelfStakeBucketIdx)
		if err != nil {
			return log, nil, err
		}
		if err := cand.SubVote(p.calculateVoteWeight(prevBucket, true)); err != nil {
			return log, nil, err
		}
		if err := cand.AddVote(p.calculateVoteWeight(prevBucket, false)); err != nil {
			return log, nil, err
		}
	}

	// convert vote bucket to self-stake bucket
	cand.SelfStakeBucketIdx = bucket.Index
	cand.SelfStake.SetBytes(bucket.StakedAmount.Bytes())
	if err := cand.SubVote(p.calculateVoteWeight(bucket, false)); err != nil {
		return log, nil, err
	}
	if err := cand.AddVote(p.calculateVoteWeight(bucket, true)); err != nil {
		return log, nil, err
	}

	if err := csm.Upsert(cand); err != nil {
		return log, nil, csmErrorToHandleError(cand.GetIdentifier().String(), err)
	}
	return log, nil, nil
}

func (p *Protocol) handleCandidateDeactivate(ctx context.Context, act *action.CandidateDeactivate, csm CandidateStateManager) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	cand := csm.GetByOwner(actCtx.Caller)
	if cand == nil {
		return nil, nil, errCandNotExist
	}
	if cand.SelfStakeBucketIdx == candidateNoSelfStakeBucketIndex {
		return nil, nil, &handleError{
			err:           ErrInvalidSelfStkIndex,
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		}
	}
	id := cand.GetIdentifier()
	var topics action.Topics
	var eventData []byte
	var err error
	switch act.Op() {
	case action.CandidateDeactivateOpRequest:
		if err = csm.requestExit(id); err == nil {
			topics, eventData, err = action.PackCandidateDeactivationRequestedEvent(id)
		}
	case action.CandidateDeactivateOpCancel:
		if err := csm.cancelExitRequest(id); err == nil {
			topics, eventData, err = action.PackCandidateDeactivationCanceledEvent(id)
		}
	case action.CandidateDeactivateOpConfirm:
		if err := csm.deactivate(id, protocol.MustGetBlockCtx(ctx).BlockHeight); err == nil {
			topics, eventData, err = action.PackCandidateDeactivatedEvent(id)
		}
	default:
		return nil, nil, &handleError{
			err:           errors.New("invalid operation"),
			failureStatus: iotextypes.ReceiptStatus_Failure,
		}
	}
	if err != nil {
		return nil, nil, csmErrorToHandleError(actCtx.Caller.String(), err)
	}
	return &receiptLog{
		addr:                  p.addr.String(),
		postFairbankMigration: true,
		topics:                topics,
		data:                  eventData,
	}, nil, nil
}

func (p *Protocol) handleScheduleCandidateDeactivation(ctx context.Context, act *action.ScheduleCandidateDeactivation, csm CandidateStateManager) (*receiptLog, []*action.TransactionLog, error) {
	c := csm.GetByIdentifier(act.Delegate())
	g := genesis.MustExtractGenesisContext(ctx)
	if c == nil {
		return nil, nil, errCandNotExist
	}
	rp := rolldpos.FindProtocol(protocol.MustGetRegistry(ctx))
	if rp == nil {
		return nil, nil, errors.New("rolldpos protocol not found")
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	currentEpochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	if currentEpochNum == 0 {
		return nil, nil, errors.New("invalid epoch number")
	}
	c.DeactivatedAt = blkCtx.BlockHeight + g.ExitAdmissionInterval*rp.NumBlocksByEpoch(currentEpochNum)
	if err := csm.Upsert(c); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to update candidate %s", c.GetIdentifier().String())
	}
	if _, err := csm.SM().PutState(&lastExitEpoch{epoch: currentEpochNum}, protocol.NamespaceOption(CandsMapNS), protocol.KeyOption(_lastExitEpoch)); err != nil {
		return nil, nil, err
	}
	topics, eventData, err := action.PackCandidateDeactivationScheduledEvent(c.GetIdentifier(), c.DeactivatedAt)
	if err != nil {
		return nil, nil, err
	}

	return &receiptLog{
		addr:                  p.addr.String(),
		postFairbankMigration: true,
		topics:                topics,
		data:                  eventData,
	}, nil, nil
}

func (p *Protocol) validateBucketSelfStake(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, bucket *VoteBucket, cand *Candidate) ReceiptError {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if err := validateBucketMinAmount(bucket, p.config.RegistrationConsts.MinSelfStake); err != nil {
		return err
	}
	if err := validateBucketStake(bucket, true); err != nil {
		return err
	}
	if err := validateBucketSelfStake(featureCtx, csm, bucket, false); err != nil {
		return err
	}
	if err := validateBucketCandidate(bucket, cand.GetIdentifier()); err != nil {
		return err
	}
	if validateBucketOwner(bucket, cand.Owner) != nil &&
		validateBucketWithEndorsement(ctx, esm, bucket, blkCtx.BlockHeight) != nil {
		return &handleError{
			err:           errors.New("bucket is not a self-owned or endorsed bucket"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	return nil
}
