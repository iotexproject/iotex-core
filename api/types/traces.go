package apitypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
)

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

// DebugTxTraceResult is the result of a single transaction trace.
type DebugTxTraceResult struct {
	Failed      bool        `json:"failed"`
	Revert      string      `json:"revert"`
	ReturnValue string      `json:"returnValue"`
	Gas         uint64      `json:"gas"`
	StructLogs  []StructLog `json:"structLogs"`
}

// TxTraceResult is the result of a single transaction trace.
type TxTraceResult struct {
	Result interface{} `json:"result,omitempty"` // Trace results produced by the tracer
	Error  string      `json:"error,omitempty"`  // Trace failure produced by the tracer
}
