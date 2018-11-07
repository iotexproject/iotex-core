// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package execution

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/state"
)

// ExecutionSizeLimit is the maximum size of execution allowed
const ExecutionSizeLimit = 32 * 1024

// Protocol defines the protocol of handling executions
type Protocol struct{}

// NewProtocol instantiates the protocol of exeuction
func NewProtocol() *Protocol { return &Protocol{} }

// Handle handles an execution
func (p *Protocol) Handle(act action.Action, ws state.WorkingSet) (*action.Receipt, error) {
	exec, ok := act.(*action.Execution)
	if !ok {
		return nil, nil
	}
	executorPKHash, err := iotxaddress.AddressToPKHash(exec.Executor())
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert address to PK hash")
	}
	s, err := ws.CachedState(executorPKHash, &state.Account{})
	if err != nil {
		return nil, errors.Wrap(err, "executor does not exist")
	}
	account, ok := s.(*state.Account)
	if !ok {
		return nil, errors.Wrap(err, "failed to convert state to account state")
	}
	if exec.Nonce() > account.Nonce {
		account.Nonce = exec.Nonce()
		ws.UpdateCachedStates(executorPKHash, account)
	}
	return nil, nil
}

// Validate validates an execution
func (p *Protocol) Validate(act action.Action) error {
	exec, ok := act.(*action.Execution)
	if !ok {
		return nil
	}
	// Reject oversized exeuction
	if exec.TotalSize() > ExecutionSizeLimit {
		return errors.Wrap(action.ErrActPool, "oversized data")
	}
	// Reject execution of negative amount
	if exec.Amount().Sign() < 0 {
		return errors.Wrap(action.ErrBalance, "negative value")
	}
	// check if contract's address is valid
	if exec.Contract() != action.EmptyAddress {
		if _, err := iotxaddress.GetPubkeyHash(exec.Contract()); err != nil {
			return errors.Wrapf(err, "error when validating contract's address %s", exec.Contract())
		}
	}
	return nil
}
