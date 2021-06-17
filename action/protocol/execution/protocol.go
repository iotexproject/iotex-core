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
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// ExecutionSizeLimit is the maximum size of execution allowed
	ExecutionSizeLimit = 32 * 1024
	// TODO: it works only for one instance per protocol definition now
	protocolID = "smart_contract"
)

// Protocol defines the protocol of handling executions
type Protocol struct {
	getBlockHash evm.GetBlockHash
	depositGas   evm.DepositGas
	addr         address.Address
}

// NewProtocol instantiates the protocol of exeuction
func NewProtocol(getBlockHash evm.GetBlockHash, depostGas evm.DepositGas) *Protocol {
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of vote protocol", zap.Error(err))
	}
	return &Protocol{getBlockHash: getBlockHash, depositGas: depostGas, addr: addr}
}

// FindProtocol finds the registered protocol from registry
func FindProtocol(registry *protocol.Registry) *Protocol {
	if registry == nil {
		return nil
	}
	p, ok := registry.Find(protocolID)
	if !ok {
		return nil
	}
	ep, ok := p.(*Protocol)
	if !ok {
		log.S().Panic("fail to cast execution protocol")
	}
	return ep
}

// Handle handles an execution
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	exec, ok := act.(*action.Execution)
	if !ok {
		return nil, nil
	}
	_, receipt, err := evm.ExecuteContract(ctx, sm, exec, p.getBlockHash, p.depositGas)

	if err != nil {
		return nil, errors.Wrap(err, "failed to execute contract")
	}

	return receipt, nil
}

// Validate validates the size of an execution
func (p *Protocol) Validate(_ context.Context, act action.Action, _ protocol.StateReader) error {
	if exec, ok := act.(*action.Execution); ok {
		if exec.TotalSize() > ExecutionSizeLimit {
			return errors.Wrap(action.ErrActPool, "oversized data")
		}
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, uint64(0), protocol.ErrUnimplemented
}

// Register registers the protocol with a unique ID
func (p *Protocol) Register(r *protocol.Registry) error {
	return r.Register(protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *Protocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, p)
}

// Name returns the name of protocol
func (p *Protocol) Name() string {
	return protocolID
}
