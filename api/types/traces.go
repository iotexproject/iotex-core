package apitypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/holiman/uint256"
)

// TransactionArgs represents the arguments to construct a new transaction
// or a message call.
type TransactionArgs struct {
	From     string `json:"from"`     //(optional) The address the transaction is sent from
	To       string `json:"to"`       //The address the transaction is directed to
	Gas      uint64 `json:"gas"`      //(optional) The integer of the gas provided for the transaction execution
	GasPrice int64  `json:"gasPrice"` //(optional) The integer of the gasPrice used for each paid gas
	Value    int64  `json:"value"`    //(optional) The integer of the value sent with this transaction
	Data     string `json:"data"`     //(optional) The hash of the method signature and encoded parameters
}

// BlockNumberOrHash represents a block number or a block hash
type BlockNumberOrHash struct {
	BlockNumber uint64 `json:"blockNumber,omitempty"`
	BlockHash   string `json:"blockHash,omitempty"`
}

// StructLog represents a structured log created during the execution of the EVM.
type StructLog struct {
	Pc            uint64                      `json:"pc"`
	Op            vm.OpCode                   `json:"op"`
	Gas           math.HexOrDecimal64         `json:"gas"`
	GasCost       math.HexOrDecimal64         `json:"gasCost"`
	Memory        hexutil.Bytes               `json:"memory"`
	MemorySize    int                         `json:"memSize"`
	Stack         []uint256.Int               `json:"stack"`
	ReturnData    hexutil.Bytes               `json:"returnData"`
	Storage       map[common.Hash]common.Hash `json:"storage"`
	Depth         int                         `json:"depth"`
	RefundCounter uint64                      `json:"refund"`
	OpName        string                      `json:"opName"`
	ErrorString   string                      `json:"error"`
}

// FromLoggerStructLogs converts logger.StructLog to apitypes.StructLog
func FromLoggerStructLogs(logs []logger.StructLog) []StructLog {
	ret := make([]StructLog, len(logs))
	for index, log := range logs {
		ret[index] = StructLog{
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
