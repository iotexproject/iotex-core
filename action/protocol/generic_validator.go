// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

// GenericValidator is the validator for generic action verification
type GenericValidator struct {
	mu sync.RWMutex
	cm ChainManager
}

// NewGenericValidator constructs a new genericValidator
func NewGenericValidator(cm ChainManager) *GenericValidator { return &GenericValidator{cm: cm} }

// Validate validates a generic action
func (v *GenericValidator) Validate(ctx context.Context, act action.SealedEnvelope) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// TODO skip coinbase transfer for now because nonce is wrong.
	if a, ok := act.Action().(*action.Transfer); ok && a.IsCoinbase() {
		return nil
	}
	vaCtx, validateInBlock := GetValidateActionsCtx(ctx)
	if validateInBlock {
		if vaCtx.BlockHeight == 0 {
			return nil
		}
	}
	// Reject over-gassed action
	if act.GasLimit() > genesis.ActionGasLimit {
		return errors.Wrap(action.ErrGasHigherThanLimit, "gas is higher than gas limit")
	}
	// Reject action with insufficient gas limit
	intrinsicGas, err := act.IntrinsicGas()
	if intrinsicGas > act.GasLimit() || err != nil {
		return errors.Wrap(action.ErrInsufficientBalanceForGas, "insufficient gas")
	}
	// Check if action source address is valid
	if _, err := iotxaddress.GetPubkeyHash(act.SrcAddr()); err != nil {
		return errors.Wrapf(err, "error when validating source address %s", act.SrcAddr())
	}
	// Verify action using action sender's public key
	if err := action.Verify(act); err != nil {
		return errors.Wrap(err, "failed to verify action signature")
	}
	// Reject action if nonce is too low
	confirmedNonce, err := v.cm.Nonce(act.SrcAddr())
	if err != nil {
		return errors.Wrapf(err, "invalid nonce value of account %s", act.SrcAddr())
	}
	pendingNonce := confirmedNonce + 1
	if pendingNonce > act.Nonce() {
		return errors.Wrap(action.ErrNonce, "nonce is too low")
	}
	// Check if action's nonce is in correct order
	if validateInBlock {
		value, _ := vaCtx.NonceTracker.Load(act.SrcAddr())
		nonceList, ok := value.([]uint64)
		if !ok {
			return errors.Errorf("failed to load received nonces for account %s", act.SrcAddr())
		}
		vaCtx.NonceTracker.Store(act.SrcAddr(), append(nonceList, act.Nonce()))
	}
	return nil
}
