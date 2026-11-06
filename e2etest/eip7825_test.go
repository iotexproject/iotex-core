// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-resty/resty/v2"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// payload has the method set EnvelopeBuilder.SetAction accepts
type payload interface {
	IntrinsicGas() (uint64, error)
	SanityCheck() error
	FillAction(*iotextypes.ActionCore)
}

// _gasBurnerRuntime loops calldata[0:32] times (40 gas per round) and then
// returns the gas left as a 32-byte word
var _gasBurnerRuntime = "6000355b8015601057600190036003565b505a60005260206000f3"

// TestEIP7825 drives the EIP-7825 per-action gas cap through a full node, one
// block before and at the activation height (ToBeEnabled, as the fork is not
// named yet):
//
//   - admission: the action pool, eth_sendRawTransaction and the bundle pool
//     refuse an over-cap action of any type once the cap applies at the next
//     block, and accept it before;
//   - block validation: a block at the activation height carrying an over-cap
//     action is invalid, both when the action is unknown to the validator
//     (stateless check) and when it is already in its pool, which skips the
//     stateless check and leaves the verdict to ValidateWithState;
//   - block production: an over-cap action admitted before the fork is left
//     out of the first post-fork block and dropped, not settled with a failure
//     receipt;
//   - simulation: eth_call, ReadContract and debug_traceCall run with their
//     gas lowered to the cap rather than failing, and eth_estimateGas /
//     EstimateGasForAction report an explicit error for a call that needs
//     more than the cap.
func TestEIP7825(t *testing.T) {
	const fork = 5
	r := require.New(t)
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Genesis.YapBetaBlockHeight = 1
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	// set after normalizing, so the unscheduled Zanzibar family is not pulled
	// down with it
	cfg.Genesis.ToBeEnabledBlockHeight = fork
	for i := 3; i <= 6; i++ {
		cfg.Genesis.InitBalanceMap[identityset.Address(i).String()] = unit.ConvertIotxToRau(100000000).String()
	}
	test := newE2ETest(t, cfg)
	defer test.teardown()

	var (
		ctx          = context.Background()
		bc           = test.cs.Blockchain()
		ap           = test.cs.ActionPool()
		chainID      = cfg.Chain.ID
		evmNetworkID = cfg.Chain.EVMNetworkID
		gasPrice     = big.NewInt(unit.Qev * 2)
		overCap      = action.MaxTxGas + 1
		producerKey  = cfg.Chain.ProducerPrivateKeys()[0]
		sender       = identityset.Address(1)
		senderSK     = identityset.PrivateKey(1)
		to           = identityset.Address(30).String()
		web3URL      = fmt.Sprintf("http://localhost:%d", cfg.API.HTTPPort)
	)
	r.Greater(cfg.Genesis.BlockGasLimitByHeight(fork), action.MaxTxGas, "the block gas limit must not hide the cap")
	signWith := func(sk crypto.PrivateKey, nonce, gas uint64, act payload) *action.SealedEnvelope {
		elp := (&action.EnvelopeBuilder{}).SetChainID(chainID).SetNonce(nonce).
			SetGasPrice(gasPrice).SetGasLimit(gas).SetAction(act).Build()
		selp, err := action.Sign(elp, sk)
		r.NoError(err)
		return selp
	}
	nonce := uint64(0)
	sign := func(gas uint64, act payload) *action.SealedEnvelope {
		return signWith(senderSK, nonce, gas, act)
	}
	commit := func() *block.Block {
		blk, err := createAndCommitBlock(bc, ap, time.Now())
		r.NoError(err)
		return blk
	}
	receiptOf := func(blk *block.Block, selp *action.SealedEnvelope) *action.Receipt {
		h, err := selp.Hash()
		r.NoError(err)
		for _, rc := range blk.Receipts {
			if rc.ActionHash == h {
				return rc
			}
		}
		return nil
	}
	inPool := func(selp *action.SealedEnvelope) bool {
		h, err := selp.Hash()
		r.NoError(err)
		_, err = ap.GetActionByHash(h)
		return err == nil
	}

	// block 1: deploy the gas burner
	deploy := sign(1_000_000, action.NewExecution("", big.NewInt(0),
		mustNoErr(hex.DecodeString("601b80600b6000396000f3"+_gasBurnerRuntime))))
	r.NoError(ap.Add(ctx, deploy))
	nonce++
	blk := commit()
	deployReceipt := receiptOf(blk, deploy)
	r.NotNil(deployReceipt)
	r.EqualValues(1, deployReceipt.Status)
	burnerIo := deployReceipt.ContractAddress
	burner := common.BytesToAddress(mustNoErr(address.FromString(burnerIo)).Bytes())
	from := common.BytesToAddress(sender.Bytes())
	rounds := func(n int64) []byte { return common.LeftPadBytes(big.NewInt(n).Bytes(), 32) }
	const heavy = 450_000 // about 18M gas, above the cap and below the block gas limit
	const light = 10_000  // about 400k gas

	// block 2: nothing
	commit()
	r.EqualValues(fork-3, bc.TipHeight())

	// block fork-2 (pre-fork): an over-cap transfer is admitted and settled
	preForkTransfer := sign(overCap, action.NewTransfer(big.NewInt(1), to, nil))
	r.NoError(ap.Add(ctx, preForkTransfer))
	nonce++
	blk, err := bc.MintNewBlock(time.Now())
	r.NoError(err)
	rc := receiptOf(blk, preForkTransfer)
	r.NotNil(rc, "an over-cap action is valid before the fork")
	r.EqualValues(1, rc.Status)
	blk.Receipts = nil
	r.NoError(bc.ValidateBlock(blk), "a pre-fork block carrying an over-cap action is valid")
	r.NoError(bc.CommitBlock(blk))
	ap.Reset()
	r.EqualValues(fork-2, bc.TipHeight())

	// simulation one block before the fork (the next block is fork-1)
	type simResult struct {
		estimateLight, estimateHeavy, callGas, call20MGas, readGas, traceGas, estimateAction uint64
	}
	simulate := func(postFork bool) simResult {
		var res simResult
		var err error
		// eth_estimateGas
		res.estimateLight, err = test.ethcli.EstimateGas(ctx, ethereum.CallMsg{From: from, To: &burner, Data: rounds(light)})
		r.NoError(err)
		res.estimateHeavy, err = test.ethcli.EstimateGas(ctx, ethereum.CallMsg{From: from, To: &burner, Data: rounds(heavy)})
		if postFork {
			r.ErrorContains(err, "gas required exceeds allowance (16777216)")
		} else {
			r.NoError(err)
		}
		// eth_call without a gas limit runs with the block gas limit
		ret, err := test.ethcli.CallContract(ctx, ethereum.CallMsg{From: from, To: &burner, Data: rounds(0)}, nil)
		r.NoError(err)
		res.callGas = new(big.Int).SetBytes(ret).Uint64()
		// eth_call asking for more than the cap
		ret, err = test.ethcli.CallContract(ctx, ethereum.CallMsg{From: from, To: &burner, Data: rounds(0), Gas: 20_000_000}, nil)
		r.NoError(err)
		res.call20MGas = new(big.Int).SetBytes(ret).Uint64()
		// gRPC ReadContract
		resp, err := test.api.ReadContract(ctx, &iotexapi.ReadContractRequest{
			Execution:     action.NewExecution(burnerIo, big.NewInt(0), rounds(0)).Proto(),
			CallerAddress: sender.String(),
		})
		r.NoError(err)
		res.readGas = new(big.Int).SetBytes(mustNoErr(hex.DecodeString(resp.Data))).Uint64()
		// debug_traceCall, with the default struct logger: a named tracer
		// (callTracer etc.) cannot be used here, debug_traceCall panics on it
		// independently of this change
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"debug_traceCall","params":[{"from":"%s","to":"%s","data":"%s","gas":"%s"},"latest",{"disableStorage":true}]}`,
			from.Hex(), burner.Hex(), hexutil.Encode(rounds(0)), hexutil.EncodeUint64(20_000_000))
		traceResp, err := resty.New().R().SetBody(body).Post(web3URL)
		r.NoError(err)
		out := gjson.Get(traceResp.String(), "result.returnValue").String()
		if len(out) > 1 && out[:2] != "0x" {
			out = "0x" + out
		}
		r.NotEmpty(out, "status %d body %q", traceResp.StatusCode(), traceResp.String())
		res.traceGas = new(big.Int).SetBytes(mustNoErr(hexutil.Decode(out))).Uint64()
		// gRPC EstimateGasForAction on a signed action
		heavyExec := signWith(identityset.PrivateKey(4), 0, 0, action.NewExecution(burnerIo, big.NewInt(0), rounds(heavy)))
		est, err := test.api.EstimateGasForAction(ctx, &iotexapi.EstimateGasForActionRequest{Action: heavyExec.Proto()})
		if postFork {
			r.ErrorContains(err, "gas required exceeds allowance (16777216)")
		} else {
			r.NoError(err)
			res.estimateAction = est.Gas
		}
		return res
	}
	pre := simulate(false)
	r.Greater(pre.estimateHeavy, action.MaxTxGas)
	r.Greater(pre.estimateAction, action.MaxTxGas)
	r.Greater(pre.callGas, action.MaxTxGas)
	r.Greater(pre.call20MGas, action.MaxTxGas)
	r.Greater(pre.callGas, pre.call20MGas)
	r.Greater(pre.readGas, action.MaxTxGas)
	r.Greater(pre.traceGas, action.MaxTxGas)

	// a bundle targeting the last pre-fork block may carry an over-cap action
	bp := ap.BundlePool()
	r.NotNil(bp)
	newBundle := func(height uint64, selps ...*action.SealedEnvelope) *action.Bundle {
		b := action.NewBundle()
		r.NoError(b.Add(selps...))
		b.SetTargetBlockHeight(height)
		return b
	}
	r.NoError(bp.AddBundle(ctx, identityset.Address(5), "pre-fork",
		newBundle(fork-1, signWith(identityset.PrivateKey(5), 0, overCap, action.NewTransfer(big.NewInt(1), to, nil)))))

	// block fork-1 is built first; the over-cap transfer below is admitted
	// while the next block is still fork-1, then stays in the pool across the
	// fork
	blk, err = bc.MintNewBlock(time.Now())
	r.NoError(err)
	pending := sign(overCap, action.NewTransfer(big.NewInt(1), to, nil))
	r.NoError(ap.Add(ctx, pending))
	nonce++
	r.NoError(bc.CommitBlock(blk))
	ap.Reset()
	r.EqualValues(fork-1, bc.TipHeight())
	r.True(inPool(pending))

	// admission: every kind of action is refused over the cap, accepted at it
	createStake, err := action.NewCreateStake("cand1", unit.ConvertIotxToRau(100).String(), 1, true, nil)
	r.NoError(err)
	for _, act := range []payload{
		action.NewTransfer(big.NewInt(1), to, nil),
		action.NewExecution(burnerIo, big.NewInt(0), rounds(1)),
		action.NewExecution("", big.NewInt(0), []byte{0}),
		createStake,
		action.NewClaimFromRewardingFund(big.NewInt(1), nil, nil),
		action.NewMigrateStake(1),
	} {
		err := ap.Add(ctx, sign(overCap, act))
		r.ErrorIs(err, action.ErrGasLimitTooHigh, "%T", act)
	}
	// a raw Ethereum transaction (tx container) through eth_sendRawTransaction
	ethTx := func(gas uint64) *types.Transaction {
		toAddr := common.BytesToAddress(identityset.Address(30).Bytes())
		return types.MustSignNewTx(identityset.PrivateKey(3).EcdsaPrivateKey().(*ecdsa.PrivateKey),
			types.LatestSignerForChainID(big.NewInt(int64(evmNetworkID))), &types.DynamicFeeTx{
				ChainID:   big.NewInt(int64(evmNetworkID)),
				Nonce:     0,
				Gas:       gas,
				GasTipCap: big.NewInt(1),
				GasFeeCap: gasPrice,
				To:        &toAddr,
				Value:     big.NewInt(1),
			})
	}
	err = test.ethcli.SendTransaction(ctx, ethTx(overCap))
	r.ErrorContains(err, "transaction gas limit too high")
	r.NoError(test.ethcli.SendTransaction(ctx, ethTx(action.MaxTxGas)))
	// a bundle targeting the fork block
	err = bp.AddBundle(ctx, identityset.Address(6), "post-fork",
		newBundle(fork, signWith(identityset.PrivateKey(6), 0, overCap, action.NewTransfer(big.NewInt(1), to, nil))))
	r.ErrorIs(err, action.ErrGasLimitTooHigh)

	// simulation at the fork (the next block is fork)
	post := simulate(true)
	r.Equal(pre.estimateLight, post.estimateLight, "an estimate below the cap is unchanged")
	r.Equal(post.callGas, post.call20MGas)
	for name, gas := range map[string]uint64{"eth_call": post.callGas, "eth_call 20M": post.call20MGas, "ReadContract": post.readGas, "debug_traceCall": post.traceGas} {
		r.LessOrEqual(gas, action.MaxTxGas, name)
		r.Greater(gas, action.MaxTxGas-100_000, "%s runs with the cap, not with a rejection", name)
	}

	// block validation at the fork
	bcCtx := protocol.MustGetBlockchainCtx(mustNoErr(bc.Context(ctx)))
	buildFork := func(selps ...*action.SealedEnvelope) *block.Block {
		b, err := block.NewBuilder(block.NewRunnableActionsBuilder().AddActions(selps...).Build()).
			SetVersion(1).
			SetHeight(fork).
			SetPrevBlockHash(bc.TipHash()).
			SetTimestamp(time.Now()).
			SetBaseFee(protocol.CalcBaseFee(cfg.Genesis.Blockchain, &bcCtx.Tip)).
			SignAndBuild(producerKey)
		r.NoError(err)
		return &b
	}
	// the pending transfer is in the pool, so the pool's stateless check is
	// skipped for it and only ValidateWithState can reject it
	r.True(inPool(pending))
	err = bc.ValidateBlock(buildFork(pending))
	r.ErrorIs(err, action.ErrGasLimitTooHigh)
	unknown := signWith(identityset.PrivateKey(4), 0, overCap, action.NewExecution(burnerIo, big.NewInt(0), rounds(1)))
	r.False(inPool(unknown))
	err = bc.ValidateBlock(buildFork(unknown))
	r.ErrorIs(err, action.ErrGasLimitTooHigh)

	// block production at the fork leaves the pending transfer out
	pendingHash, err := pending.Hash()
	r.NoError(err)
	blk = commit()
	r.EqualValues(fork, blk.Height())
	for _, selp := range blk.Actions {
		h, err := selp.Hash()
		r.NoError(err)
		r.NotEqual(pendingHash, h, "an over-cap action is not included")
		r.LessOrEqual(selp.Gas(), action.MaxTxGas)
	}
	r.Nil(receiptOf(blk, pending), "an over-cap action gets no receipt")
	r.False(inPool(pending), "the proposer drops it")
}
