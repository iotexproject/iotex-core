package e2etest

import (
	"context"
	"math/big"
	"testing"
	"time"

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
	cfg := initCfg(r)
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.VanuatuBlockHeight = 5
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(10000).String()
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
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
	test.run([]*testcase{
		{
			name: "reject before Vanuatu",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelop(newDynamicTx(test.nonceMgr[(sender)]), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, action.ErrInvalidAct)
				r.ErrorContains(err, "dynamic fee tx is not enabled")
			}}},
		},
		{
			name: "success",
			preActs: []*actionWithTime{
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			},
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelop(newDynamicTx(test.nonceMgr.pop((sender))), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
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
				r.Equal([]*action.TransactionLog{
					{
						Type:      iotextypes.TransactionLogType_NATIVE_TRANSFER,
						Amount:    transfer.Amount(),
						Sender:    sender,
						Recipient: transfer.Recipient(),
					},
					{
						Type:      iotextypes.TransactionLogType_GAS_FEE,
						Amount:    new(big.Int).Mul(baseFee, big.NewInt(int64(receipt.GasConsumed))),
						Sender:    sender,
						Recipient: address.RewardingPoolAddr,
					},
					{
						Type:      iotextypes.TransactionLogType_PRIORITY_FEE,
						Amount:    new(big.Int).Mul(act.GasTipCap(), big.NewInt(int64(receipt.GasConsumed))),
						Sender:    sender,
						Recipient: address.RewardingPoolAddr,
					},
				}, receipt.TransactionLogs())
			}}},
		},
		{
			name: "suggestGasTipCap",
			act:  &actionWithTime{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				tip, err := test.ethcli.SuggestGasTipCap(context.Background())
				r.NoError(err)
				r.NotNil(tip)
				r.True(tip.Cmp(big.NewInt(0)) > 0, "tip should be positive")
			}}},
		},
	})
}
