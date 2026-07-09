package e2etest

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-resty/resty/v2"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func traceBlockSetup(t *testing.T) (cfg config.Config, sender string, senderSK crypto.PrivateKey, receiver string) {
	r := require.New(t)
	sender = identityset.Address(10).String()
	senderSK = identityset.PrivateKey(10)
	receiver = identityset.Address(11).String()

	cfg = initCfg(r)
	historyIndexPath, err := os.MkdirTemp("", "historyindex")
	r.NoError(err)
	cfg.Chain.HistoryIndexPath = historyIndexPath
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(1000000).String()
	cfg.Genesis.YapBetaBlockHeight = 1
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	return
}

func rpcTraceBlock(t *testing.T, port int, height uint64, tracerName string) (json.RawMessage, error) {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"debug_traceBlockByNumber","params":["0x%x",{"tracer":%q}],"id":1}`, height, tracerName)
	url := fmt.Sprintf("http://localhost:%d", port)
	type web3Response struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	result := &web3Response{}
	resp, err := resty.New().R().SetBody(body).Post(url)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(resp.Body(), result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", result.Error.Code, result.Error.Message)
	}
	return result.Result, nil
}

func TestTraceBlockByNumber(t *testing.T) {
	r := require.New(t)
	cfg, sender, senderSK, receiver := traceBlockSetup(t)

	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	ctx := context.Background()

	// deploy a simple contract so the traced block has EVM execution
	var contractAddr string
	var deployBlkHeight uint64
	test.runCase(ctx, &testcase{
		name: "deploy contract",
		act: &actionWithTime{
			mustNoErr(action.Sign(
				action.NewEnvelope(
					action.NewLegacyTx(chainID, test.nonceMgr.pop(sender), gasLimit, gasPrice),
					// minimal bytecode: PUSH1 0x00 PUSH1 0x00 RETURN — deploys empty contract
					action.NewExecution("", big.NewInt(0), []byte{0x60, 0x00, 0x60, 0x00, 0xf3}),
				),
				senderSK,
			)),
			time.Now(),
		},
		blockExpect: func(test *e2etest, blk *block.Block, err error) {
			r.NoError(err)
			r.EqualValues(1, blk.Receipts[0].Status)
			contractAddr = blk.Receipts[0].ContractAddress
			deployBlkHeight = blk.Height()
			t.Log("contract deployed at block height:", deployBlkHeight, "addr:", contractAddr)
		},
	})

	// produce one more block so deployBlkHeight is historical
	test.runCase(ctx, &testcase{
		name: "another tx",
		act: &actionWithTime{
			mustNoErr(action.Sign(
				action.NewEnvelope(
					action.NewLegacyTx(chainID, test.nonceMgr.pop(sender), gasLimit, gasPrice),
					action.NewTransfer(big.NewInt(1), receiver, nil),
				),
				senderSK,
			)),
			time.Now(),
		},
	})

	// trace the block containing the contract deployment
	t.Logf("calling debug_traceBlockByNumber for height %d", deployBlkHeight)
	result, err := rpcTraceBlock(t, cfg.API.HTTPPort, deployBlkHeight, "callTracer")
	r.NoError(err, "callTracer should succeed for historical block with EVM tx")
	t.Logf("callTracer result: %s", string(result))
	r.NotNil(result)

	var entries []json.RawMessage
	r.NoError(json.Unmarshal(result, &entries))
	r.Len(entries, 1, "should have one trace entry for the single tx in the block")

	_ = contractAddr
}

func TestTraceBlockByNumberPrestateTracer(t *testing.T) {
	r := require.New(t)
	cfg, sender, senderSK, receiver := traceBlockSetup(t)

	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	ctx := context.Background()

	// deploy a simple contract so the traced block has EVM execution
	var deployBlkHeight uint64
	test.runCase(ctx, &testcase{
		name: "deploy contract",
		act: &actionWithTime{
			mustNoErr(action.Sign(
				action.NewEnvelope(
					action.NewLegacyTx(chainID, test.nonceMgr.pop(sender), gasLimit, gasPrice),
					// minimal bytecode: PUSH1 0x00 PUSH1 0x00 RETURN — deploys empty contract
					action.NewExecution("", big.NewInt(0), []byte{0x60, 0x00, 0x60, 0x00, 0xf3}),
				),
				senderSK,
			)),
			time.Now(),
		},
		blockExpect: func(test *e2etest, blk *block.Block, err error) {
			r.NoError(err)
			r.EqualValues(1, blk.Receipts[0].Status)
			deployBlkHeight = blk.Height()
			t.Log("contract deployed at block height:", deployBlkHeight)
		},
	})

	// produce one more block so deployBlkHeight is historical
	test.runCase(ctx, &testcase{
		name: "another tx",
		act: &actionWithTime{
			mustNoErr(action.Sign(
				action.NewEnvelope(
					action.NewLegacyTx(chainID, test.nonceMgr.pop(sender), gasLimit, gasPrice),
					action.NewTransfer(big.NewInt(1), receiver, nil),
				),
				senderSK,
			)),
			time.Now(),
		},
	})

	// trace with prestateTracer
	t.Logf("calling debug_traceBlockByNumber with prestateTracer for height %d", deployBlkHeight)
	result, err := rpcTraceBlock(t, cfg.API.HTTPPort, deployBlkHeight, "prestateTracer")
	r.NoError(err, "prestateTracer should succeed for historical block with EVM tx")
	t.Logf("prestateTracer result: %s", string(result))
	r.NotNil(result)

	var entries []json.RawMessage
	r.NoError(json.Unmarshal(result, &entries))
	r.Len(entries, 1, "should have one trace entry for the single tx in the block")
}

// TestTraceBlockPerTxIsolation traces a block containing two independent
// transactions (two distinct senders, two distinct receivers) and asserts that
// each transaction's trace is isolated from the others. This is the regression
// test for issue #4825: a single tracer shared across all transactions in the
// block causes prestate to accumulate and the callTracer callstack to never
// reset, silently dropping the second tx's result.
func TestTraceBlockPerTxIsolation(t *testing.T) {
	r := require.New(t)
	cfg, senderA, senderASK, receiverA := traceBlockSetup(t)
	// fund a second, independent sender so the two txs touch disjoint accounts
	senderB := identityset.Address(12).String()
	senderBSK := identityset.PrivateKey(12)
	receiverB := identityset.Address(13).String()
	cfg.Genesis.InitBalanceMap[senderB] = unit.ConvertIotxToRau(1000000).String()

	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	ctx := context.Background()

	// eth-hex forms of the two senders (prestateTracer keys accounts by address)
	senderAHex := strings.ToLower(common.BytesToAddress(identityset.Address(10).Bytes()).Hex())
	senderBHex := strings.ToLower(common.BytesToAddress(identityset.Address(12).Bytes()).Hex())

	// mine one block containing two independent transfers
	var traceBlkHeight uint64
	test.runCase(ctx, &testcase{
		name: "two independent transfers",
		acts: []*actionWithTime{
			{
				mustNoErr(action.Sign(
					action.NewEnvelope(
						action.NewLegacyTx(chainID, test.nonceMgr.pop(senderA), gasLimit, gasPrice),
						action.NewTransfer(big.NewInt(1), receiverA, nil),
					),
					senderASK,
				)),
				time.Now(),
			},
			{
				mustNoErr(action.Sign(
					action.NewEnvelope(
						action.NewLegacyTx(chainID, test.nonceMgr.pop(senderB), gasLimit, gasPrice),
						action.NewTransfer(big.NewInt(1), receiverB, nil),
					),
					senderBSK,
				)),
				time.Now(),
			},
		},
		blockExpect: func(test *e2etest, blk *block.Block, err error) {
			r.NoError(err)
			// block also carries a system GrantReward action besides the 2 user txs
			r.GreaterOrEqual(len(blk.Actions), 2, "block should contain the two user txs")
			traceBlkHeight = blk.Height()
			t.Log("two-tx block at height:", traceBlkHeight)
		},
	})

	// produce one more block so traceBlkHeight is historical
	test.runCase(ctx, &testcase{
		name: "another tx",
		act: &actionWithTime{
			mustNoErr(action.Sign(
				action.NewEnvelope(
					action.NewLegacyTx(chainID, test.nonceMgr.pop(senderA), gasLimit, gasPrice),
					action.NewTransfer(big.NewInt(1), receiverA, nil),
				),
				senderASK,
			)),
			time.Now(),
		},
	})

	// callTracer: both txs must yield a valid top-level call result; none dropped.
	callResult, err := rpcTraceBlock(t, cfg.API.HTTPPort, traceBlkHeight, "callTracer")
	r.NoError(err, "callTracer should succeed")
	t.Logf("callTracer result: %s", string(callResult))
	var callEntries []struct {
		TxHash string          `json:"txHash"`
		Result json.RawMessage `json:"result"`
	}
	r.NoError(json.Unmarshal(callResult, &callEntries))
	r.Len(callEntries, 2, "callTracer must return one result per tx; none may be dropped")
	seenTx := map[string]bool{}
	for i, e := range callEntries {
		r.NotEmpty(e.TxHash, "entry %d must carry a tx hash", i)
		r.False(seenTx[e.TxHash], "entry %d duplicates tx hash %s", i, e.TxHash)
		seenTx[e.TxHash] = true
		var frame struct {
			Type  string `json:"type"`
			From  string `json:"from"`
			Error string `json:"error"`
		}
		r.NoError(json.Unmarshal(e.Result, &frame), "entry %d must be a valid call frame", i)
		r.NotEmpty(frame.Type, "entry %d must have a call type", i)
		r.NotEmpty(frame.From, "entry %d must have a from address", i)
	}

	// prestateTracer: each tx's prestate must contain only its own touched
	// accounts. No single entry may contain both senders' accounts.
	preResult, err := rpcTraceBlock(t, cfg.API.HTTPPort, traceBlkHeight, "prestateTracer")
	r.NoError(err, "prestateTracer should succeed")
	t.Logf("prestateTracer result: %s", string(preResult))
	var preEntries []struct {
		TxHash string          `json:"txHash"`
		Result json.RawMessage `json:"result"`
	}
	r.NoError(json.Unmarshal(preResult, &preEntries))
	r.Len(preEntries, 2, "prestateTracer must return one result per tx")
	sawA, sawB := false, false
	for i, e := range preEntries {
		var pre map[string]json.RawMessage
		r.NoError(json.Unmarshal(e.Result, &pre))
		hasA, hasB := false, false
		for addr := range pre {
			switch strings.ToLower(addr) {
			case senderAHex:
				hasA = true
			case senderBHex:
				hasB = true
			}
		}
		r.Falsef(hasA && hasB, "entry %d leaked both senders into one prestate (accumulation bug)", i)
		sawA = sawA || hasA
		sawB = sawB || hasB
	}
	r.True(sawA, "senderA must appear in exactly one tx's prestate")
	r.True(sawB, "senderB must appear in exactly one tx's prestate")
}
