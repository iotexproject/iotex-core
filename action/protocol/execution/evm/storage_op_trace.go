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
	// StorageOpSnapshot is a Snapshot call.
	StorageOpSnapshot
	// StorageOpRevertToSnapshot is a RevertToSnapshot call.
	StorageOpRevertToSnapshot
	// StorageOpGetStateFailed is a GetState call that returned an error.
	StorageOpGetStateFailed
	// StorageOpGetCommittedStateFailed is a GetCommittedState call that returned an error.
	StorageOpGetCommittedStateFailed
)

func (t StorageOpType) String() string {
	switch t {
	case StorageOpGetState:
		return "GetState"
	case StorageOpSetState:
		return "SetState"
	case StorageOpGetCommittedState:
		return "GetCommittedState"
	case StorageOpSnapshot:
		return "Snapshot"
	case StorageOpRevertToSnapshot:
		return "RevertToSnapshot"
	case StorageOpGetStateFailed:
		return "GetStateFailed"
	case StorageOpGetCommittedStateFailed:
		return "GetCommittedStateFailed"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

// StorageOp records one GetState/SetState/GetCommittedState/Snapshot/RevertToSnapshot call.
type StorageOp struct {
	OpType     StorageOpType
	Addr       common.Address
	Key        common.Hash
	Value      common.Hash // for SetState: value being set; for Get*: value returned
	SnapshotID int         // for Snapshot/RevertToSnapshot: the snapshot ID
	ErrMsg     string      // for failed ops: the error message
}

func (op *StorageOp) String() string {
	if op.OpType == StorageOpSnapshot || op.OpType == StorageOpRevertToSnapshot {
		return fmt.Sprintf("%s snapshotID=%d", op.OpType, op.SnapshotID)
	}
	if op.OpType == StorageOpGetStateFailed || op.OpType == StorageOpGetCommittedStateFailed {
		return fmt.Sprintf("%s addr=%s key=%s err=%s",
			op.OpType, op.Addr.Hex(), op.Key.Hex(), op.ErrMsg)
	}
	return fmt.Sprintf("%s addr=%s key=%s val=%s",
		op.OpType, op.Addr.Hex(), op.Key.Hex(), op.Value.Hex())
}

// StorageOpTraceJSON is the JSON-serializable form of StorageOp.
type StorageOpTraceJSON struct {
	Op         string `json:"op"`
	Addr       string `json:"addr,omitempty"`
	Key        string `json:"key,omitempty"`
	Value      string `json:"value,omitempty"`
	SnapshotID int    `json:"snapshotID,omitempty"`
	ErrMsg     string `json:"errMsg,omitempty"`
}

// StorageOpsToJSON converts a slice of StorageOp to JSON-serializable form.
func StorageOpsToJSON(ops []StorageOp) []StorageOpTraceJSON {
	if len(ops) == 0 {
		return nil
	}
	out := make([]StorageOpTraceJSON, len(ops))
	for i, op := range ops {
		out[i] = StorageOpTraceJSON{
			Op:         op.OpType.String(),
			Addr:       op.Addr.Hex(),
			Key:        "0x" + hex.EncodeToString(op.Key[:]),
			Value:      "0x" + hex.EncodeToString(op.Value[:]),
			SnapshotID: op.SnapshotID,
			ErrMsg:     op.ErrMsg,
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
		case "Snapshot":
			out[i].OpType = StorageOpSnapshot
		case "RevertToSnapshot":
			out[i].OpType = StorageOpRevertToSnapshot
		case "GetStateFailed":
			out[i].OpType = StorageOpGetStateFailed
		case "GetCommittedStateFailed":
			out[i].OpType = StorageOpGetCommittedStateFailed
		}
		out[i].Addr = common.HexToAddress(op.Addr)
		out[i].Key = common.HexToHash(op.Key)
		out[i].Value = common.HexToHash(op.Value)
		out[i].SnapshotID = op.SnapshotID
		out[i].ErrMsg = op.ErrMsg
	}
	return out
}
