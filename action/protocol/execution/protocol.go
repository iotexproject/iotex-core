// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package execution

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// the maximum size of execution allowed
	_executionSizeLimit48KB = uint32(48 * 1024)
	_executionSizeLimit32KB = uint32(32 * 1024)
	// TODO: it works only for one instance per protocol definition now
	_protocolID = "smart_contract"
)

// Protocol defines the protocol of handling executions
type Protocol struct {
	getBlockHash evm.GetBlockHash
	getBlockTime evm.GetBlockTime
	depositGas   evm.DepositGasWithSGD
	addr         address.Address
	sgdRegistry  evm.SGDRegistry
}

// NewProtocol instantiates the protocol of exeuction
func NewProtocol(getBlockHash evm.GetBlockHash, depositGasWithSGD evm.DepositGasWithSGD, sgd evm.SGDRegistry, getBlockTime evm.GetBlockTime) *Protocol {
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of vote protocol", zap.Error(err))
	}
	return &Protocol{getBlockHash: getBlockHash, depositGas: depositGasWithSGD, addr: addr, sgdRegistry: sgd, getBlockTime: getBlockTime}
}

// FindProtocol finds the registered protocol from registry
func FindProtocol(registry *protocol.Registry) *Protocol {
	if registry == nil {
		return nil
	}
	p, ok := registry.Find(_protocolID)
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
	ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
		GetBlockHash:   p.getBlockHash,
		GetBlockTime:   p.getBlockTime,
		DepositGasFunc: p.depositGas,
		Sgd:            p.sgdRegistry,
	})
	_, receipt, err := evm.ExecuteContract(ctx, sm, action.NewEvmTx(exec))

	if err != nil {
		return nil, errors.Wrap(err, "failed to execute contract")
	}

	return receipt, nil
}

// HandleCrossProtocol handles an execution from another protocol
func (p *Protocol) HandleCrossProtocol(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	receipt, err := p.Handle(ctx, act, sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute contract")
	}

	// reset caller nonce
	acc, err := accountutil.AccountState(ctx, sm, protocol.MustGetActionCtx(ctx).Caller)
	if err != nil {
		log.L().Panic("failed to get account state", zap.Error(err))
	}
	acc.DecreaseNonce()
	if err := accountutil.StoreAccount(sm, protocol.MustGetActionCtx(ctx).Caller, acc); err != nil {
		log.L().Panic("failed to store account", zap.Error(err))
	}
	return receipt, nil
}

// Validate validates an execution
func (p *Protocol) Validate(ctx context.Context, act action.Action, _ protocol.StateReader) error {
	exec, ok := act.(*action.Execution)
	if !ok {
		return nil
	}
	var (
		sizeLimit = _executionSizeLimit48KB
		dataSize  = uint32(len(exec.Data()))
	)
	fCtx := protocol.MustGetFeatureCtx(ctx)
	if fCtx.ExecutionSizeLimit32KB {
		sizeLimit = _executionSizeLimit32KB
		dataSize = exec.TotalSize()
	}

	// Reject oversize execution
	if dataSize > sizeLimit {
		return action.ErrOversizedData
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, uint64(0), protocol.ErrUnimplemented
}

// Register registers the protocol with a unique ID
func (p *Protocol) Register(r *protocol.Registry) error {
	return r.Register(_protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *Protocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(_protocolID, p)
}

// Name returns the name of protocol
func (p *Protocol) Name() string {
	return _protocolID
}
