package e2etest

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestDynamicFeeTx(t *testing.T) {
	r := require.New(t)
	sender := identityset.Address(10).String()
	senderSK := identityset.PrivateKey(10)
	recipient := identityset.Address(11).String()
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.VanuatuBlockHeight = 5
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(10000).String()
	normalizeGenesisHeights(&cfg)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	gasFeeCap := big.NewInt(unit.Qev * 2)
	gasTipCap := big.NewInt(1)
	newDynamicTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewDynamicFeeTx(chainID, nonce, gasLimit, gasFeeCap, gasTipCap, nil)
	}
	newLegacyTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewLegacyTx(chainID, nonce, gasLimit, gasPrice)
	}
	balance := big.NewInt(0)
	test.run([]*testcase{
		{
			name: "reject before Vanuatu",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelope(newDynamicTx(test.nonceMgr[(sender)]), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, action.ErrInvalidAct)
				r.ErrorContains(err, "dynamic fee tx is not enabled")
			}}},
		},
		{
			name: "reach Vanuatua",
			preActs: []*actionWithTime{
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			},
		},
		{
			name: "tip+base<feeCap",
			preFunc: func(e *e2etest) {
				b, err := e.ethcli.BalanceAt(context.Background(), common.BytesToAddress(identityset.Address(10).Bytes()), nil)
				r.NoError(err)
				balance.Set(b)
			},
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelope(newDynamicTx(test.nonceMgr.pop((sender))), action.NewTransfer(big.NewInt(1), recipient, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.Nil(err)
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				// check transaction logs
				resp, rerr := test.api.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{StartHeight: receipt.BlockHeight, Count: 1})
				r.NoError(rerr)
				r.Len(resp.Blocks, 1)
				d := &block.Deserializer{}
				blk, err := d.SetEvmNetworkID(test.cfg.Chain.EVMNetworkID).FromBlockProto(resp.Blocks[0].Block)
				r.NoError(err)
				baseFee := blk.BaseFee()
				transfer := act.Envelope.Action().(*action.Transfer)
				gasFee := new(big.Int).Mul(baseFee, big.NewInt(int64(receipt.GasConsumed)))
				priorityFee := new(big.Int).Mul(act.GasTipCap(), big.NewInt(int64(receipt.GasConsumed)))
				r.Equal([]*action.TransactionLog{
					{
						Type:      iotextypes.TransactionLogType_NATIVE_TRANSFER,
						Amount:    transfer.Amount(),
						Sender:    sender,
						Recipient: transfer.Recipient(),
					},
					{
						Type:      iotextypes.TransactionLogType_GAS_FEE,
						Amount:    gasFee,
						Sender:    sender,
						Recipient: address.RewardingPoolAddr,
					},
					{
						Type:      iotextypes.TransactionLogType_PRIORITY_FEE,
						Amount:    priorityFee,
						Sender:    sender,
						Recipient: address.RewardingPoolAddr,
					},
				}, receipt.TransactionLogs())
				// check balance
				b, err := test.ethcli.BalanceAt(context.Background(), common.BytesToAddress(identityset.Address(10).Bytes()), big.NewInt(int64(blk.Height())))
				r.NoError(err)
				spend := new(big.Int).Add(transfer.Amount(), gasFee)
				spend = spend.Add(spend, priorityFee)
				r.Equal(balance.Sub(balance, spend), b)
				// check receipt
				expectPrice := new(big.Int).Add(baseFee, act.GasTipCap())
				r.Equal(expectPrice, receipt.EffectiveGasPrice)
			}}},
		},
		{
			name: "tip+base>feeCap",
			preFunc: func(e *e2etest) {
				b, err := e.ethcli.BalanceAt(context.Background(), common.BytesToAddress(identityset.Address(10).Bytes()), nil)
				r.NoError(err)
				balance.Set(b)
			},
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelope(action.NewDynamicFeeTx(chainID, test.nonceMgr.pop((sender)), gasLimit, big.NewInt(unit.Qev+1000), big.NewInt(2000), nil), action.NewTransfer(big.NewInt(1), recipient, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.Nil(err)
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				resp, rerr := test.api.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{StartHeight: receipt.BlockHeight, Count: 1})
				r.NoError(rerr)
				r.Len(resp.Blocks, 1)
				d := &block.Deserializer{}
				blk, err := d.SetEvmNetworkID(test.cfg.Chain.EVMNetworkID).FromBlockProto(resp.Blocks[0].Block)
				r.NoError(err)
				baseFee := blk.BaseFee()
				transfer := act.Envelope.Action().(*action.Transfer)
				gasFee := new(big.Int).Mul(baseFee, big.NewInt(int64(receipt.GasConsumed)))
				priorityFee := new(big.Int).Mul(new(big.Int).Sub(act.GasFeeCap(), baseFee), big.NewInt(int64(receipt.GasConsumed)))
				r.Equal([]*action.TransactionLog{
					{
						Type:      iotextypes.TransactionLogType_NATIVE_TRANSFER,
						Amount:    transfer.Amount(),
						Sender:    sender,
						Recipient: transfer.Recipient(),
					},
					{
						Type:      iotextypes.TransactionLogType_GAS_FEE,
						Amount:    gasFee,
						Sender:    sender,
						Recipient: address.RewardingPoolAddr,
					},
					{
						Type:      iotextypes.TransactionLogType_PRIORITY_FEE,
						Amount:    priorityFee,
						Sender:    sender,
						Recipient: address.RewardingPoolAddr,
					},
				}, receipt.TransactionLogs())
				// check balance
				b, err := test.ethcli.BalanceAt(context.Background(), common.BytesToAddress(identityset.Address(10).Bytes()), big.NewInt(int64(blk.Height())))
				r.NoError(err)
				spend := new(big.Int).Add(transfer.Amount(), gasFee)
				spend = spend.Add(spend, priorityFee)
				r.Equal(balance.Sub(balance, spend), b)
				// check receipt
				r.Equal(act.GasFeeCap(), receipt.EffectiveGasPrice)
			}}},
		},
		{
			name: "suggestGasTipCap",
			act:  &actionWithTime{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				tip, err := test.ethcli.SuggestGasTipCap(context.Background())
				r.NoError(err)
				r.NotNil(tip)
				r.True(tip.Cmp(big.NewInt(0)) > 0, "tip should be positive")
			}}},
		},
	})
}
