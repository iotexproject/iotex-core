package evm

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// StorageOpType indicates the type of storage operation.
type StorageOpType uint8

const (
	// StorageOpGetState is a GetState call.
	StorageOpGetState StorageOpType = iota
	// StorageOpSetState is a SetState call.
	StorageOpSetState
	// StorageOpGetCommittedState is a GetCommittedState call.
	StorageOpGetCommittedState
)

func (t StorageOpType) String() string {
	switch t {
	case StorageOpGetState:
		return "GetState"
	case StorageOpSetState:
		return "SetState"
	case StorageOpGetCommittedState:
		return "GetCommittedState"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

// StorageOp records one GetState/SetState/GetCommittedState call.
type StorageOp struct {
	OpType StorageOpType
	Addr   common.Address
	Key    common.Hash
	Value  common.Hash // for SetState: value being set; for Get*: value returned
}

func (op *StorageOp) String() string {
	return fmt.Sprintf("%s addr=%s key=%s val=%s",
		op.OpType, op.Addr.Hex(), op.Key.Hex(), op.Value.Hex())
}

// StorageOpTraceJSON is the JSON-serializable form of StorageOp.
type StorageOpTraceJSON struct {
	Op    string `json:"op"`
	Addr  string `json:"addr"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// StorageOpsToJSON converts a slice of StorageOp to JSON-serializable form.
func StorageOpsToJSON(ops []StorageOp) []StorageOpTraceJSON {
	if len(ops) == 0 {
		return nil
	}
	out := make([]StorageOpTraceJSON, len(ops))
	for i, op := range ops {
		out[i] = StorageOpTraceJSON{
			Op:    op.OpType.String(),
			Addr:  op.Addr.Hex(),
			Key:   "0x" + hex.EncodeToString(op.Key[:]),
			Value: "0x" + hex.EncodeToString(op.Value[:]),
		}
	}
	return out
}

// StorageOpsFromJSON converts JSON-serializable form back to StorageOp slice.
func StorageOpsFromJSON(ops []StorageOpTraceJSON) []StorageOp {
	if len(ops) == 0 {
		return nil
	}
	out := make([]StorageOp, len(ops))
	for i, op := range ops {
		switch op.Op {
		case "GetState":
			out[i].OpType = StorageOpGetState
		case "SetState":
			out[i].OpType = StorageOpSetState
		case "GetCommittedState":
			out[i].OpType = StorageOpGetCommittedState
		}
		out[i].Addr = common.HexToAddress(op.Addr)
		out[i].Key = common.HexToHash(op.Key)
		out[i].Value = common.HexToHash(op.Value)
	}
	return out
}
