// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// AccountState defines a function to return the account state of a given address
	AccountState func(context.Context, StateReader, address.Address) (*state.Account, error)
	// GenericValidator is the validator for generic action verification
	GenericValidator struct {
		accountState AccountState
		sr           StateReader
	}
)

var (
	// MinTipCap is the minimum tip cap
	MinTipCap = big.NewInt(1)
)

// NewGenericValidator constructs a new genericValidator
func NewGenericValidator(sr StateReader, accountState AccountState) *GenericValidator {
	return &GenericValidator{
		sr:           sr,
		accountState: accountState,
	}
}

// Validate validates a generic action
func (v *GenericValidator) Validate(ctx context.Context, selp *action.SealedEnvelope) error {
	intrinsicGas, err := selp.IntrinsicGas()
	if err != nil {
		return err
	}
	if intrinsicGas > selp.Gas() {
		return action.ErrIntrinsicGas
	}

	// Verify action using action sender's public key
	if err := selp.VerifySignature(); err != nil {
		return err
	}
	caller := selp.SenderAddress()
	if caller == nil {
		return errors.New("failed to get address")
	}
	if err = selp.Envelope.SanityCheck(); err != nil {
		return err
	}
	// Reject action if nonce is too low
	if action.IsSystemAction(selp) {
		if selp.Nonce() != 0 {
			return action.ErrSystemActionNonce
		}
	} else {
		var (
			nonce          uint64
			featureCtx, ok = GetFeatureCtx(ctx)
		)
		if ok && featureCtx.FixGasAndNonceUpdate || selp.Nonce() != 0 {
			confirmedState, err := v.accountState(ctx, v.sr, caller)
			if err != nil {
				return errors.Wrapf(err, "invalid state of account %s", caller.String())
			}
			if featureCtx.UseZeroNonceForFreshAccount {
				nonce = confirmedState.PendingNonceConsideringFreshAccount()
			} else {
				nonce = confirmedState.PendingNonce()
			}
			if nonce > selp.Nonce() {
				return action.ErrNonceTooLow
			}
		}
		if ok && !featureCtx.EnableAccessListTx && selp.TxType() == action.AccessListTxType {
			return errors.Wrap(action.ErrInvalidAct, "access list tx is not enabled")
		}
		if ok && !featureCtx.EnableDynamicFeeTx && selp.TxType() == action.DynamicFeeTxType {
			return errors.Wrap(action.ErrInvalidAct, "dynamic fee tx is not enabled")
		}
		if ok && !featureCtx.EnableBlobTransaction && selp.TxType() == action.BlobTxType {
			return errors.Wrap(action.ErrInvalidAct, "blob tx is not enabled")
		}
		if ok && featureCtx.EnableDynamicFeeTx {
			// check transaction's max fee can cover base fee
			blkCtx := MustGetBlockCtx(ctx)
			if baseFee := blkCtx.BaseFee; baseFee != nil && selp.Envelope.GasFeeCap().Cmp(baseFee) < 0 {
				return errors.Errorf("transaction cannot cover base fee, max fee = %s, base fee = %s",
					selp.Envelope.GasFeeCap().String(), baseFee.String())
			}
			if selp.Envelope.GasTipCap().Cmp(MinTipCap) < 0 {
				return errors.Wrapf(action.ErrUnderpriced, "tip cap is too low: %s, min tip cap: %s", selp.Envelope.GasTipCap().String(), MinTipCap.String())
			}
		}
		if ok && featureCtx.SufficentBalanceGuarantee {
			acc, err := v.accountState(ctx, v.sr, caller)
			if err != nil {
				return errors.Wrapf(err, "invalid state of account %s", caller.String())
			}
			cost, err := selp.Cost()
			if err != nil {
				return errors.Wrap(err, "failed to get cost of action")
			}
			if acc.Balance.Cmp(cost) < 0 {
				return errors.Wrapf(state.ErrNotEnoughBalance, "sender %s balance %s, cost %s", caller.String(), acc.Balance, cost)
			}
		}
		if ok && featureCtx.EnableBlobTransaction && len(selp.BlobHashes()) > 0 {
			// blobFeeCap must be not less than the blob price
			basefee := CalcBlobFee(MustGetBlockCtx(ctx).ExcessBlobGas)
			if selp.BlobGasFeeCap().Cmp(basefee) < 0 {
				return errors.Wrapf(action.ErrUnderpriced, "blob fee cap is too low: %s, base fee: %s", selp.BlobGasFeeCap().String(), basefee.String())
			}
			// validate sidecar
			if blkCtx, ok := GetBlockCtx(ctx); (ok && !blkCtx.SkipSidecarValidation) || selp.BlobTxSidecar() != nil {
				if err := selp.ValidateSidecar(); err != nil {
					return errors.Wrap(err, "failed to validate blob sidecar")
				}
			}
		}
	}
	return nil
}
