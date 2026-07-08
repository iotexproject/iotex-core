package e2etest

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

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
