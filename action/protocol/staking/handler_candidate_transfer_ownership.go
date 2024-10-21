package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/state"
)

const (
	handleCandidateTransferOwnership = "candidateTransferOwnership"
)

func (p *Protocol) handleCandidateTransferOwnership(ctx context.Context, act *action.CandidateTransferOwnership, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)

	log := newReceiptLog(p.addr.String(), handleCandidateTransferOwnership, featureCtx.NewStakingReceiptFormat)
	_, fetchErr := fetchCaller(ctx, csm, big.NewInt(0))
	if fetchErr != nil {
		return log, nil, fetchErr
	}

	if err := p.validateCandidateTransferOwnership(ctx, act, csm, actCtx.Caller); err != nil {
		return log, nil, err
	}
	candidate := csm.GetByOwner(actCtx.Caller)
	if candidate.Identifier == nil || candidate.Identifier.String() == "" {
		candidate.Identifier = candidate.Owner
	}
	candidate.Owner = act.NewOwner()
	// clear selfstake
	needClear := func() (bool, *big.Int, error) {
		bucket, err := csm.getBucket(candidate.SelfStakeBucketIdx)
		if err == nil {
			// keep the self-stake bucket if it's endorse bucket
			esm := NewEndorsementStateReader(csm.SR())
			height, serr := esm.Height()
			if serr != nil {
				return false, nil, errors.Wrap(serr, "failed to get height")
			}
			endorse, serr := esm.Status(featureCtx, bucket.Index, height)
			if serr != nil {
				return false, nil, errors.Wrap(serr, "failed to get endorsement status")
			}
			if endorse != EndorseExpired {
				return false, big.NewInt(0), nil
			}
			// change the self-stake bucket to vote bucket
			subVotes := big.NewInt(0)
			if !bucket.isUnstaked() {
				selfStakeVotes := p.calculateVoteWeight(bucket, true)
				votes := p.calculateVoteWeight(bucket, false)
				subVotes.Sub(selfStakeVotes, votes)
			}
			return true, subVotes, nil
		} else if errors.Is(err, state.ErrStateNotExist) {
			return true, big.NewInt(0), nil
		}
		return false, nil, errors.Wrap(err, "failed to get self-stake bucket")
	}
	if candidate.isSelfStakeBucketSettled() {
		clear, subVotes, err := needClear()
		if err != nil {
			return log, nil, err
		}
		if clear {
			candidate.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
			candidate.SelfStake = big.NewInt(0)
			candidate.Votes.Sub(candidate.Votes, subVotes)
		}
	}
	if err := csm.Upsert(candidate); err != nil {
		return log, nil, csmErrorToHandleError(candidate.GetIdentifier().String(), err)
	}
	log.AddTopics(actCtx.Caller.Bytes(), act.NewOwner().Bytes())
	return log, nil, nil
}

func (p *Protocol) validateCandidateTransferOwnership(_ context.Context, act *action.CandidateTransferOwnership,
	csm CandidateStateManager, caller address.Address) ReceiptError {
	//check if the candidate exists
	candidate := csm.GetByOwner(caller)
	if candidate == nil {
		return &handleError{
			err:           errors.New("candidate does not exist"),
			failureStatus: iotextypes.ReceiptStatus_ErrCandidateNotExist,
		}
	}
	//check if the new owner is self
	if address.Equal(act.NewOwner(), caller) {
		return &handleError{
			err:           errors.New("new owner is the same as the current owner"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	//check the new owner is exist or is contract address
	// TODO: only accept an existed EOA as new owner
	if acc, err := accountutil.LoadOrCreateAccount(csm.SM(), act.NewOwner()); err != nil || acc.IsContract() {
		return &handleError{
			err:           errors.New("new owner is not a valid address"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnknown,
		}
	}

	//check the new owner is not in the candidate list
	cand := csm.GetByOwner(act.NewOwner())
	if cand != nil {
		return &handleError{
			err:           errors.New("new owner is already a candidate"),
			failureStatus: iotextypes.ReceiptStatus_ErrCandidateAlreadyExist,
		}
	}

	// check the new owner is exist in the identifier list
	// except the candidate itself
	cand = csm.GetByIdentifier(act.NewOwner())
	if cand != nil && !cand.Equal(candidate) {
		return &handleError{
			err:           errors.New("new owner is already a candidate"),
			failureStatus: iotextypes.ReceiptStatus_ErrCandidateAlreadyExist,
		}
	}
	return nil
}
