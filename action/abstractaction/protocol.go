package abstractaction

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/state"
)

// Protocol defines the protocol of handling abstract action
type Protocol struct{ bc blockchain.Blockchain }

// NewProtocol instantiates the protocol of abstract action
func NewProtocol(bc blockchain.Blockchain) *Protocol { return &Protocol{bc} }

// Handle handles an abstract action
func (p *Protocol) Handle(act action.Action, ws state.WorkingSet) error {
	return nil
}

// Validate validates an abstract action
func (p *Protocol) Validate(act action.Action) error {
	// Reject over-gassed action
	if act.GasLimit() > blockchain.GasLimit {
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
	confirmedNonce, err := p.bc.Nonce(act.SrcAddr())
	if err != nil {
		return errors.Wrapf(err, "invalid nonce value of account %s", act.SrcAddr())
	}
	pendingNonce := confirmedNonce + 1
	if pendingNonce > act.Nonce() {
		return errors.Wrap(action.ErrNonce, "nonce is too low")
	}
	return nil
}
