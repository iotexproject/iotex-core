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

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
	depositGas protocol.DepositGas
	addr       address.Address
}

// NewProtocol instantiates the protocol of exeuction
// TODO: remove unused getBlockHash and getBlockTime
func NewProtocol(_ evm.GetBlockHash, depositGas protocol.DepositGas, _ evm.GetBlockTime) *Protocol {
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of vote protocol", zap.Error(err))
	}
	return &Protocol{depositGas: depositGas, addr: addr}
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
func (p *Protocol) Handle(ctx context.Context, elp action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
	if _, ok := elp.Action().(*action.Execution); !ok {
		return nil, nil
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
		GetBlockHash:   bcCtx.GetBlockHash,
		GetBlockTime:   bcCtx.GetBlockTime,
		DepositGasFunc: p.depositGas,
	})
	_, receipt, err := evm.ExecuteContract(ctx, sm, elp)

	if err != nil {
		return nil, errors.Wrap(err, "failed to execute contract")
	}
	return receipt, nil
}

// Validate validates an execution
func (p *Protocol) Validate(ctx context.Context, elp action.Envelope, _ protocol.StateReader) error {
	exec, ok := elp.Action().(*action.Execution)
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
		dataSize = elp.Size()
	}

	// Reject oversize execution
	if dataSize > sizeLimit {
		return action.ErrOversizedData
	}
	if len(elp.BlobHashes()) > 0 && elp.To() == nil {
		return errors.New("cannot create contract in blob tx")
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
