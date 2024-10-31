// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

const (
	_stakeDurationLimit = 1050
)

// Errors
var (
	ErrInvalidOwner        = errors.New("invalid owner address")
	ErrInvalidOperator     = errors.New("invalid operator address")
	ErrInvalidReward       = errors.New("invalid reward address")
	ErrInvalidSelfStkIndex = errors.New("invalid self-staking bucket index")
	ErrMissingField        = errors.New("missing data field")
	ErrTypeAssertion       = errors.New("failed type assertion")
	ErrDurationTooHigh     = errors.New("stake duration cannot exceed 1050 days")
)

func (p *Protocol) validateCreateStake(ctx context.Context, act *action.CreateStake) error {
	if !action.IsValidCandidateName(act.Candidate()) {
		return action.ErrInvalidCanName
	}
	if act.Amount().Cmp(p.config.MinStakeAmount) == -1 {
		return errors.Wrap(action.ErrInvalidAmount, "stake amount is less than the minimum requirement")
	}
	if protocol.MustGetFeatureCtx(ctx).CheckStakingDurationUpperLimit && act.Duration() > _stakeDurationLimit {
		return ErrDurationTooHigh
	}
	return nil
}

func (p *Protocol) validateUnstake(ctx context.Context, act *action.Unstake) error {
	return nil
}

func (p *Protocol) validateWithdrawStake(ctx context.Context, act *action.WithdrawStake) error {
	return nil
}

func (p *Protocol) validateChangeCandidate(ctx context.Context, act *action.ChangeCandidate) error {
	if !action.IsValidCandidateName(act.Candidate()) {
		return action.ErrInvalidCanName
	}
	return nil
}

func (p *Protocol) validateTransferStake(ctx context.Context, act *action.TransferStake) error {
	return nil
}

func (p *Protocol) validateDepositToStake(ctx context.Context, act *action.DepositToStake) error {
	return nil
}

func (p *Protocol) validateRestake(ctx context.Context, act *action.Restake) error {
	if protocol.MustGetFeatureCtx(ctx).CheckStakingDurationUpperLimit && act.Duration() > _stakeDurationLimit {
		return ErrDurationTooHigh
	}
	return nil
}

func (p *Protocol) validateCandidateRegister(ctx context.Context, act *action.CandidateRegister) error {
	if !action.IsValidCandidateName(act.Name()) {
		return action.ErrInvalidCanName
	}

	if act.Amount().Cmp(p.config.RegistrationConsts.MinSelfStake) < 0 {
		if !protocol.MustGetFeatureCtx(ctx).CandidateRegisterMustWithStake &&
			act.Amount().Sign() == 0 {
			return nil
		}
		return errors.Wrap(action.ErrInvalidAmount, "self staking amount is not valid")
	}
	if protocol.MustGetFeatureCtx(ctx).CheckStakingDurationUpperLimit && act.Duration() > _stakeDurationLimit {
		return ErrDurationTooHigh
	}
	return nil
}

func (p *Protocol) validateCandidateUpdate(ctx context.Context, act *action.CandidateUpdate) error {
	if len(act.Name()) != 0 {
		if !action.IsValidCandidateName(act.Name()) {
			return action.ErrInvalidCanName
		}
	}
	return nil
}

func (p *Protocol) validateCandidateEndorsement(ctx context.Context, act *action.CandidateEndorsement) error {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.DisableDelegateEndorsement {
		return errors.Wrap(action.ErrInvalidAct, "candidate endorsement is disabled")
	}
	if featureCtx.EnforceLegacyEndorsement && !act.IsLegacy() {
		return errors.Wrap(action.ErrInvalidAct, "new candidate endorsement is disabled")
	}
	return nil
}

func (p *Protocol) validateCandidateActivate(ctx context.Context, act *action.CandidateActivate) error {
	if protocol.MustGetFeatureCtx(ctx).DisableDelegateEndorsement {
		return errors.Wrap(action.ErrInvalidAct, "candidate activate is disabled")
	}
	return nil
}

func (p *Protocol) validateCandidateTransferOwnershipAction(ctx context.Context, act *action.CandidateTransferOwnership) error {
	// TODO: remove this check after candidate transfer ownership is enabled
	if protocol.MustGetFeatureCtx(ctx).CandidateIdentifiedByOwner {
		return errors.Wrap(action.ErrInvalidAct, "candidate transfer ownership is disabled")
	}
	return nil
}

func (p *Protocol) validateMigrateStake(ctx context.Context, act *action.MigrateStake) error {
	if !protocol.MustGetFeatureCtx(ctx).MigrateNativeStake {
		return errors.Wrap(action.ErrInvalidAct, "migrate stake is disabled")
	}
	return nil
}
