package e2etest

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// gasBurner is contract-creation init code that loops until it runs out of gas:
// JUMPDEST PUSH1 0 POP PUSH1 0 JUMP. Every iteration is 5 opcodes, so the
// struct-log output of a trace grows with the gas it is given.
var gasBurner = []byte{0x5b, 0x60, 0x00, 0x50, 0x60, 0x00, 0x56}

const errTraceOutputLimitMsg = "trace output exceeds the limit"

// TestTraceCallOutputLimit checks that a struct-log trace cannot retain output in
// proportion to the gas the caller supplies: with no limit set, a trace that
// would produce more than the default limit fails instead, and it fails early
// rather than running to the trace timeout.
func TestTraceCallOutputLimit(t *testing.T) {
	r := require.New(t)
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	test := newE2ETest(t, cfg)
	defer test.teardown()

	core := test.svr.APIServer(test.cfg.Chain.ID).CoreService()
	caller := identityset.Address(1)
	blockGas := test.cfg.Genesis.BlockGasLimitByHeight(core.TipHeight() + 1)

	trace := func(gas uint64, cfg *tracers.TraceConfig) (json.RawMessage, error) {
		_, _, tr, _ := core.TraceCall(context.Background(), caller, 0, "", 0, big.NewInt(0), gas, gasBurner, cfg)
		r.NotNil(tr)
		return tr.(*tracers.Tracer).GetResult()
	}

	for _, c := range []struct {
		name string
		cfg  *tracers.TraceConfig
	}{
		// gRPC callers and internal callers may pass no config at all
		{"nil config", nil},
		// the web3 handlers always pass a config, with a zero limit by default
		{"zero limit", &tracers.TraceConfig{Config: &logger.Config{}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			// a small trace is unaffected
			res, err := trace(30_000, c.cfg)
			r.NoError(err)
			r.NotEmpty(res)

			// a trace at the block gas limit fails at the limit, well before
			// the trace timeout fires
			start := time.Now()
			_, err = trace(blockGas, c.cfg)
			r.ErrorContains(err, errTraceOutputLimitMsg)
			r.Less(time.Since(start), 4*time.Second, "the trace must be aborted at the limit, not at the timeout")
		})
	}

	t.Run("caller limit", func(t *testing.T) {
		r := require.New(t)
		// a limit the output fits in is honored
		res, err := trace(30_000, &tracers.TraceConfig{Config: &logger.Config{Limit: 1 << 20}})
		r.NoError(err)
		r.NotEmpty(res)
		// a lower limit fails rather than returning a truncated trace
		_, err = trace(300_000, &tracers.TraceConfig{Config: &logger.Config{Limit: 4096}})
		r.ErrorContains(err, errTraceOutputLimitMsg)
	})
}

// TestTraceBlockOutputLimit checks that the results of a block trace are bounded
// together: each transaction below stays within the per-transaction limit, but
// the block as a whole exceeds the block limit, so the request fails.
func TestTraceBlockOutputLimit(t *testing.T) {
	r := require.New(t)
	cfg, sender, senderSK, _ := traceBlockSetup(t)
	test := newE2ETest(t, cfg)
	defer test.teardown()

	var (
		chainID  = cfg.Chain.ID
		gasPrice = big.NewInt(unit.Qev)
		// each transaction produces ~14 MB of struct logs, under the 16 MiB
		// per-transaction default; six of them exceed the 64 MiB block limit
		txGas  = uint64(700_000)
		acts   []*actionWithTime
		height uint64
	)
	for range 6 {
		acts = append(acts, &actionWithTime{
			mustNoErr(action.Sign(
				action.NewEnvelope(
					action.NewLegacyTx(chainID, test.nonceMgr.pop(sender), txGas, gasPrice),
					action.NewExecution("", big.NewInt(0), gasBurner),
				),
				senderSK,
			)),
			time.Now(),
		})
	}
	test.runCase(context.Background(), &testcase{
		name: "block of gas burners",
		acts: acts,
		blockExpect: func(test *e2etest, blk *block.Block, err error) {
			r.NoError(err)
			// the block also carries the system actions
			r.Greater(len(blk.Receipts), len(acts))
			height = blk.Height()
		},
	})

	// the block traces fine with a tracer whose output is small
	_, err := rpcTraceBlock(t, cfg.API.HTTPPort, height, "callTracer")
	r.NoError(err)

	// a single transaction's struct logs fit in the per-transaction limit
	hash, err := acts[0].act.Hash()
	r.NoError(err)
	_, _, tr, err := test.svr.APIServer(cfg.Chain.ID).CoreService().TraceTransaction(context.Background(), fmt.Sprintf("%x", hash), nil)
	r.NoError(err)
	_, err = tr.(*tracers.Tracer).GetResult()
	r.NoError(err)

	// the struct logs of the whole block do not fit in the block limit
	_, err = rpcTraceBlockStructLogs(cfg.API.HTTPPort, height)
	r.ErrorContains(err, errTraceOutputLimitMsg)
}

func rpcTraceBlockStructLogs(port int, height uint64) (json.RawMessage, error) {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"debug_traceBlockByNumber","params":["0x%x",{}],"id":1}`, height)
	var result struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	resp, err := resty.New().R().SetBody(body).Post(fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", result.Error.Message)
	}
	return result.Result, nil
}
