// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// AccountState defines a function to return the account state of a given address
	AccountState func(context.Context, StateReader, address.Address) (*state.Account, error)
	// GenericValidator is the validator for generic action verification
	GenericValidator struct {
		accountState  AccountState
		sr            StateReader
		validatedActs *cache.ThreadSafeLruCache
	}
)

const (
	_maxItems = 3e4
)

// NewGenericValidator constructs a new genericValidator
func NewGenericValidator(sr StateReader, accountState AccountState) *GenericValidator {
	return &GenericValidator{
		sr:            sr,
		accountState:  accountState,
		validatedActs: cache.NewThreadSafeLruCache(_maxItems),
	}
}

// Validate validates a generic action
func (v *GenericValidator) Validate(ctx context.Context, selp action.SealedEnvelope) error {
	actHash, _ := selp.Hash()

	if _, exist := v.validatedActs.Get(actHash); exist {
		return nil
	}

	intrinsicGas, err := selp.IntrinsicGas()
	if err != nil {
		return err
	}
	if intrinsicGas > selp.GasLimit() {
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
	// Reject action if nonce is too low
	if action.IsSystemAction(selp) {
		if selp.Nonce() != 0 {
			return action.ErrSystemActionNonce
		}
	} else {
		featureCtx, ok := GetFeatureCtx(ctx)
		if ok && featureCtx.FixGasAndNonceUpdate || selp.Nonce() != 0 {
			confirmedState, err := v.accountState(ctx, v.sr, caller)
			if err != nil {
				return errors.Wrapf(err, "invalid state of account %s", caller.String())
			}
			if confirmedState.PendingNonce() > selp.Nonce() {
				return action.ErrNonceTooLow
			}
		}
	}

	if err := selp.Action().SanityCheck(); err != nil {
		return err
	}
	// For testing
	v.validatedActs.Add(actHash, struct{}{})
	return nil
}
