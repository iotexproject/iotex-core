package e2etest

import (
	"context"
	_ "embed"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

var (
	//go:embed erc20_bytecode
	erc20Bytecode string
	//go:embed erc20_abi.json
	erc20ABIJSON string
	erc20ABI     abi.ABI
)

func init() {
	var err error
	erc20ABI, err = abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		panic(err)
	}
}

func TestErigonArchiveContract(t *testing.T) {
	r := require.New(t)
	sender := identityset.Address(10).String()
	senderSK := identityset.PrivateKey(10)
	ethSender := common.BytesToAddress(identityset.Address(10).Bytes())
	holder := identityset.Address(11).String()
	ethHolder := common.BytesToAddress(identityset.Address(11).Bytes())
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.UpernavikBlockHeight = 5
	cfg.Genesis.VanuatuBlockHeight = 5
	balance := unit.ConvertIotxToRau(2000000)
	cfg.Genesis.InitBalanceMap[sender] = balance.String()
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	newLegacyTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewLegacyTx(chainID, nonce, gasLimit, gasPrice)
	}
	bytecode, err := hex.DecodeString(erc20Bytecode)
	r.NoError(err)
	mustCallData := func(m string, args ...any) []byte {
		data, err := abiCall(erc20ABI, m, args...)
		r.NoError(err)
		return data
	}
	contractAddr := ""
	ethContractAddr := common.Address{}
	ctx := context.Background()
	consumeFee := func(receipt *action.Receipt, from string) *big.Int {
		fee := big.NewInt(0)
		for _, l := range receipt.TransactionLogs() {
			if l.Sender == from {
				fee.Add(fee, l.Amount)
			}
		}
		return fee
	}
	test.runCase(ctx, &testcase{
		name: "transfer",
		act: &actionWithTime{
			mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(10), holder, nil)), senderSK)),
			time.Now(),
		},
		blockExpect: func(test *e2etest, blk *block.Block, err error) {
			r.NoError(err)
			r.EqualValues(1, blk.Receipts[0].Status)
			t.Log("transfer success, block height:", blk.Height())
			// check holder balance
			amount := big.NewInt(10)
			b0, err := test.ethcli.BalanceAt(ctx, ethHolder, big.NewInt(int64(blk.Height()-1)))
			r.NoError(err)
			b1, err := test.ethcli.BalanceAt(ctx, ethHolder, nil)
			r.NoError(err)
			r.Equal(new(big.Int).Add(b0, amount).String(), b1.String())
			b0, err = test.ethcli.BalanceAt(ctx, ethSender, big.NewInt(int64(blk.Height()-1)))
			r.NoError(err)
			b1, err = test.ethcli.BalanceAt(ctx, ethSender, nil)
			r.NoError(err)
			consumed := new(big.Int).Mul(gasPrice, big.NewInt(int64(blk.Receipts[0].GasConsumed)))
			r.Equal(new(big.Int).Sub(balance, consumed.Add(consumed, amount)).String(), b1.String())
			balance = b1
		},
	})
	registerAmount := unit.ConvertIotxToRau(1200000)
	test.runCase(ctx, &testcase{
		name: "register candidate",
		acts: []*actionWithTime{
			{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), mustNoErr(action.NewCandidateRegister("cand1", identityset.Address(1).String(), identityset.Address(1).String(), sender, registerAmount.String(), 1, true, nil))), senderSK)), time.Now()},
		},
		blockExpect: func(test *e2etest, blk *block.Block, err error) {
			r.NoError(err)
			r.EqualValues(1, blk.Receipts[0].Status)
			t.Log("register candidate, block height:", blk.Height())
			// check sender balance
			consumed := consumeFee(blk.Receipts[0], sender)
			ctx := context.Background()
			b, err := test.ethcli.BalanceAt(ctx, ethSender, big.NewInt(int64(blk.Height()-1)))
			r.NoError(err)
			r.Equal(balance.String(), b.String())
			b, err = test.ethcli.BalanceAt(ctx, ethSender, nil)
			r.NoError(err)
			balance.Sub(balance, consumed)
			r.Equal(balance.String(), b.String())
		},
	})
	var code []byte
	test.runCase(ctx, &testcase{
		name: "deploy erc20",
		act: &actionWithTime{
			mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewExecution("", big.NewInt(0), bytecode)), senderSK)),
			time.Now(),
		},
		blockExpect: func(test *e2etest, blk *block.Block, err error) {
			r.NoError(err)
			// contract deploy success
			r.EqualValues(1, blk.Receipts[0].Status)
			contractAddr = blk.Receipts[0].ContractAddress
			addr, err := address.FromString(contractAddr)
			r.NoError(err)
			ethContractAddr = common.BytesToAddress(addr.Bytes())
			t.Log("contract address:", contractAddr, "block height:", blk.Height())
			// check sender balance
			ctx := context.Background()
			b, err := test.ethcli.BalanceAt(ctx, ethSender, big.NewInt(int64(blk.Height()-1)))
			r.NoError(err)
			r.Equal(balance.String(), b.String())
			b, err = test.ethcli.BalanceAt(ctx, ethSender, nil)
			r.NoError(err)
			consumed := new(big.Int).Mul(gasPrice, big.NewInt(int64(blk.Receipts[0].GasConsumed)))
			balance.Sub(balance, consumed)
			r.Equal(balance.String(), b.String())
			code, err = test.ethcli.CodeAt(ctx, ethContractAddr, nil)
			r.NoError(err)
		},
	})
	test.runCase(ctx, &testcase{
		name: "mint",
		act: &actionWithTime{
			mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewExecution(contractAddr, big.NewInt(0), mustCallData("mint(address,uint256)", ethHolder, big.NewInt(1)))), senderSK)),
			time.Now(),
		},
		blockExpect: func(test *e2etest, blk *block.Block, err error) {
			r.NoError(err)
			r.EqualValues(1, blk.Receipts[0].Status, blk.Receipts[0].ExecutionRevertMsg())
			t.Log("mint success, block height:", blk.Height())
			// check holder erc20 balance
			ctx := context.Background()
			b, err := test.ethcli.CallContract(ctx, ethereum.CallMsg{
				To:   &ethContractAddr,
				Data: mustCallData("balanceOf(address)", ethHolder),
			}, big.NewInt(int64(blk.Height()-1)))
			r.NoError(err)
			r.Equal("0", big.NewInt(0).SetBytes(b).String())
			b, err = test.ethcli.CallContract(ctx, ethereum.CallMsg{
				To:   &ethContractAddr,
				Data: mustCallData("balanceOf(address)", ethHolder),
			}, nil)
			r.NoError(err)
			r.Equal("1", big.NewInt(0).SetBytes(b).String())
			// check code
			c, err := test.ethcli.CodeAt(ctx, ethContractAddr, big.NewInt(int64(blk.Height()-1)))
			r.NoError(err)
			r.Equal(code, c)
		},
	})
}

func TestErigonArchiveAccountForAllActions(t *testing.T) {
	r := require.New(t)
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Genesis.VanuatuBlockHeight = 1
	cfg.Genesis.SystemStakingContractV2Address = "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
	cfg.Genesis.SystemStakingContractV2Height = 1
	cfg.DardanellesUpgrade.BlockInterval = time.Second * 8640
	balance := unit.ConvertIotxToRau(2000000)
	for i := 0; i < 10; i++ {
		cfg.Genesis.InitBalanceMap[identityset.Address(i).String()] = balance.String()
	}
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	newLegacyTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewLegacyTx(chainID, nonce, gasLimit, gasPrice)
	}
	deployERC20, err := hex.DecodeString(erc20Bytecode)
	r.NoError(err)
	deploySystemStaking, err := hex.DecodeString(stakingContractV2Bytecode)
	r.NoError(err)
	mustCallDataSystemStaking := func(m string, args ...any) []byte {
		data, err := abiCall(staking.StakingContractABI, m, args...)
		r.NoError(err)
		return data
	}
	type actionPayload interface {
		IntrinsicGas() (uint64, error)
		SanityCheck() error
		FillAction(*iotextypes.ActionCore)
	}
	ctx := context.Background()
	check := func(act *action.SealedEnvelope) {
		test.runCase(ctx, &testcase{
			act: &actionWithTime{act, time.Now()},
			expect: []actionExpect{&functionExpect{fn: func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.NoError(err)
				r.EqualValues(iotextypes.ReceiptStatus_Success, receipt.Status)
				t.Log("contract address", receipt.ContractAddress)
				sender := act.SrcPubkey().Address()
				preBalance, err := test.ethcli.BalanceAt(ctx, common.BytesToAddress(sender.Bytes()), big.NewInt(int64(receipt.BlockHeight-1)))
				r.NoError(err)
				t.Log("pre balance:", preBalance, "block height:", receipt.BlockHeight)
				postBalance, err := test.ethcli.BalanceAt(ctx, common.BytesToAddress(sender.Bytes()), big.NewInt(int64(receipt.BlockHeight)))
				r.NoError(err)
				t.Log("post balance:", postBalance)
				preNonce, err := test.ethcli.NonceAt(ctx, common.BytesToAddress(sender.Bytes()), big.NewInt(int64(receipt.BlockHeight-1)))
				r.NoError(err)
				t.Log("pre nonce:", preNonce)
				if !(preNonce == 1 && act.Nonce() == 0) {
					r.Equal(act.Nonce(), preNonce, "prenonce mismatch")
				}
				postNonce, err := test.ethcli.NonceAt(ctx, common.BytesToAddress(sender.Bytes()), big.NewInt(int64(receipt.BlockHeight)))
				r.NoError(err)
				t.Log("post nonce:", postNonce)
				cost := costOfAction(act.SrcPubkey().Address().String(), receipt)
				r.Equal(postBalance.String(), big.NewInt(0).Sub(preBalance, cost).String(), "balance mismatch")
				r.Equal(act.Nonce()+1, postNonce, "postnonce mismatch")
			}}},
		})
	}
	type payloadWithSender struct {
		payload actionPayload
		sender  int
	}
	payloads := []payloadWithSender{
		{action.NewExecution("", big.NewInt(0), append(deploySystemStaking, mustCallDataSystemStaking("", unit.ConvertIotxToRau(100))...)), 1}, // deploy systemstaking contract
		{action.NewTransfer(big.NewInt(10), identityset.Address(11).String(), nil), rand.Intn(10)},
		{action.NewExecution("", big.NewInt(0), deployERC20), rand.Intn(10)},
		{action.NewDepositToRewardingFund(big.NewInt(10000), nil), rand.Intn(10)},
		{mustNoErr(action.NewCandidateRegister("cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(1).String(), unit.ConvertIotxToRau(1200000).String(), 0, false, nil)), rand.Intn(10)},
		{mustNoErr(action.NewCreateStake("cand1", unit.ConvertIotxToRau(200).String(), 1, true, nil)), 1},
		{action.NewMigrateStake(1), 1},
	}

	for i := range payloads {
		senderIdx := payloads[i].sender
		act := mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(identityset.Address(senderIdx).String())), payloads[i].payload), identityset.PrivateKey(senderIdx)))
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) { check(act) })
	}
}

func costOfAction(sender string, receipt *action.Receipt) *big.Int {
	cost := big.NewInt(0)
	for _, l := range receipt.TransactionLogs() {
		if l.Sender == sender {
			cost.Add(cost, l.Amount)
		}
		if l.Recipient == sender {
			cost.Sub(cost, l.Amount)
		}
	}
	return cost
}
