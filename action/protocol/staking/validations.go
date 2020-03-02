// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/unit"
)

// DurationBase is the base multiple of staking duration
const DurationBase = 7

// Errors
var (
	ErrNilAction       = errors.New("action is nil")
	ErrInvalidAmount   = errors.New("invalid staking amount")
	ErrInvalidCanName  = errors.New("invalid candidate name")
	ErrInvalidOwner    = errors.New("invalid owner address")
	ErrInvalidOperator = errors.New("invalid operator address")
	ErrMissingField    = errors.New("misssing data field")
	ErrInvalidDuration = errors.New("invalid staking duration")
)

func (p *Protocol) validateCreateStake(ctx context.Context, act *action.CreateStake) error {
	if act == nil {
		return ErrNilAction
	}
	if !IsValidCandidateName(act.Candidate()) {
		return ErrInvalidCanName
	}
	if act.Amount().Sign() <= 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	if act.Duration()%DurationBase != 0 {
		return ErrInvalidDuration
	}
	return nil
}

func (p *Protocol) validateUnstake(ctx context.Context, act *action.Unstake) error {
	if act == nil {
		return ErrNilAction
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	return nil
}

func (p *Protocol) validateWithdrawStake(ctx context.Context, act *action.WithdrawStake) error {
	if act == nil {
		return ErrNilAction
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	return nil
}

func (p *Protocol) validateChangeCandidate(ctx context.Context, act *action.ChangeCandidate) error {
	if act == nil {
		return ErrNilAction
	}
	if !IsValidCandidateName(act.Candidate()) {
		return ErrInvalidCanName
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	return nil
}

func (p *Protocol) validateTransferStake(ctx context.Context, act *action.TransferStake) error {
	if act == nil {
		return ErrNilAction
	}
	if _, err := address.FromString(act.VoterAddress()); err != nil {
		return errors.Wrap(address.ErrInvalidAddr, "invalid voter address")
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	return nil
}

func (p *Protocol) validateDepositToStake(ctx context.Context, act *action.DepositToStake) error {
	if act == nil {
		return ErrNilAction
	}
	if act.Amount().Sign() <= 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	return nil
}

func (p *Protocol) validateRestake(ctx context.Context, act *action.Restake) error {
	if act == nil {
		return ErrNilAction
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	if act.Duration()%DurationBase != 0 {
		return ErrInvalidDuration
	}
	return nil
}

func (p *Protocol) validateCandidateRegister(ctx context.Context, act *action.CandidateRegister) error {
	if act == nil {
		return ErrNilAction
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	if !IsValidCandidateName(act.Name()) {
		return ErrInvalidCanName
	}

	if act.OperatorAddress() == nil || act.RewardAddress() == nil {
		return errors.New("empty addresses")
	}

	minSelfStake := unit.ConvertIotxToRau(1200000)
	if act.Amount() == nil || act.Amount().Cmp(minSelfStake) < 0 {
		return errors.New("self staking amount is not valid")
	}

	if act.Duration()%DurationBase != 0 {
		return ErrInvalidDuration
	}

	return nil
}

func (p *Protocol) validateCandidateUpdate(ctx context.Context, act *action.CandidateUpdate) error {
	if act == nil {
		return ErrNilAction
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	if len(act.Name()) != 0 {
		if !IsValidCandidateName(act.Name()) {
			return ErrInvalidCanName
		}
	}
	return nil
}

// IsValidCandidateName check if a candidate name string is valid.
func IsValidCandidateName(s string) bool {
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
