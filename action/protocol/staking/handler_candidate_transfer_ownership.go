package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
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
	store := newCandidateTransferOwnership()
	if err := store.LoadFromStateManager(csm.SM()); err != nil {
		return log, nil, err
	}

	if err := p.validateCandidateTransferOwnership(ctx, act, csm, store, actCtx.Caller); err != nil {
		return log, nil, err
	}
	candidate := csm.GetByOwner(actCtx.Caller)
	if candidate.Voter == nil || candidate.Voter.String() == "" {
		candidate.Voter = candidate.Owner
	}
	candidate.Owner = act.NewOwner()
	if err := csm.Upsert(candidate); err != nil {
		return log, nil, csmErrorToHandleError(candidate.Owner.String(), err)
	}
	store.Update(candidate.Voter, act.NewOwner())
	if err := store.StoreToStateManager(csm.SM()); err != nil {
		return log, nil, err
	}
	log.AddTopics(actCtx.Caller.Bytes(), act.NewOwner().Bytes())
	return log, nil, nil
}

func (p *Protocol) validateCandidateTransferOwnership(_ context.Context, act *action.CandidateTransferOwnership,
	csm CandidateStateManager, cto *CandidateTransferOwnership, caller address.Address) ReceiptError {
	//check if the candidate exists
	candidate := csm.GetByOwner(caller)
	if candidate == nil {
		return &handleError{
			err:           errors.New("candidate does not exist"),
			failureStatus: iotextypes.ReceiptStatus_ErrCandidateNotExist,
		}
	}
	//check if the new owner is self
	if !address.Equal(act.NewOwner(), caller) {
		return &handleError{
			err:           errors.New("operator is not authorized"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	//check the new owner is exist or is contract address
	if acc, err := accountutil.LoadOrCreateAccount(csm.SM(), act.NewOwner()); err != nil || acc.IsContract() {
		return &handleError{
			err:           errors.New("new owner is not a valid address"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}

	//check the new owner is not in the candidate list
	cand := csm.GetByOwner(act.NewOwner())
	if cand != nil {
		return &handleError{
			err:           errors.New("new owner is already a candidate"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	return nil
}
