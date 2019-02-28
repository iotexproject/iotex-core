// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_dispatcher"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	testTriePath = "trie.test"
	testDBPath   = "db.test"
)

var (
	testTransfer, _ = testutil.SignedTransfer(ta.Addrinfo["alfa"].String(),
		ta.Keyinfo["alfa"].PriKey, 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPrice))

	testTransferPb = testTransfer.Proto()

	testExecution, _ = testutil.SignedExecution(ta.Addrinfo["bravo"].String(),
		ta.Keyinfo["bravo"].PriKey, 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPrice), []byte{})

	testExecutionPb = testExecution.Proto()

	testTransfer1, _ = testutil.SignedTransfer(ta.Addrinfo["charlie"].String(), ta.Keyinfo["producer"].PriKey, 1,
		big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	transferHash1 = testTransfer1.Hash()
	testVote1, _  = testutil.SignedVote(ta.Addrinfo["charlie"].String(), ta.Keyinfo["charlie"].PriKey, 5,
		testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	voteHash1         = testVote1.Hash()
	testExecution1, _ = testutil.SignedExecution(ta.Addrinfo["delta"].String(), ta.Keyinfo["producer"].PriKey, 5,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	executionHash1    = testExecution1.Hash()
	testExecution2, _ = testutil.SignedExecution(ta.Addrinfo["delta"].String(), ta.Keyinfo["charlie"].PriKey, 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice), []byte{1})
	executionHash2    = testExecution2.Hash()
	testExecution3, _ = testutil.SignedExecution(ta.Addrinfo["delta"].String(), ta.Keyinfo["alfa"].PriKey, 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice), []byte{1})
	executionHash3 = testExecution3.Hash()
)

var (
	getAccountTests = []struct {
		in           string
		address      string
		balance      string
		nonce        uint64
		pendingNonce uint64
	}{
		{ta.Addrinfo["charlie"].String(),
			"io1hw79kmqxlp33h7t83wrf9gkduy58th4vmkkue4",
			"3",
			8,
			9,
		},
		{
			ta.Addrinfo["producer"].String(),
			"io14485vn8markfupgy86at5a0re78jll0pmq8fjv",
			"9999999999999999999999999991",
			1,
			6,
		},
	}

	getActionsTests = []struct {
		start      uint64
		count      uint64
		numActions int
	}{
		{
			0,
			11,
			11,
		},
		{
			11,
			21,
			4,
		},
	}

	getActionTests = []struct {
		checkPending bool
		in           string
		nonce        uint64
		senderPubKey string
	}{
		{
			false,
			hex.EncodeToString(transferHash1[:]),
			1,
			keypair.EncodePublicKey(testTransfer1.SrcPubkey()),
		},
		{
			false,
			hex.EncodeToString(voteHash1[:]),
			5,
			keypair.EncodePublicKey(testVote1.SrcPubkey()),
		},
		{
			true,
			hex.EncodeToString(executionHash1[:]),
			5,
			keypair.EncodePublicKey(testExecution1.SrcPubkey()),
		},
	}

	getActionsByAddressTests = []struct {
		address    string
		start      uint64
		count      uint64
		numActions int
	}{
		{
			ta.Addrinfo["producer"].String(),
			0,
			3,
			2,
		},
		{
			ta.Addrinfo["charlie"].String(),
			1,
			8,
			8,
		},
	}

	getUnconfirmedActionsByAddressTests = []struct {
		address    string
		start      uint64
		count      uint64
		numActions int
	}{
		{
			ta.Addrinfo["producer"].String(),
			0,
			4,
			4,
		},
	}

	getActionsByBlockTests = []struct {
		blkHeight  uint64
		start      uint64
		count      uint64
		numActions int
	}{
		{
			2,
			0,
			7,
			7,
		},
		{
			4,
			0,
			5,
			5,
		},
	}

	getBlockMetasTests = []struct {
		start   uint64
		count   uint64
		numBlks int
	}{
		{
			0,
			4,
			4,
		},
		{
			1,
			5,
			4,
		},
	}

	getBlockMetaTests = []struct {
		blkHeight      uint64
		numActions     int64
		transferAmount string
	}{
		{
			2,
			7,
			"4",
		},
		{
			4,
			5,
			"0",
		},
	}

	getChainMetaTests = []struct {
		height     uint64
		numActions int64
		tps        int64
		epoch      iotextypes.EpochData
	}{
		{
			4,
			15,
			15,
			iotextypes.EpochData{
				Num:    1,
				Height: 1,
			},
		},
	}

	sendActionTests = []struct {
		actionPb *iotextypes.Action
	}{
		{
			testTransferPb,
		},
		{
			testExecutionPb,
		},
	}

	getReceiptByActionTests = []struct {
		in     string
		status uint64
	}{
		{
			hex.EncodeToString(executionHash2[:]),
			1,
		},
		{
			hex.EncodeToString(executionHash3[:]),
			1,
		},
	}

	readContractTests = []struct {
		execHash string
		retValue string
	}{
		{
			hex.EncodeToString(executionHash2[:]),
			"",
		},
	}

	suggestGasPriceTests = []struct {
		defaultGasPrice   uint64
		suggestedGasPrice uint64
	}{
		{
			1,
			1,
		},
	}

	estimateGasForActionTests = []struct {
		actionHash   string
		estimatedGas uint64
	}{
		{
			hex.EncodeToString(transferHash1[:]),
			10000,
		},
		{
			hex.EncodeToString(voteHash1[:]),
			10000,
		},
	}

	readUnclaimedBalanceTests = []struct {
		// Arguments
		protocolID string
		methodName string
		addr       string
		// Expected values
		returnErr bool
		balance   *big.Int
	}{
		{
			protocolID: rewarding.ProtocolID,
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(0).String(),
			returnErr:  false,
			balance:    unit.ConvertIotxToRau(144), // 4 block * 36 IOTX reward by default = 144 IOTX
		},
		{
			protocolID: rewarding.ProtocolID,
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(1).String(),
			returnErr:  false,
			balance:    unit.ConvertIotxToRau(0), // 4 block * 36 IOTX reward by default = 144 IOTX
		},
		{
			protocolID: "Wrong ID",
			methodName: "UnclaimedBalance",
			addr:       ta.Addrinfo["producer"].String(),
			returnErr:  true,
		},
		{
			protocolID: rewarding.ProtocolID,
			methodName: "Wrong Method",
			addr:       ta.Addrinfo["producer"].String(),
			returnErr:  true,
		},
	}
)

func TestServer_GetAccount(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, true)
	require.NoError(err)

	// success
	for _, test := range getAccountTests {
		request := &iotexapi.GetAccountRequest{Address: test.in}
		res, err := svr.GetAccount(context.Background(), request)
		require.NoError(err)
		accountMeta := res.AccountMeta
		require.Equal(test.address, accountMeta.Address)
		require.Equal(test.balance, accountMeta.Balance)
		require.Equal(test.nonce, accountMeta.Nonce)
		require.Equal(test.pendingNonce, accountMeta.PendingNonce)
	}
	// failure
	_, err = svr.GetAccount(context.Background(), &iotexapi.GetAccountRequest{})
	require.Error(err)
}

func TestServer_GetActions(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getActionsTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByIndex{
				ByIndex: &iotexapi.GetActionsByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(test.numActions, len(res.Actions))
	}
}

func TestServer_GetAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, true)
	require.NoError(err)

	for _, test := range getActionTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByHash{
				ByHash: &iotexapi.GetActionByHashRequest{
					ActionHash:   test.in,
					CheckPending: test.checkPending,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(1, len(res.Actions))
		actPb := res.Actions[0]
		require.Equal(test.nonce, actPb.GetCore().GetNonce())
		require.Equal(test.senderPubKey, hex.EncodeToString(actPb.SenderPubKey))
	}
}

func TestServer_GetActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getActionsByAddressTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByAddr{
				ByAddr: &iotexapi.GetActionsByAddressRequest{
					Address: test.address,
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(test.numActions, len(res.Actions))
	}
}

func TestServer_GetUnconfirmedActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, true)
	require.NoError(err)

	for _, test := range getUnconfirmedActionsByAddressTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_UnconfirmedByAddr{
				UnconfirmedByAddr: &iotexapi.GetUnconfirmedActionsByAddressRequest{
					Address: test.address,
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(test.numActions, len(res.Actions))
	}
}

func TestServer_GetActionsByBlock(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getActionsByBlockTests {
		blk, err := svr.bc.GetBlockByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := blk.HashBlock()
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByBlk{
				ByBlk: &iotexapi.GetActionsByBlockRequest{
					BlkHash: hex.EncodeToString(blkHash[:]),
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(test.numActions, len(res.Actions))
	}
}

func TestServer_GetBlockMetas(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getBlockMetasTests {
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
				ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}
		res, err := svr.GetBlockMetas(context.Background(), request)
		require.NoError(err)
		require.Equal(test.numBlks, len(res.BlkMetas))
		var prevBlkPb *iotextypes.BlockMeta
		for _, blkPb := range res.BlkMetas {
			if prevBlkPb != nil {
				require.True(blkPb.Timestamp < prevBlkPb.Timestamp)
				require.True(blkPb.Height < prevBlkPb.Height)
				prevBlkPb = blkPb
			}
		}
	}
}

func TestServer_GetBlockMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getBlockMetaTests {
		blk, err := svr.bc.GetBlockByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := blk.HashBlock()
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
				ByHash: &iotexapi.GetBlockMetaByHashRequest{
					BlkHash: hex.EncodeToString(blkHash[:]),
				},
			},
		}
		res, err := svr.GetBlockMetas(context.Background(), request)
		require.NoError(err)
		require.Equal(1, len(res.BlkMetas))
		blkPb := res.BlkMetas[0]
		require.Equal(test.numActions, blkPb.NumActions)
		require.Equal(test.transferAmount, blkPb.TransferAmount)
	}
}

func TestServer_GetChainMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getChainMetaTests {
		res, err := svr.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
		require.NoError(err)
		chainMetaPb := res.ChainMeta
		require.Equal(test.height, chainMetaPb.Height)
		require.Equal(test.numActions, chainMetaPb.NumActions)
		require.Equal(test.tps, chainMetaPb.Tps)
		require.Equal(test.epoch.Num, chainMetaPb.Epoch.Num)
		require.Equal(test.epoch.Height, chainMetaPb.Epoch.Height)
	}
}

func TestServer_SendAction(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	broadcastHandlerCount := 0
	svr := Server{bc: chain, dp: mDp, broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
		broadcastHandlerCount++
		return nil
	}}

	chain.EXPECT().ChainID().Return(uint32(1)).Times(4)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)

	for i, test := range sendActionTests {
		request := &iotexapi.SendActionRequest{Action: test.actionPb}
		_, err := svr.SendAction(context.Background(), request)
		require.NoError(err)
		require.Equal(i+1, broadcastHandlerCount)
	}
}

func TestServer_GetReceiptByAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getReceiptByActionTests {
		request := &iotexapi.GetReceiptByActionRequest{ActionHash: test.in}
		res, err := svr.GetReceiptByAction(context.Background(), request)
		require.NoError(err)
		receiptPb := res.Receipt
		require.Equal(test.status, receiptPb.Status)
	}
}

func TestServer_ReadContract(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range readContractTests {
		hash, err := toHash256(test.execHash)
		require.NoError(err)
		exec, err := svr.bc.GetActionByActionHash(hash)
		require.NoError(err)
		request := &iotexapi.ReadContractRequest{Action: exec.Proto()}

		res, err := svr.ReadContract(context.Background(), request)
		require.NoError(err)
		require.Equal(test.retValue, res.Data)
	}
}

func TestServer_SuggestGasPrice(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	for _, test := range suggestGasPriceTests {
		cfg.API.GasStation.DefaultGas = test.defaultGasPrice
		svr, err := createServer(cfg, false)
		require.NoError(err)
		res, err := svr.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
		require.NoError(err)
		require.Equal(test.suggestedGasPrice, res.GasPrice)
	}
}

func TestServer_EstimateGasForAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range estimateGasForActionTests {
		hash, err := toHash256(test.actionHash)
		require.NoError(err)
		act, err := svr.bc.GetActionByActionHash(hash)
		require.NoError(err)
		request := &iotexapi.EstimateGasForActionRequest{Action: act.Proto()}

		res, err := svr.EstimateGasForAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.estimatedGas, res.Gas)
	}
}

func TestServer_ReadUnclaimedBalance(t *testing.T) {
	cfg := newConfig()
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	for _, test := range readUnclaimedBalanceTests {
		out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(test.addr)},
		})
		if test.returnErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		val, ok := big.NewInt(0).SetString(string(out.Data), 10)
		require.True(t, ok)
		assert.Equal(t, test.balance, val)
	}
}

func addProducerToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = accountutil.LoadOrCreateAccount(ws, ta.Addrinfo["producer"].String(),
		blockchain.Gen.TotalSupply); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: ta.Addrinfo["producer"],
			GasLimit: &gasLimit,
		})
	if _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}

func addTestingBlocks(bc blockchain.Blockchain) error {
	addr0 := ta.Addrinfo["producer"].String()
	priKey0 := ta.Keyinfo["producer"].PriKey
	addr1 := ta.Addrinfo["alfa"].String()
	priKey1 := ta.Keyinfo["alfa"].PriKey
	addr2 := ta.Addrinfo["bravo"].String()
	addr3 := ta.Addrinfo["charlie"].String()
	priKey3 := ta.Keyinfo["charlie"].PriKey
	addr4 := ta.Addrinfo["delta"].String()
	// Add block 1
	// Producer transfer--> C
	tsf, err := testutil.SignedTransfer(addr3, priKey0, 1, big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[addr0] = []action.SealedEnvelope{tsf}
	blk, err := bc.MintNewBlock(
		actionMap,
		time.Now().Unix(),
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie transfer--> A, B, D, P
	// Charlie vote--> C
	// Charlie exec--> D
	recipients := []string{addr1, addr2, addr4, addr0}
	selps := make([]action.SealedEnvelope, 0)
	for i, recipient := range recipients {
		selp, err := testutil.SignedTransfer(recipient, priKey3, uint64(i+1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
		if err != nil {
			return err
		}
		selps = append(selps, selp)
	}
	vote1, err := testutil.SignedVote(addr3, priKey3, 5, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	execution1, err := testutil.SignedExecution(addr4, priKey3, 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice), []byte{1})
	if err != nil {
		return err
	}

	selps = append(selps, vote1)
	selps = append(selps, execution1)
	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[addr3] = selps
	if blk, err = bc.MintNewBlock(
		actionMap,
		time.Now().Unix(),
	); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Empty actions
	if blk, err = bc.MintNewBlock(
		nil,
		time.Now().Unix(),
	); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Charlie vote--> C
	// Charlie exec--> D
	// Alfa vote--> A
	// Alfa exec--> D
	vote1, err = testutil.SignedVote(addr3, priKey3, 7, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	vote2, err := testutil.SignedVote(addr1, priKey1, 1, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	execution1, err = testutil.SignedExecution(addr4, priKey3, 8,
		big.NewInt(2), testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice), []byte{1})
	if err != nil {
		return err
	}
	execution2, err := testutil.SignedExecution(addr4, priKey1, 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice), []byte{1})
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[addr3] = []action.SealedEnvelope{vote1, execution1}
	actionMap[addr1] = []action.SealedEnvelope{vote2, execution2}
	if blk, err = bc.MintNewBlock(
		actionMap,
		time.Now().Unix(),
	); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func addActsToActPool(ap actpool.ActPool) error {
	// Producer transfer--> A
	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["alfa"].String(), ta.Keyinfo["producer"].PriKey, 2, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	// Producer vote--> P
	vote1, err := testutil.SignedVote(ta.Addrinfo["producer"].String(), ta.Keyinfo["producer"].PriKey, 3, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	// Producer transfer--> B
	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 4, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	// Producer exec--> D
	execution1, err := testutil.SignedExecution(ta.Addrinfo["delta"].String(), ta.Keyinfo["producer"].PriKey, 5,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	if err != nil {
		return err
	}
	if err := ap.Add(tsf1); err != nil {
		return err
	}
	if err := ap.Add(vote1); err != nil {
		return err
	}
	if err := ap.Add(tsf2); err != nil {
		return err
	}
	return ap.Add(execution1)
}

func setupChain(cfg config.Config) (blockchain.Blockchain, *protocol.Registry, error) {
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(keypair.PrivateKeyToBytes(identityset.PrivateKey(0)))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	if err != nil {
		return nil, nil, err
	}

	// create chain
	registry := protocol.Registry{}
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.PrecreatedStateFactoryOption(sf),
		blockchain.InMemDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	if bc == nil {
		return nil, nil, errors.New("failed to create blockchain")
	}

	acc := account.NewProtocol()
	v := vote.NewProtocol(bc)
	evm := execution.NewProtocol(bc)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	)
	r := rewarding.NewProtocol(bc, rolldposProtocol)
	if err := registry.Register(rolldpos.ProtocolID, rolldposProtocol); err != nil {
		return nil, nil, err
	}
	if err := registry.Register(account.ProtocolID, acc); err != nil {
		return nil, nil, err
	}
	if err := registry.Register(vote.ProtocolID, v); err != nil {
		return nil, nil, err
	}
	if err := registry.Register(execution.ProtocolID, evm); err != nil {
		return nil, nil, err
	}
	if err := registry.Register(rewarding.ProtocolID, r); err != nil {
		return nil, nil, err
	}
	sf.AddActionHandlers(acc, v, evm, r)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	bc.Validator().AddActionValidators(acc, v, evm, r)

	return bc, &registry, nil
}

func setupActPool(bc blockchain.Blockchain, cfg config.ActPool) (actpool.ActPool, error) {
	ap, err := actpool.NewActPool(bc, cfg)
	if err != nil {
		return nil, err
	}
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(vote.NewProtocol(bc), execution.NewProtocol(bc))

	return ap, nil
}

func newConfig() config.Config {
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.EnableIndex = true
	return cfg
}

func createServer(cfg config.Config, needActPool bool) (*Server, error) {
	bc, registry, err := setupChain(cfg)
	if err != nil {
		return nil, err
	}

	// Start blockchain
	ctx := context.Background()
	if err := bc.Start(ctx); err != nil {
		return nil, err
	}

	// Create state for producer
	if err := addProducerToFactory(bc.GetFactory()); err != nil {
		return nil, err
	}

	// Add testing blocks
	if err := addTestingBlocks(bc); err != nil {
		return nil, err
	}

	var ap actpool.ActPool
	if needActPool {
		ap, err = setupActPool(bc, cfg.ActPool)
		if err != nil {
			return nil, err
		}
		// Add actions to actpool
		if err := addActsToActPool(ap); err != nil {
			return nil, err
		}
	}

	apiCfg := config.API{TpsWindow: 10, MaxTransferPayloadBytes: 1024, GasStation: cfg.API.GasStation}

	svr := &Server{
		bc:       bc,
		ap:       ap,
		cfg:      apiCfg,
		gs:       gasstation.NewGasStation(bc, apiCfg),
		registry: registry,
	}

	return svr, nil
}
