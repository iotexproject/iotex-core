package e2etest

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
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
	evmNetworkID := cfg.Chain.EVMNetworkID
	gasPrice := big.NewInt(unit.Qev)
	gasFeeCap := big.NewInt(unit.Qev * 2)
	gasTipCap := big.NewInt(1)
	newDynamicTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewDynamicFeeTx(chainID, nonce, gasLimit, gasFeeCap, gasTipCap, nil)
	}
	newLegacyTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewLegacyTx(chainID, nonce, gasLimit, gasPrice)
	}
	newDynamicTxWeb3 := func(nonce uint64) (*action.SealedEnvelope, error) {
		addr := mustNoErr(address.FromString(sender))
		to := common.BytesToAddress(addr.Bytes())
		txdata := types.DynamicFeeTx{
			ChainID:   big.NewInt(int64(evmNetworkID)),
			Nonce:     nonce,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			Gas:       gasLimit,
			To:        &to,
			Value:     big.NewInt(1),
		}
		tx := types.MustSignNewTx(senderSK.EcdsaPrivateKey().(*ecdsa.PrivateKey), types.NewCancunSigner(txdata.ChainID), &txdata)
		_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
		if err != nil {
			return nil, err
		}
		req := &iotextypes.Action{
			Core:         mustNoErr(action.EthRawToContainer(chainID, hex.EncodeToString(mustNoErr(tx.MarshalBinary())))),
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_TX_CONTAINER,
		}
		return (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(req)
	}
	test.run([]*testcase{
		{
			name: "accept after Vanuatu",
			preActs: []*actionWithTime{
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			},
			act: &actionWithTime{
				mustNoErr(newDynamicTxWeb3(test.nonceMgr.pop((sender)))), time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.Nil(err)
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				// check transaction logs
				resp := mustNoErr(test.api.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{StartHeight: receipt.BlockHeight, Count: 1}))
				r.Len(resp.Blocks, 1)
				d := &block.Deserializer{}
				blk := mustNoErr(d.SetEvmNetworkID(test.cfg.Chain.EVMNetworkID).FromBlockProto(resp.Blocks[0].Block))
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
			name: "protobuf does not support blob",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelope(newDynamicTx(test.nonceMgr[sender]), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, action.ErrInvalidAct)
				r.ErrorContains(err, "protobuf encoding only supports legacy tx")
			}}},
		},
		{
			name: "suggestGasTipCap",
			act:  &actionWithTime{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				tip := mustNoErr(test.ethcli.SuggestGasTipCap(context.Background()))
				r.NotNil(tip)
				r.True(tip.Cmp(big.NewInt(0)) > 0, "tip should be positive")
			}}},
		},
	})
}
