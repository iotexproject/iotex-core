package e2etest

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestWeb3API(t *testing.T) {
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
	cfg.Chain.BlobStoreRetentionDays = 1
	cfg.Genesis.InitBalanceMap[sender] = unit.ConvertIotxToRau(10000).String()
	normalizeGenesisHeights(&cfg)
	test := newE2ETest(t, cfg)
	chainID := cfg.Chain.ID
	evmID := cfg.Chain.EVMNetworkID
	gasPrice := big.NewInt(unit.Qev)
	gasFeeCap := big.NewInt(unit.Qev * 2)
	gasTipCap := big.NewInt(1)
	blobFeeCap := uint256.MustFromBig(big.NewInt(1))
	// contractBytecode, err := hex.DecodeString(`6080604052348015600e575f80fd5b506101438061001c5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c80632e64cec1146100385780636057361d14610056575b5f80fd5b610040610072565b60405161004d919061009b565b60405180910390f35b610070600480360381019061006b91906100e2565b61007a565b005b5f8054905090565b805f8190555050565b5f819050919050565b61009581610083565b82525050565b5f6020820190506100ae5f83018461008c565b92915050565b5f80fd5b6100c181610083565b81146100cb575f80fd5b50565b5f813590506100dc816100b8565b92915050565b5f602082840312156100f7576100f66100b4565b5b5f610104848285016100ce565b9150509291505056fea26469706673582212209a0dd35336aff1eb3eeb11db76aa60a1427a12c1b92f945ea8c8d1dfa337cf2264736f6c634300081a0033`)
	// r.NoError(err)
	// balance := big.NewInt(0)
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
	newDynamicTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewDynamicFeeTx(chainID, nonce, gasLimit, gasFeeCap, gasTipCap, nil)
	}
	newAccessListTx := func(nonce uint64) action.TxCommonInternal {
		return action.NewAccessListTx(chainID, nonce, gasLimit, gasPrice, nil)
	}
	test.run([]*testcase{
		{
			name: "before Vanuatu",
			preActs: []*actionWithTime{
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			},
			acts: []*actionWithTime{
				{mustNoErr(action.Sign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), senderSK)), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				ethblk, err := test.ethcli.BlockByNumber(context.Background(), big.NewInt(int64(blk.Height())))
				r.NoError(err)
				r.Equal(2, ethblk.Transactions().Len())
				blkHash := blk.HashBlock()
				r.NotEqual(common.BytesToHash(blkHash[:]), ethblk.Hash(), "block hash mismatch")
				for i := range blk.Actions {
					h := mustNoErr(blk.Actions[i].Hash())
					r.NotEqual(common.BytesToHash(h[:]), ethblk.Transactions()[i].Hash(), "action hash mismatch %d, version %d", i, blk.Actions[i].Version())
				}
			},
		},
		{
			name: "Vanuatu",
			acts: []*actionWithTime{
				{mustNoErr(action.Sign155(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), evmID, senderSK)), time.Now()},
				{mustNoErr(action.Sign155(action.NewEnvelope(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), recipient, nil)), evmID, senderSK)), time.Now()},
				{mustNoErr(action.Sign155(action.NewEnvelope(newDynamicTx(test.nonceMgr.pop((sender))), action.NewTransfer(big.NewInt(1), recipient, nil)), evmID, senderSK)), time.Now()},
				{mustNoErr(action.Sign155(action.NewEnvelope(newAccessListTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), evmID, senderSK)), time.Now()},
				{mustNoErr(action.EthSign(action.NewEnvelope(newLegacyTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), evmID, senderSK)), time.Now()},
				{mustNoErr(action.EthSign(action.NewEnvelope(newBlobTx(test.nonceMgr.pop(sender), sidecar, sidecar.BlobHashes()), action.NewTransfer(big.NewInt(1), recipient, nil)), evmID, senderSK)), time.Now()},
				{mustNoErr(action.EthSign(action.NewEnvelope(newDynamicTx(test.nonceMgr.pop((sender))), action.NewTransfer(big.NewInt(1), recipient, nil)), evmID, senderSK)), time.Now()},
				{mustNoErr(action.EthSign(action.NewEnvelope(newAccessListTx(test.nonceMgr.pop(sender)), action.NewTransfer(big.NewInt(1), sender, nil)), evmID, senderSK)), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				ethblk, err := test.ethcli.BlockByNumber(context.Background(), big.NewInt(int64(blk.Height())))
				r.NoError(err)
				r.Equal(9, ethblk.Transactions().Len())
				blkHash := blk.HashBlock()
				r.NotEqual(common.BytesToHash(blkHash[:]), ethblk.Hash())
				nativeCnt, web3Cnt := 0, 0
				for i := range blk.Actions {
					h := mustNoErr(blk.Actions[i].Hash())
					if blk.Actions[i].Encoding() == uint32(iotextypes.Encoding_IOTEX_PROTOBUF) {
						nativeCnt++
						r.NotEqual(common.BytesToHash(h[:]), ethblk.Transactions()[i].Hash(), "action hash mismatch %d, version %d", i, blk.Actions[i].Version())
					} else {
						web3Cnt++
						r.Equal(common.BytesToHash(h[:]), ethblk.Transactions()[i].Hash(), "action hash mismatch %d, version %d", i, blk.Actions[i].Version())
					}
				}
				r.Equal(1, nativeCnt)
				r.Equal(8, web3Cnt)
			},
		},
	})
}
