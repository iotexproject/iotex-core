// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
)

// Errors
var (
	ErrInvalidAmount       = errors.New("invalid staking amount")
	ErrInvalidCanName      = errors.New("invalid candidate name")
	ErrInvalidOwner        = errors.New("invalid owner address")
	ErrInvalidOperator     = errors.New("invalid operator address")
	ErrInvalidReward       = errors.New("invalid reward address")
	ErrInvalidSelfStkIndex = errors.New("invalid self-staking bucket index")
	ErrMissingField        = errors.New("missing data field")
	ErrTypeAssertion       = errors.New("failed type assertion")
)

func (p *Protocol) validateCreateStake(ctx context.Context, act *action.CreateStake) error {
	if !isValidCandidateName(act.Candidate()) {
		return ErrInvalidCanName
	}
	if act.Amount().Cmp(p.config.MinStakeAmount) == -1 {
		return errors.Wrap(ErrInvalidAmount, "stake amount is less than the minimum requirement")
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
	if !isValidCandidateName(act.Candidate()) {
		return ErrInvalidCanName
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
	return nil
}

func (p *Protocol) validateCandidateRegister(ctx context.Context, act *action.CandidateRegister) error {
	if !isValidCandidateName(act.Name()) {
		return ErrInvalidCanName
	}

	if act.Amount().Cmp(p.config.RegistrationConsts.MinSelfStake) < 0 {
		return errors.Wrap(ErrInvalidAmount, "self staking amount is not valid")
	}
	return nil
}

func (p *Protocol) validateCandidateUpdate(ctx context.Context, act *action.CandidateUpdate) error {
	if len(act.Name()) != 0 {
		if !isValidCandidateName(act.Name()) {
			return ErrInvalidCanName
		}
	}
	return nil
}

// IsValidCandidateName check if a candidate name string is valid.
func isValidCandidateName(s string) bool {
	if len(s) == 0 || len(s) > 12 {
		return false
	}
	for _, c := range s {
		if !(('a' <= c && c <= 'z') || ('0' <= c && c <= '9')) {
			return false
		}
	}
	return true
}
