// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/unit"
)

// Errors
var (
	ErrNilAction           = errors.New("action is nil")
	ErrInvalidAmount       = errors.New("invalid staking amount")
	ErrInvalidCanName      = errors.New("invalid candidate name")
	ErrInvalidOwner        = errors.New("invalid owner address")
	ErrInvalidOperator     = errors.New("invalid operator address")
	ErrInvalidSelfStkIndex = errors.New("invalid self-staking bucket index")
	ErrMissingField        = errors.New("missing data field")
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
	if act.Amount().Cmp(unit.ConvertIotxToRau(p.config.MinStakeAmount)) == -1 {
		return errors.Wrap(ErrInvalidAmount, "stake amount is less than the minimum requirement")
	}
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	if !p.inMemCandidates.ContainsName(act.Candidate()) {
		return errors.Wrap(ErrInvalidCanName, "cannot find candidate in candidate center")
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
	if !p.inMemCandidates.ContainsName(act.Candidate()) {
		return errors.Wrap(ErrInvalidCanName, "cannot find candidate in candidate center")
	}
	return nil
}

func (p *Protocol) validateTransferStake(ctx context.Context, act *action.TransferStake) error {
	if act == nil {
		return ErrNilAction
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
	return nil
}

func (p *Protocol) validateCandidateRegister(ctx context.Context, act *action.CandidateRegister) error {
	if act == nil {
		return ErrNilAction
	}

	actCtx := protocol.MustGetActionCtx(ctx)
	if act.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}

	if !IsValidCandidateName(act.Name()) {
		return ErrInvalidCanName
	}

	if act.OperatorAddress() == nil || act.RewardAddress() == nil {
		return errors.New("empty addresses")
	}

	minSelfStake := unit.ConvertIotxToRau(p.config.RegistrationConsts.MinSelfStake)
	if act.Amount() == nil || act.Amount().Cmp(minSelfStake) < 0 {
		return errors.New("self staking amount is not valid")
	}

	owner := actCtx.Caller
	if act.OwnerAddress() != nil {
		owner = act.OwnerAddress()
	}

	if c := p.inMemCandidates.GetByOwner(owner); c != nil {
		// an existing owner, but selfstake is 0
		if c.SelfStake.Cmp(big.NewInt(0)) != 0 {
			return ErrInvalidOwner
		}
		if act.Name() != c.Name && p.inMemCandidates.ContainsName(act.Name()) {
			return ErrInvalidCanName
		}
		if act.OperatorAddress() != c.Operator && p.inMemCandidates.ContainsOperator(act.OperatorAddress()) {
			return ErrInvalidOperator
		}
		return nil
	}

	// cannot collide with existing name
	if p.inMemCandidates.ContainsName(act.Name()) {
		return ErrInvalidCanName
	}

	// cannot collide with existing operator address
	if p.inMemCandidates.ContainsOperator(act.OperatorAddress()) {
		return ErrInvalidOperator
	}
	return nil
}

func (p *Protocol) validateCandidateUpdate(ctx context.Context, act *action.CandidateUpdate) error {
	actCtx := protocol.MustGetActionCtx(ctx)

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

	// only owner can update candidate
	c := p.inMemCandidates.GetByOwner(actCtx.Caller)
	if c == nil {
		return ErrInvalidOwner
	}

	// cannot collide with existing name
	if len(act.Name()) != 0 && act.Name() != c.Name && p.inMemCandidates.ContainsName(act.Name()) {
		return ErrInvalidCanName
	}

	// cannot collide with existing operator address
	if act.OperatorAddress() != nil && act.OperatorAddress() != c.Operator && p.inMemCandidates.ContainsOperator(act.OperatorAddress()) {
		return ErrInvalidOperator
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
