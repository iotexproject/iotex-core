package e2etest

import (
	"context"
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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestBlobTx(t *testing.T) {
	r := require.New(t)
	sender := identityset.Address(10).String()
	senderSK := identityset.PrivateKey(10)
	cfg := initCfg(r)
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.VanuatuBlockHeight = 5
	cfg.Chain.BlobStoreRetentionDays = 1
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(10000).String()
	normalizeGenesisHeights(&cfg)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	gasFeeCap := big.NewInt(unit.Qev * 2)
	gasTipCap := big.NewInt(1)
	blobFeeCap := uint256.MustFromBig(big.NewInt(1))
	contractBytecode, err := hex.DecodeString(`6080604052348015600e575f80fd5b506101438061001c5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c80632e64cec1146100385780636057361d14610056575b5f80fd5b610040610072565b60405161004d919061009b565b60405180910390f35b610070600480360381019061006b91906100e2565b61007a565b005b5f8054905090565b805f8190555050565b5f819050919050565b61009581610083565b82525050565b5f6020820190506100ae5f83018461008c565b92915050565b5f80fd5b6100c181610083565b81146100cb575f80fd5b50565b5f813590506100dc816100b8565b92915050565b5f602082840312156100f7576100f66100b4565b5b5f610104848285016100ce565b9150509291505056fea26469706673582212209a0dd35336aff1eb3eeb11db76aa60a1427a12c1b92f945ea8c8d1dfa337cf2264736f6c634300081a0033`)
	r.NoError(err)
	var (
		testBlob       = kzg4844.Blob{1, 2, 3, 4}
		testBlobCommit = assertions.MustNoErrorV(kzg4844.BlobToCommitment(testBlob))
		testBlobProof  = assertions.MustNoErrorV(kzg4844.ComputeBlobProof(testBlob, testBlobCommit))
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
	test.run([]*testcase{
		{
			name: "reject before Vanuatu",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr[(sender)], sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, action.ErrInvalidAct)
				r.ErrorContains(err, "blob tx is not enabled")
			}}},
		},
		{
			name: "must have blobs",
			preActs: []*actionWithTime{
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			},
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr[sender], nil, nil), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorContains(err, "blobless blob transaction")
			}}},
		},
		{
			name: "fail to verify blobs",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr[sender], sidecar, []common.Hash{common.Hash{}}), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorContains(err, "mismatches transaction")
			}}},
		},
		{
			name: "blobfee too low",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelop(action.NewBlobTx(chainID, test.nonceMgr[sender], gasLimit, gasTipCap, gasFeeCap, nil, action.NewBlobTxData(uint256.NewInt(0), sidecar.BlobHashes(), sidecar)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, action.ErrUnderpriced)
				r.ErrorContains(err, "blob fee cap is too low")
			}}},
		},
		{
			name: "cannot create contract",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr[sender], sidecar, sidecar.BlobHashes()), action.NewExecution("", big.NewInt(0), contractBytecode)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.ErrorIs(err, errReceiptNotFound)
			}}},
		},
		{
			name: "accept after Vanuatu",
			act: &actionWithTime{
				mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)),
				time.Now(),
			},
			expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				r.Nil(err)
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				// prepare data
				resp, rerr := test.api.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{StartHeight: receipt.BlockHeight, Count: 1})
				r.NoError(rerr)
				r.Len(resp.Blocks, 1)
				d := &block.Deserializer{}
				blk, err := d.SetEvmNetworkID(test.cfg.Chain.EVMNetworkID).FromBlockProto(resp.Blocks[0].Block)
				r.NoError(err)
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
				blobs, err := test.getBlobs(receipt.BlockHeight)
				r.NoError(err)
				r.Len(blobs, 1)
				r.Equal(act.BlobTxSidecar(), blobs[0].BlobSidecar)
			}}},
		},
		{
			name: "6 blobs per block at most",
			acts: []*actionWithTime{
				{mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelop(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				r.NoError(err)
				confirmedBlobTxCnt := 0
				for _, act := range blk.Actions {
					if act.Version() == action.BlobTxType {
						confirmedBlobTxCnt++
					}
				}
				r.Equal(6, confirmedBlobTxCnt)
			},
		},
	})
}
