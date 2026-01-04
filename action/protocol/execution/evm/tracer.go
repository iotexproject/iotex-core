package evm

import (
	"context"
	"math/big"

	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

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
	actCtx := protocol.MustGetActionCtx(ctx)
	var (
		from  = common.Address(actCtx.Caller.Bytes())
		to    *common.Address
		value = big.NewInt(0)
		input = elp.Data()
		ethTx *types.Transaction
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
		ethTx, err = elp.ToEthTx()
		if err != nil {
			return errors.Wrap(err, "failed to convert to eth tx")
		}
	default:
		return errors.New("only eth compatible action is supported for tracing")
	}
	vmCtx.Tracer.OnTxStart(evm.GetVMContext(), ethTx, from)
	if _, isExecution := elp.Action().(*action.Execution); isExecution {
		// CaptureStart will be called in evm
		return nil
	}

	vmCtx.Tracer.OnEnter(0, byte(vm.CALL), from, *to, input, elp.Gas(), value)
	return nil
}

// TraceEnd ends tracing the execution of the action in the sealed envelope
func TraceEnd(ctx context.Context, ws protocol.StateManager, elp action.Envelope, receipt *action.Receipt) {
	vmCtx, vmCtxExist := protocol.GetVMConfigCtx(ctx)
	if !vmCtxExist || vmCtx.Tracer == nil || receipt == nil {
		return
	}
	output := receipt.Output
	vmCtx.Tracer.OnExit(0, output, receipt.GasConsumed, nil, receipt.Status != uint64(iotextypes.ReceiptStatus_Success))
	ethReceipt := toEthReceipt(receipt)
	vmCtx.Tracer.OnTxEnd(ethReceipt, nil)
	if t, ok := GetTracerCtx(ctx); ok {
		t.CaptureTx(output, receipt)
	}
}

func toEthReceipt(receipt *action.Receipt) *types.Receipt {
	if receipt == nil {
		return nil
	}
	ethReceipt := &types.Receipt{
		Status:            uint64(receipt.Status),
		CumulativeGasUsed: receipt.GasConsumed,
		Bloom:             types.Bloom{},
		Logs:              []*types.Log{},
		TxHash:            common.Hash{},
		ContractAddress:   common.Address{},
		GasUsed:           receipt.GasConsumed,
	}
	for _, lg := range receipt.Logs() {
		addr, _ := address.FromString(lg.Address)
		ethLog := &types.Log{
			Address:     common.Address(addr.Bytes()),
			Topics:      make([]common.Hash, len(lg.Topics)),
			Data:        lg.Data,
			BlockNumber: lg.BlockHeight,
			TxHash:      common.Hash(lg.ActionHash[:]),
			TxIndex:     uint(lg.TxIndex),
			Index:       uint(lg.Index),
			Removed:     false,
		}
		for i, topic := range lg.Topics {
			ethLog.Topics[i] = common.Hash(topic[:])
		}
		ethReceipt.Logs = append(ethReceipt.Logs, ethLog)
	}
	return ethReceipt
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
	evm := vm.NewEVM(evmParams.context, stateDB, evmParams.chainConfig, evmParams.evmConfig)
	evm.SetTxContext(evmParams.txCtx)
	return evm, nil
}
