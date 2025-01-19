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
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
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

func TestBlobTx(t *testing.T) {
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
	cfg.Chain.BlobStoreRetentionDays = 1
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(10000).String()
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	evmNetworkID := cfg.Chain.EVMNetworkID
	gasPrice := big.NewInt(unit.Qev)
	gasFeeCap := big.NewInt(unit.Qev * 2)
	gasTipCap := big.NewInt(1)
	blobFeeCap := uint256.NewInt(1)
	var (
		testBlob       = kzg4844.Blob{1, 2, 3, 4}
		testBlobCommit = mustNoErr(kzg4844.BlobToCommitment(testBlob))
		testBlobProof  = mustNoErr(kzg4844.ComputeBlobProof(testBlob, testBlobCommit))
	)
	sidecar := &types.BlobTxSidecar{
		Blobs:       []kzg4844.Blob{testBlob},
		Commitments: []kzg4844.Commitment{testBlobCommit},
		Proofs:      []kzg4844.Proof{testBlobProof},
	}
	newBlobTx := func(nonce uint64, sidecar *types.BlobTxSidecar, blobHashes []common.Hash) action.TxCommonInternal {
		return action.NewBlobTx(chainID, nonce, gasLimit, gasTipCap, gasFeeCap, nil, action.NewBlobTxData(blobFeeCap, blobHashes, sidecar))
	}
	newLegacyTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewLegacyTx(chainID, nonce, gasLimit, gasPrice)
	}
	newBlobTxWeb3 := func(nonce uint64, addr string, value *big.Int, blobFee *uint256.Int, sidecar *types.BlobTxSidecar, blobHashes []common.Hash) (*action.SealedEnvelope, error) {
		to := mustNoErr(address.FromString(addr))
		txdata := types.BlobTx{
			ChainID:    uint256.NewInt(uint64(evmNetworkID)),
			Nonce:      nonce,
			GasTipCap:  uint256.MustFromBig(gasTipCap),
			GasFeeCap:  uint256.MustFromBig(gasFeeCap),
			Gas:        gasLimit,
			To:         common.BytesToAddress(to.Bytes()),
			Value:      uint256.MustFromBig(value),
			BlobFeeCap: blobFee,
			BlobHashes: blobHashes,
			Sidecar:    sidecar,
		}
		tx := types.MustSignNewTx(senderSK.EcdsaPrivateKey().(*ecdsa.PrivateKey), types.NewCancunSigner(txdata.ChainID.ToBig()), &txdata)
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
			name: "must have blobs",
			preActs: []*actionWithTime{
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			},
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelope(newBlobTx(test.nonceMgr[sender], nil, nil), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorContains(err, "blobless blob transaction")
			}}},
		},
		{
			name: "fail to verify blobs",
			act: &actionWithTime{
				mustNoErr(newBlobTxWeb3(test.nonceMgr[sender], sender, big.NewInt(1), blobFeeCap, sidecar, []common.Hash{{}})),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorContains(err, "mismatches transaction")
			}}},
		},
		{
			name: "blobfee too low",
			act: &actionWithTime{
				mustNoErr(newBlobTxWeb3(test.nonceMgr[sender], sender, big.NewInt(1), uint256.NewInt(0), sidecar, sidecar.BlobHashes())),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, errReceiptNotFound)
			}}},
		},
		{
			name: "protobuf does not support blob",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelope(newBlobTx(test.nonceMgr[sender], nil, []common.Hash{{}}), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, action.ErrInvalidAct)
				r.ErrorContains(err, "protobuf encoding only supports legacy tx")
			}}},
		},
		{
			name: "accept after Vanuatu",
			act: &actionWithTime{
				mustNoErr(newBlobTxWeb3(test.nonceMgr.pop(sender), sender, big.NewInt(1), blobFeeCap, sidecar, sidecar.BlobHashes())),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.Nil(err)
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				// prepare data
				resp := mustNoErr(test.api.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{StartHeight: receipt.BlockHeight, Count: 1}))
				r.Len(resp.Blocks, 1)
				blk := mustNoErr((&block.Deserializer{}).SetEvmNetworkID(test.cfg.Chain.EVMNetworkID).FromBlockProto(resp.Blocks[0].Block))
				baseFee := blk.BaseFee()
				transfer := act.Envelope.Action().(*action.Transfer)
				// check transaction logs
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
					{
						Type:      iotextypes.TransactionLogType_BLOB_FEE,
						Amount:    new(big.Int).Mul(act.BlobGasFeeCap(), big.NewInt(int64(receipt.BlobGasUsed))),
						Sender:    sender,
						Recipient: address.RewardingPoolAddr,
					},
				}, receipt.TransactionLogs())
				// retrieve the blobs via api
				blobs := mustNoErr(test.getBlobs(receipt.BlockHeight))
				r.Len(blobs, 1)
				r.Equal(act.BlobTxSidecar(), blobs[0].BlobSidecar)
			}}},
		},
		{
			name: "6 blobs per block at most",
			acts: []*actionWithTime{
				{mustNoErr(newBlobTxWeb3(test.nonceMgr.pop(sender), sender, big.NewInt(1), blobFeeCap, sidecar, sidecar.BlobHashes())), time.Now()},
				{mustNoErr(newBlobTxWeb3(test.nonceMgr.pop(sender), sender, big.NewInt(1), blobFeeCap, sidecar, sidecar.BlobHashes())), time.Now()},
				{mustNoErr(newBlobTxWeb3(test.nonceMgr.pop(sender), sender, big.NewInt(1), blobFeeCap, sidecar, sidecar.BlobHashes())), time.Now()},
				{mustNoErr(newBlobTxWeb3(test.nonceMgr.pop(sender), sender, big.NewInt(1), blobFeeCap, sidecar, sidecar.BlobHashes())), time.Now()},
				{mustNoErr(newBlobTxWeb3(test.nonceMgr.pop(sender), sender, big.NewInt(1), blobFeeCap, sidecar, sidecar.BlobHashes())), time.Now()},
				{mustNoErr(newBlobTxWeb3(test.nonceMgr.pop(sender), sender, big.NewInt(1), blobFeeCap, sidecar, sidecar.BlobHashes())), time.Now()},
				{mustNoErr(newBlobTxWeb3(test.nonceMgr.pop(sender), sender, big.NewInt(1), blobFeeCap, sidecar, sidecar.BlobHashes())), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				confirmedBlobTxCnt := 0
				for _, act := range blk.Actions {
					if act.TxType() == action.BlobTxType {
						confirmedBlobTxCnt++
					}
				}
				r.Equal(6, confirmedBlobTxCnt)
			},
		},
	})
}
