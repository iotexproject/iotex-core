package apitypes

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
)

// StructLog represents a structured log created during the execution of the EVM.
type StructLog struct {
	Pc            uint64                      `json:"pc"`
	Op            StructLogVmOpCode           `json:"op"`
	Gas           math.HexOrDecimal64         `json:"gas"`
	GasCost       math.HexOrDecimal64         `json:"gasCost"`
	Memory        StructLogMemory             `json:"memory"`
	MemorySize    int                         `json:"memSize"`
	Stack         []uint256.Int               `json:"stack"`
	ReturnData    hexutil.Bytes               `json:"returnData"`
	Storage       map[common.Hash]common.Hash `json:"storage"`
	Depth         int                         `json:"depth"`
	RefundCounter uint64                      `json:"refund"`
	OpName        string                      `json:"opName"`
	ErrorString   string                      `json:"error"`
}

type StructLogMemory hexutil.Bytes

func (m *StructLogMemory) UnmarshalJSON(input []byte) error {
	var memoryWords []string
	if err := json.Unmarshal(input, &memoryWords); err != nil {
		return err
	}
	// reconstruct the full memory from memory words
	fullMemory := make([]byte, 0, len(memoryWords)*32)
	for _, word := range memoryWords {
		fullMemory = append(fullMemory, common.FromHex(word)...)
	}
	*m = StructLogMemory(fullMemory)
	return nil
}

type StructLogVmOpCode vm.OpCode

func (op *StructLogVmOpCode) UnmarshalJSON(input []byte) error {
	var opStr string
	if err := json.Unmarshal(input, &opStr); err != nil {
		return err
	}
	*op = StructLogVmOpCode(vm.StringToOp(opStr))
	return nil
}
