package staking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

const (
	handleCandidateTransferOwnership = "candidateTransferOwnership"
)

func (p *Protocol) handleCandidateTransferOwnership(ctx context.Context, act *action.CandidateTransferOwnership, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	log := newReceiptLog(p.addr.String(), handleCandidateTransferOwnership, featureCtx.NewStakingReceiptFormat)

	if err := p.validateCandidateTransferOwnership(ctx, act, csm, actCtx.Caller); err != nil {
		return log, nil, err
	}
	store := newCandidateTransferOwnership()
	if err := store.LoadFromStateManager(csm.SM()); err != nil {
		return log, nil, err
	}
	store.Update(act.Name(), act.NewOwner())
	if err := store.StoreToStateManager(csm.SM()); err != nil {
		return log, nil, err
	}
	return log, nil, nil
}

func (p *Protocol) validateCandidateTransferOwnership(_ context.Context, act *action.CandidateTransferOwnership, csm CandidateStateManager, caller address.Address) ReceiptError {
	if !action.IsValidCandidateName(act.Name()) {
		return &handleError{
			err:           action.ErrInvalidCanName,
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	//check candidate name exists
	cand := csm.GetByName(act.Name())
	if cand == nil {
		return &handleError{
			err:           errCandNotExist,
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	//check if the caller is the owner of the candidate
	if !address.Equal(act.NewOwner(), caller) {
		return &handleError{
			err:           errors.New("operator is not authorized"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	//check Check if the delegate has been transferred
	_, ok := csm.GetNewOwner(act.Name())
	if ok {
		return &handleError{
			err:           errors.New("delegate ownership has been transferred"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	return nil
}
