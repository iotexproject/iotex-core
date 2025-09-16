package evm

import (
	"context"
	"math/big"

	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type tracerWrapper struct {
	vm.EVMLogger
	depth int
}

// NewTracerWrapper wraps the EVMLogger
func NewTracerWrapper(tracer vm.EVMLogger) vm.EVMLogger {
	return &tracerWrapper{EVMLogger: tracer}
}

func (tw *tracerWrapper) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	tw.depth++
	if tw.depth > 1 {
		op := vm.CALL
		if create {
			op = vm.CREATE
		}
		tw.EVMLogger.CaptureEnter(op, from, to, input, gas, value)
		return
	}
	tw.EVMLogger.CaptureStart(env, from, to, create, input, gas, value)
}

func (tw *tracerWrapper) CaptureEnd(output []byte, gasUsed uint64, err error) {
	if tw.depth < 1 {
		return
	}
	defer func() { tw.depth-- }()
	if tw.depth > 1 {
		tw.EVMLogger.CaptureExit(output, gasUsed, err)
		return
	}
	tw.EVMLogger.CaptureEnd(output, gasUsed, err)
}

func (tw *tracerWrapper) Unwrap() vm.EVMLogger {
	return tw.EVMLogger
}

// TraceStart starts tracing the execution of the action in the sealed envelope
func TraceStart(ctx context.Context, ws protocol.StateManager, elp action.Envelope) error {
	vmCtx, vmCtxExist := protocol.GetVMConfigCtx(ctx)
	if !vmCtxExist || vmCtx.Tracer == nil {
		return nil
	}
	evm, err := newEVM(ctx, ws, elp)
	if err != nil {
		return errors.Wrap(err, "failed to create EVM instance for tracing")
	}
	var (
		to    *common.Address
		value = big.NewInt(0)
		input = elp.Data()
	)
	switch a := elp.Action().(type) {
	case action.EthCompatibleAction:
		to, err = a.EthTo()
		if err != nil {
			return errors.Wrap(err, "failed to get eth compatible action to address")
		}
		if elp.Value() != nil {
			value = elp.Value()
		}
		input, err = a.EthData()
		if err != nil {
			return errors.Wrap(err, "failed to get eth compatible action data")
		}
	default:
		return errors.New("only eth compatible action is supported for tracing")
	}
	vmCtx.Tracer.CaptureTxStart(elp.Gas())
	if _, isExecution := elp.Action().(*action.Execution); isExecution {
		// CaptureStart will be called in evm
		return nil
	}
	actCtx := protocol.MustGetActionCtx(ctx)
	vmCtx.Tracer.CaptureStart(evm, common.Address(actCtx.Caller.Bytes()), *to, false, input, elp.Gas(), value)
	return nil
}

// TraceEnd ends tracing the execution of the action in the sealed envelope
func TraceEnd(ctx context.Context, ws protocol.StateManager, elp action.Envelope, receipt *action.Receipt) {
	vmCtx, vmCtxExist := protocol.GetVMConfigCtx(ctx)
	if !vmCtxExist || vmCtx.Tracer == nil || receipt == nil {
		return
	}
	output := receipt.Output
	vmCtx.Tracer.CaptureEnd(output, receipt.GasConsumed, nil)
	vmCtx.Tracer.CaptureTxEnd(elp.Gas() - receipt.GasConsumed)
	if t, ok := GetTracerCtx(ctx); ok {
		t.CaptureTx(output, receipt)
	}
}

func newEVM(ctx context.Context, sm protocol.StateManager, execution action.TxData) (*vm.EVM, error) {
	var stateDB stateDB
	stateDB, err := prepareStateDB(ctx, sm)
	if err != nil {
		return nil, err
	}
	if erigonsm, ok := sm.(interface {
		Erigon() (*erigonstate.IntraBlockState, bool)
	}); ok {
		if in, dryrun := erigonsm.Erigon(); in != nil {
			if !dryrun {
				log.S().Panic("should not happen, use dryrun instead")
			}
			stateDB = NewErigonStateDBAdapterDryrun(stateDB.(*StateDBAdapter), in)
		}
	}
	evmParams, err := newParams(ctx, execution)
	if err != nil {
		return nil, err
	}
	evm := vm.NewEVM(evmParams.context, evmParams.txCtx, stateDB, evmParams.chainConfig, evmParams.evmConfig)
	return evm, nil
}
