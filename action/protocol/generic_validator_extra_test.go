// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// buildValidateWithStateCtx returns a context at a very high block height so
// that all fork-gated features (Okhotsk balance guarantee, Vanuatu dynamic
// fee, etc.) are active and deterministic.
func buildValidateWithStateCtx(baseFee *big.Int) context.Context {
	ctx := WithBlockCtx(context.Background(), BlockCtx{
		BlockHeight: 1_000_000_000,
		Producer:    identityset.Address(27),
		BaseFee:     baseFee,
	})
	ctx = WithActionCtx(ctx, ActionCtx{Caller: identityset.Address(28)})
	return WithFeatureCtx(genesis.WithGenesisContext(ctx, genesis.TestDefault()))
}

func accountWithBalanceAndNonce(t *testing.T, balance *big.Int, pendingNonce uint64) *state.Account {
	acct, err := state.NewAccount()
	require.NoError(t, err)
	for i := uint64(0); i < pendingNonce; i++ {
		require.NoError(t, acct.SetPendingNonce(i+1))
	}
	acct.Balance = balance
	return acct
}

func signedExecution(t *testing.T, nonce uint64, gasLimit uint64, gasFeeCap, gasTipCap *big.Int) *action.SealedEnvelope {
	exec := action.NewExecution("", big.NewInt(0), nil)
	elp := (&action.EnvelopeBuilder{}).SetTxType(action.DynamicFeeTxType).SetChainID(0).
		SetNonce(nonce).SetGasLimit(gasLimit).SetDynamicGas(gasFeeCap, gasTipCap).
		SetAction(exec).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(28))
	require.NoError(t, err)
	return selp
}

func TestValidateWithState_SystemAction(t *testing.T) {
	r := require.New(t)
	ctx := buildValidateWithStateCtx(nil)
	v := NewGenericValidator(nil, func(context.Context, StateReader, address.Address) (*state.Account, error) {
		return nil, errors.New("should not be called for system action")
	})
	gr := action.NewGrantReward(action.BlockReward, 2)
	elp := (&action.EnvelopeBuilder{}).SetGasLimit(100000).SetAction(gr).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(28))
	r.NoError(err)
	// system actions bypass all state checks
	r.NoError(v.ValidateWithState(ctx, selp))
}

func TestValidateWithState_NonceTooLow(t *testing.T) {
	r := require.New(t)
	ctx := buildValidateWithStateCtx(nil)
	// pending nonce 5 means an action with nonce 3 is stale
	acct := accountWithBalanceAndNonce(t, big.NewInt(1e18), 5)
	v := NewGenericValidator(nil, func(context.Context, StateReader, address.Address) (*state.Account, error) {
		return acct, nil
	})
	selp := signedExecution(t, 3, 100000, big.NewInt(1000), big.NewInt(1))
	err := v.ValidateWithState(ctx, selp)
	r.ErrorIs(errors.Cause(err), action.ErrNonceTooLow)
}

func TestValidateWithState_AccountStateError(t *testing.T) {
	r := require.New(t)
	ctx := buildValidateWithStateCtx(nil)
	v := NewGenericValidator(nil, func(context.Context, StateReader, address.Address) (*state.Account, error) {
		return nil, errors.New("db down")
	})
	selp := signedExecution(t, 10, 100000, big.NewInt(1000), big.NewInt(1))
	err := v.ValidateWithState(ctx, selp)
	r.Error(err)
	r.Contains(err.Error(), "invalid state of account")
}

func TestValidateWithState_InsufficientBalance(t *testing.T) {
	r := require.New(t)
	ctx := buildValidateWithStateCtx(nil)
	// nonce high enough not to trip the nonce check; balance far too small
	acct := accountWithBalanceAndNonce(t, big.NewInt(1), 0)
	v := NewGenericValidator(nil, func(context.Context, StateReader, address.Address) (*state.Account, error) {
		return acct, nil
	})
	selp := signedExecution(t, 100, 100000, big.NewInt(1000), big.NewInt(1))
	err := v.ValidateWithState(ctx, selp)
	r.ErrorIs(errors.Cause(err), state.ErrNotEnoughBalance)
}

func TestValidateWithState_CannotCoverBaseFee(t *testing.T) {
	r := require.New(t)
	// baseFee larger than the action's gasFeeCap
	ctx := buildValidateWithStateCtx(big.NewInt(1_000_000))
	acct := accountWithBalanceAndNonce(t, new(big.Int).SetUint64(1e18), 0)
	v := NewGenericValidator(nil, func(context.Context, StateReader, address.Address) (*state.Account, error) {
		return acct, nil
	})
	selp := signedExecution(t, 100, 100000, big.NewInt(1000), big.NewInt(1))
	err := v.ValidateWithState(ctx, selp)
	r.Error(err)
	r.Contains(err.Error(), "cannot cover base fee")
}

func TestValidateWithState_Success(t *testing.T) {
	r := require.New(t)
	ctx := buildValidateWithStateCtx(big.NewInt(1))
	acct := accountWithBalanceAndNonce(t, new(big.Int).SetUint64(1e18), 0)
	v := NewGenericValidator(nil, func(context.Context, StateReader, address.Address) (*state.Account, error) {
		return acct, nil
	})
	selp := signedExecution(t, 100, 100000, big.NewInt(1_000_000), big.NewInt(1))
	r.NoError(v.ValidateWithState(ctx, selp))
}
