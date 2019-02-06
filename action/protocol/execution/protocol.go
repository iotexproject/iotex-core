// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package execution

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/address"
)

// ExecutionSizeLimit is the maximum size of execution allowed
const ExecutionSizeLimit = 32 * 1024

// Protocol defines the protocol of handling executions
type Protocol struct {
	cm protocol.ChainManager
}

// NewProtocol instantiates the protocol of exeuction
func NewProtocol(cm protocol.ChainManager) *Protocol { return &Protocol{cm: cm} }

// Handle handles an execution
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	exec, ok := act.(*action.Execution)
	if !ok {
		return nil, nil
	}

	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		return nil, errors.New("failed to get RunActionsCtx")
	}
	receipt, err := evm.ExecuteContract(raCtx.BlockHeight, raCtx.BlockHash, raCtx.Producer, raCtx.BlockTimeStamp,
		sm, exec, p.cm, raCtx.GasLimit, raCtx.EnableGasCharge)

	if err != nil {
		return nil, errors.Wrap(err, "failed to execute contract")
	}

	return receipt, nil
}

// Validate validates an execution
func (p *Protocol) Validate(_ context.Context, act action.Action) error {
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
		if _, err := address.Bech32ToAddress(exec.Contract()); err != nil {
			return errors.Wrapf(err, "error when validating contract's address %s", exec.Contract())
		}
	}
	return nil
}
