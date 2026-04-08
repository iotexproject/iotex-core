package witness

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
)

func TestCollectorCapturesStorageOps(t *testing.T) {
	r := require.New(t)
	c := NewCollector()

	ops := []evm.StorageOp{
		{OpType: evm.StorageOpGetState, Addr: common.HexToAddress("0x1"), Key: common.HexToHash("0xaa"), Value: common.HexToHash("0xbb")},
		{OpType: evm.StorageOpSetState, Addr: common.HexToAddress("0x1"), Key: common.HexToHash("0xcc"), Value: common.HexToHash("0xdd")},
	}
	c.CaptureStorageOps(ops)

	actionHash := hash.Hash256{1, 2, 3}
	c.CaptureTx(nil, &action.Receipt{ActionHash: actionHash})

	r.Len(c.actionStorageOps[actionHash], 2)
	r.Equal(evm.StorageOpGetState, c.actionStorageOps[actionHash][0].OpType)
	r.Equal(evm.StorageOpSetState, c.actionStorageOps[actionHash][1].OpType)
	// currentStorageOps should be cleared after CaptureTx
	r.Nil(c.currentStorageOps)
}

func TestCollectorCapturesWriteEntries(t *testing.T) {
	r := require.New(t)
	c := NewCollector()

	entries := []string{
		"[0] PUT ns=evm key=abcd vlen=32 vhash=1234",
		"[1] DEL ns=evm key=ef01 vlen=0 vhash=0000",
	}
	c.CaptureWriteEntries(entries)

	r.Equal(entries, c.debugWriteEntries)
}

func TestDebugInfoJSONRoundTrip(t *testing.T) {
	r := require.New(t)

	original := &BlockResult{
		Transactions: []TransactionResult{
			{
				TxHash: "0xaabb",
				DebugStorageOps: []evm.StorageOpTraceJSON{
					{Op: "GetState", Addr: "0x0001", Key: "0xaa", Value: "0xbb"},
					{Op: "SetState", Addr: "0x0001", Key: "0xcc", Value: "0xdd"},
				},
			},
			{
				TxHash: "0xccdd",
			},
		},
		DebugWriteEntries: []string{
			"[0] PUT ns=evm key=abcd vlen=32 vhash=1234",
			"[1] DEL ns=evm key=ef01 vlen=0 vhash=0000",
		},
	}

	data, err := json.Marshal(original)
	r.NoError(err)

	var decoded BlockResult
	r.NoError(json.Unmarshal(data, &decoded))

	r.Equal(original.DebugWriteEntries, decoded.DebugWriteEntries)
	r.Len(decoded.Transactions, 2)
	r.Len(decoded.Transactions[0].DebugStorageOps, 2)
	r.Equal("GetState", decoded.Transactions[0].DebugStorageOps[0].Op)
	r.Equal("SetState", decoded.Transactions[0].DebugStorageOps[1].Op)
	r.Nil(decoded.Transactions[1].DebugStorageOps)
}

func TestParseValidationContextWithDebugInfo(t *testing.T) {
	r := require.New(t)

	result := &BlockResult{
		Transactions: []TransactionResult{
			{
				TxHash: "0x0102030000000000000000000000000000000000000000000000000000000000",
				DebugStorageOps: []evm.StorageOpTraceJSON{
					{Op: "GetState", Addr: "0x0001", Key: "0xaa", Value: "0xbb"},
				},
			},
		},
		DebugWriteEntries: []string{"[0] PUT ns=evm key=abcd vlen=32 vhash=1234"},
	}

	raw, err := json.Marshal(result)
	r.NoError(err)

	ctx, err := ParseValidationContext(raw)
	r.NoError(err)
	r.True(ctx.Enabled)

	// verify debug write entries
	r.Equal([]string{"[0] PUT ns=evm key=abcd vlen=32 vhash=1234"}, ctx.DebugWriteEntries)

	// verify debug storage ops
	r.Len(ctx.DebugStorageOps, 1)
	for _, ops := range ctx.DebugStorageOps {
		r.Len(ops, 1)
		r.Equal("GetState", ops[0].Op)
		r.Equal("0x0001", ops[0].Addr)
	}
}

func TestDebugInfoOmittedWhenEmpty(t *testing.T) {
	r := require.New(t)

	result := &BlockResult{
		Transactions: []TransactionResult{
			{TxHash: "0xaabb"},
		},
	}

	data, err := json.Marshal(result)
	r.NoError(err)

	// verify omitempty: no debugWriteEntries or debugStorageOps in JSON
	var raw map[string]json.RawMessage
	r.NoError(json.Unmarshal(data, &raw))
	_, hasWriteEntries := raw["debugWriteEntries"]
	r.False(hasWriteEntries, "debugWriteEntries should be omitted when empty")

	var txs []map[string]json.RawMessage
	r.NoError(json.Unmarshal(raw["transactions"], &txs))
	_, hasOps := txs[0]["debugStorageOps"]
	r.False(hasOps, "debugStorageOps should be omitted when empty")
}
