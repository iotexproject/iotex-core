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
