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
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestTraceBlockByNumber(t *testing.T) {
	r := require.New(t)
	sender := identityset.Address(10).String()
	senderSK := identityset.PrivateKey(10)
	receiver := identityset.Address(11).String()

	cfg := initCfg(r)
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

	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	ctx := context.Background()

	rpcCall := func(height uint64) (json.RawMessage, error) {
		body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"debug_traceBlockByNumber","params":["0x%x",{"tracer":"callTracer"}],"id":1}`, height)
		url := fmt.Sprintf("http://localhost:%d", cfg.API.HTTPPort)
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

	// deploy a simple contract so the traced block has EVM execution
	var contractAddr string
	var deployBlkHeight uint64
	test.runCase(ctx, &testcase{
		name: "deploy contract",
		act: &actionWithTime{
			mustNoErr(action.Sign(
				action.NewEnvelope(
					action.NewLegacyTx(chainID, test.nonceMgr.pop(sender), gasLimit, gasPrice),
					// minimal bytecode: PUSH1 0x00 PUSH1 0x00 RETURN (6000600000) — deploys empty contract
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
	result, err := rpcCall(deployBlkHeight)
	r.NoError(err, "debug_traceBlockByNumber should succeed for historical block with EVM tx")
	t.Logf("trace result: %s", string(result))
	r.NotNil(result)

	// also verify the result array contains one entry per tx
	var entries []json.RawMessage
	r.NoError(json.Unmarshal(result, &entries))
	r.Len(entries, 1, "should have one trace entry for the single tx in the block")
}
