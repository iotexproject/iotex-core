// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package execution

import (
	"context"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// ExecutionSizeLimit is the maximum size of execution allowed
	ExecutionSizeLimit = 32 * 1024
	// ProtocolID is the protocol ID
	// TODO: it works only for one instance per protocol definition now
	ProtocolID = "smart_contract"
)

// Protocol defines the protocol of handling executions
type Protocol struct {
	cm   protocol.ChainManager
	addr address.Address
	hu   config.HeightUpgrade
}

// NewProtocol instantiates the protocol of exeuction
func NewProtocol(cm protocol.ChainManager, hu config.HeightUpgrade) *Protocol {
	h := hash.Hash160b([]byte(ProtocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of vote protocol", zap.Error(err))
	}
	return &Protocol{cm: cm, addr: addr, hu: hu}
}

// Handle handles an execution
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	exec, ok := act.(*action.Execution)
	if !ok {
		return nil, nil
	}
	_, receipt, err := evm.ExecuteContract(ctx, sm, exec, p.cm, p.hu)

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
	// Reject execution of negative gas price
	if exec.GasPrice().Sign() < 0 {
		return errors.Wrap(action.ErrGasPrice, "negative value")
	}
	// check if contract's address is valid
	if exec.Contract() != action.EmptyAddress {
		if _, err := address.FromString(exec.Contract()); err != nil {
			return errors.Wrapf(err, "error when validating contract's address %s", exec.Contract())
		}
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}
