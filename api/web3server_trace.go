package api

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/tidwall/gjson"
)

// fromLoggerStructLogs converts logger.StructLog to apitypes.StructLog
func fromLoggerStructLogs(logs []logger.StructLog) []apitypes.StructLog {
	ret := make([]apitypes.StructLog, len(logs))
	for index, log := range logs {
		ret[index] = apitypes.StructLog{
			Pc:            log.Pc,
			Op:            log.Op,
			Gas:           math.HexOrDecimal64(log.Gas),
			GasCost:       math.HexOrDecimal64(log.GasCost),
			Memory:        log.Memory,
			MemorySize:    log.MemorySize,
			Stack:         log.Stack,
			ReturnData:    log.ReturnData,
			Storage:       log.Storage,
			Depth:         log.Depth,
			RefundCounter: log.RefundCounter,
			OpName:        log.OpName(),
			ErrorString:   log.ErrorString(),
		}
	}
	return ret
}

func (svr *web3Handler) traceCall(ctx context.Context, in *gjson.Result) (interface{}, error) {
	var callArgs apitypes.TransactionArgs
	var blkHash hash.Hash256
	var err error
	var contractAddr string
	var addrFrom string
	var callData []byte
	var gasLimit uint64
	core := svr.coreService.(*coreService)
	callArgsObj, blkNumOrHashObj, options := in.Get("params.0"), in.Get("params.1"), in.Get("params.2")
	addrFrom = address.ZeroAddress
	gasLimit = core.bc.Genesis().BlockGasLimit
	if callArgsObj.Exists() {
		if err := json.Unmarshal([]byte(callArgsObj.Raw), &callArgs); err != nil {
			return nil, err
		}
		addr, err := address.FromHex(callArgs.To)
		if err != nil {
			return nil, err
		}
		contractAddr = addr.String()
		if callArgs.From != "" {
			addr, err := address.FromHex(callArgs.From)
			if err != nil {
				return nil, err
			}
			addrFrom = addr.String()
		}
		if callArgs.Data != "" {
			callData, err = hexToBytes(callArgs.Data)
			if err != nil {
				return nil, err
			}
		}
		if callArgs.Gas != 0 {
			gasLimit = callArgs.Gas
		}
	}
	var blkNumOrHash apitypes.BlockNumberOrHash
	if blkNumOrHashObj.Exists() {
		if err := json.Unmarshal([]byte(blkNumOrHashObj.Raw), &blkNumOrHash); err != nil {
			return nil, err
		}
		if blkNumOrHash.BlockHash != "" {
			blkHash, err = hash.HexStringToHash256(blkNumOrHash.BlockHash)
		} else {
			blkHash, err = core.dao.GetBlockHash(blkNumOrHash.BlockNumber)
		}
		if err != nil {
			return nil, err
		}
	}
	getblockHash := func(height uint64) (hash.Hash256, error) {
		return blkHash, nil
	}

	var (
		enableMemory, disableStack, disableStorage, enableReturnData bool
	)
	if options.Exists() {
		enableMemory = options.Get("enableMemory").Bool()
		disableStack = options.Get("disableStack").Bool()
		disableStorage = options.Get("disableStorage").Bool()
		enableReturnData = options.Get("enableReturnData").Bool()
	}
	cfg := &logger.Config{
		EnableMemory:     enableMemory,
		DisableStack:     disableStack,
		DisableStorage:   disableStorage,
		EnableReturnData: enableReturnData,
	}

	exec, err := action.NewExecution(
		contractAddr,
		uint64(0),
		big.NewInt(callArgs.Value),
		gasLimit,
		big.NewInt(callArgs.GasPrice),
		callData,
	)
	if err != nil {
		return nil, err
	}
	addr, _ := address.FromString(addrFrom)

	ctx = genesis.WithGenesisContext(ctx, core.bc.Genesis())
	state, err := accountutil.AccountState(ctx, core.sf, addr)
	if err != nil {
		return nil, err
	}
	ctx, err = core.bc.Context(ctx)
	if err != nil {
		return nil, err
	}
	exec.SetNonce(state.PendingNonce())

	traces := logger.NewStructLogger(cfg)
	ctx = protocol.WithVMConfigCtx(ctx, vm.Config{
		Debug:     true,
		Tracer:    traces,
		NoBaseFee: true,
	})
	retval, receipt, err := core.sf.SimulateExecution(ctx, addr, exec, getblockHash)
	if err != nil {
		return nil, err
	}

	return &debugTraceTransactionResult{
		Failed:      receipt.Status != uint64(iotextypes.ReceiptStatus_Success),
		Revert:      receipt.ExecutionRevertMsg(),
		ReturnValue: byteToHex(retval),
		StructLogs:  fromLoggerStructLogs(traces.StructLogs()),
		Gas:         receipt.GasConsumed,
	}, nil
}
