package e2etest

import (
	"context"
	_ "embed"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
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

func TestArchiveModeByErigon(t *testing.T) {
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
			b0, err := test.ethcli.BalanceAt(ctx, ethHolder, big.NewInt(int64(blk.Height())))
			r.NoError(err)
			b1, err := test.ethcli.BalanceAt(ctx, ethHolder, nil)
			r.NoError(err)
			r.Equal(new(big.Int).Add(b0, amount).String(), b1.String())
			b0, err = test.ethcli.BalanceAt(ctx, ethSender, big.NewInt(int64(blk.Height())))
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
			b, err := test.ethcli.BalanceAt(ctx, ethSender, big.NewInt(int64(blk.Height())))
			r.NoError(err)
			r.Equal(balance.String(), b.String())
			b, err = test.ethcli.BalanceAt(ctx, ethSender, nil)
			r.NoError(err)
			balance.Sub(balance, consumed)
			r.Equal(balance.String(), b.String())
		},
	})
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
			b, err := test.ethcli.BalanceAt(ctx, ethSender, big.NewInt(int64(blk.Height())))
			r.NoError(err)
			r.Equal(balance.String(), b.String())
			b, err = test.ethcli.BalanceAt(ctx, ethSender, nil)
			r.NoError(err)
			consumed := new(big.Int).Mul(gasPrice, big.NewInt(int64(blk.Receipts[0].GasConsumed)))
			balance.Sub(balance, consumed)
			r.Equal(balance.String(), b.String())
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
			}, big.NewInt(int64(blk.Height())))
			r.NoError(err)
			r.Equal("0", big.NewInt(0).SetBytes(b).String())
			b, err = test.ethcli.CallContract(ctx, ethereum.CallMsg{
				To:   &ethContractAddr,
				Data: mustCallData("balanceOf(address)", ethHolder),
			}, nil)
			r.NoError(err)
			r.Equal("1", big.NewInt(0).SetBytes(b).String())
		},
	})
}
