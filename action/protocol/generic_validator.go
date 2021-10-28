// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// AccountState defines a function to return the account state of a given address
	AccountState func(StateReader, address.Address) (*state.Account, error)
	// GenericValidator is the validator for generic action verification
	GenericValidator struct {
		accountState AccountState
		sr           StateReader
	}
)

// NewGenericValidator constructs a new genericValidator
func NewGenericValidator(sr StateReader, accountState AccountState) *GenericValidator {
	return &GenericValidator{
		sr:           sr,
		accountState: accountState,
	}
}

// Validate validates a generic action
func (v *GenericValidator) Validate(ctx context.Context, selp action.SealedEnvelope) error {
	// Verify action using action sender's public key
	if err := action.Verify(selp); err != nil {
		return errors.Wrap(err, "failed to verify action signature")
	}
	caller := selp.SrcPubkey().Address()
	if caller == nil {
		return errors.New("failed to get address")
	}
	// Reject action if nonce is too low
	confirmedState, err := v.accountState(v.sr, caller)
	if err != nil {
		return errors.Wrapf(err, "invalid state of account %s", caller.String())
	}

	pendingNonce := confirmedState.Nonce + 1
	if selp.Nonce() > 0 && pendingNonce > selp.Nonce() {
		return errors.Wrap(action.ErrNonce, "nonce is too low")
	}
	return selp.Action().SanityCheck()
}
