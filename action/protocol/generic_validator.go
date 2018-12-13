package protocol

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

// GasLimit is the total gas limit could be consumed in a block
const GasLimit = uint64(1000000000)

// GenericValidator is the validator for generic action verification
type GenericValidator struct {
	mu sync.RWMutex
	cm ChainManager
}

// NewGenericValidator constructs a new genericValidator
func NewGenericValidator(cm ChainManager) *GenericValidator { return &GenericValidator{cm: cm} }

// Validate validates a generic action
func (v *GenericValidator) Validate(ctx context.Context, act action.Action) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	vaCtx, validateInBlock := GetValidateActionsCtx(ctx)
	if validateInBlock && (vaCtx.BlockHeight == 0 || act.SrcAddr() == "") {
		return nil
	}
	// Reject over-gassed action
	if act.GasLimit() > GasLimit {
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
		vaCtx.NonceTracker[act.SrcAddr()] = append(vaCtx.NonceTracker[act.SrcAddr()], act.Nonce())
	}
	return nil
}
