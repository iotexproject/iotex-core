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
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if act.Amount().Cmp(p.config.RegistrationConsts.MinSelfStake) < 0 {
		if !featureCtx.CandidateRegisterMustWithStake &&
			act.Amount().Sign() == 0 {
			return nil
		}
		return errors.Wrap(action.ErrInvalidAmount, "self staking amount is not valid")
	}
	if featureCtx.CheckStakingDurationUpperLimit && act.Duration() > _stakeDurationLimit {
		return ErrDurationTooHigh
	}
	switch {
	case !featureCtx.CandidateBLSPublicKey:
		// Pre-Xingu: BLS is not part of the action at all, and the amount
		// travels in the legacy field.
		if act.WithBLS() || act.Value() != nil {
			return errors.Wrap(action.ErrInvalidAct, "candidate registration cannot include BLS public key or value")
		}
	case featureCtx.OptionalCandidateBLSPublicKey:
		// Post-fork: the key is optional.
		//
		// A registration that supplies a key still has to use the
		// value-carrying ABI, as it has since Xingu. One that omits the key
		// has no such method to call -- candidateRegisterWithBLS* reject an
		// empty blsPubKey at decode time -- so it goes back through the
		// legacy candidateRegister entry, whose amount lands in the legacy
		// field. Both conventions therefore coexist post-fork, and which one
		// Amount() reads is already keyed off WithBLS().
		if act.WithBLS() && act.LegacyAmount() != nil {
			return errors.Wrap(action.ErrInvalidAct, "candidate registration with BLS public key cannot include legacy amount")
		}
		if err := validateBLSPairing(act.WithBLS(), act.BLSPop()); err != nil {
			return err
		}
	default:
		// Xingu until the fork: the key is mandatory.
		if !act.WithBLS() || act.LegacyAmount() != nil {
			return errors.Wrap(action.ErrInvalidAct, "candidate registration must include BLS public key and cannot include legacy amount")
		}
	}

	return nil
}

// validateBLSPairing rejects a PoP that arrives without the public key it is
// supposed to attest to.
//
// The mirror case -- a key with no PoP -- is deliberately not checked here.
// EnforceBLSPoP activates on the same fork, and the register and update
// handlers already reject it through VerifyBLSPop, surfacing
// ErrUnauthorizedOperator on the receipt. Duplicating the rule here would move
// that rejection from "included in a block with a failure receipt" to "refused
// at validation", changing whether the sender pays gas for it.
func validateBLSPairing(withBLS bool, pop []byte) error {
	if !withBLS && len(pop) > 0 {
		return errors.Wrap(action.ErrInvalidAct, "BLS proof-of-possession must be accompanied by a public key")
	}
	return nil
}

func (p *Protocol) validateCandidateUpdate(ctx context.Context, act *action.CandidateUpdate) error {
	if len(act.Name()) != 0 {
		if !action.IsValidCandidateName(act.Name()) {
			return action.ErrInvalidCanName
		}
	}
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	switch {
	case !featureCtx.CandidateBLSPublicKey:
		if act.WithBLS() {
			return errors.Wrap(action.ErrInvalidAct, "candidate update cannot include BLS public key")
		}
	case featureCtx.OptionalCandidateBLSPublicKey:
		// Post-fork: omitting both fields leaves any registered key as it is;
		// a PoP without a key is rejected.
		if err := validateBLSPairing(act.WithBLS(), act.BLSPop()); err != nil {
			return err
		}
	default:
		if !act.WithBLS() {
			return errors.Wrap(action.ErrInvalidAct, "candidate update must include BLS public key")
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

func (p *Protocol) validateCandidateDeactivate(ctx context.Context, act *action.CandidateDeactivate) error {
	if protocol.MustGetFeatureCtx(ctx).NoCandidateExitQueue {
		return errors.Wrap(action.ErrInvalidAct, "candidate deactivation is disabled")
	}
	return nil
}
