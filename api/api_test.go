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
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	iproto "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state/factory"
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
	marshaler jsonpb.Marshaler

	testTransfer, _ = testutil.SignedTransfer(ta.Addrinfo["alfa"].String(),
		ta.Keyinfo["alfa"].PriKey, 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPrice))
	testTransferHash = testTransfer.Hash()

	testTransferStr, _ = marshaler.MarshalToString(testTransfer.Proto())

	testExecution, _ = testutil.SignedExecution(ta.Addrinfo["bravo"].String(),
		ta.Keyinfo["bravo"].PriKey, 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPrice), []byte{})
	testExecutionHash = testExecution.Hash()

	testExecutionStr, _ = marshaler.MarshalToString(testExecution.Proto())
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
			uint64(8),
			uint64(9),
		},
		{
			ta.Addrinfo["producer"].String(),
			"io14485vn8markfupgy86at5a0re78jll0pmq8fjv",
			"9999999999999999999999999991",
			uint64(1),
			uint64(6),
		},
	}

	getActionsTests = []struct {
		start      int64
		count      int64
		numActions int
	}{
		{
			int64(0),
			int64(11),
			11,
		},
		{
			int64(11),
			int64(21),
			21,
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
			"be60e55d583093162cfafa1967bed3454942faae83b956cb60f9936b508bdea3",
			uint64(1),
			"04755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf33",
		},
		{
			false,
			"786177f72c5a0196487b34343daaa1852afd16eece9222cee22df615b7594ef1",
			uint64(5),
			"043489f788326de4844ebc745f782bc4fc4c67d420cd9913282598999af32f832a3dd980f2b6667cbcb7844d7f557933e917858d4e6a6f58b870bebd2a6c496bd6",
		},
		{
			true,
			"b3583c3bfe08d96dec46401397c0aa648135b2892dca24bb8b9cf05fb52773eb",
			uint64(5),
			"04755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf33",
		},
	}

	getActionsByAddressTests = []struct {
		address    string
		start      int64
		count      int64
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
		start      int64
		count      int64
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
		start      int64
		count      int64
		numActions int
	}{
		{
			uint64(2),
			0,
			7,
			7,
		},
		{
			uint64(4),
			0,
			5,
			5,
		},
	}

	getBlockMetasTests = []struct {
		start   int64
		count   int64
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
			uint64(2),
			7,
			"4",
		},
		{
			uint64(4),
			5,
			"0",
		},
	}

	getChainMetaTests = []struct {
		height     uint64
		numActions int64
		tps        int64
	}{
		{
			4,
			36,
			36,
		},
	}

	sendActionTests = []struct {
		action string
		hash   string
	}{
		{
			testTransferStr,
			hex.EncodeToString(testTransferHash[:]),
		},
		{
			testExecutionStr,
			hex.EncodeToString(testExecutionHash[:]),
		},
	}

	getReceiptByActionTests = []struct {
		in     string
		status uint64
	}{
		{
			"3d7f6918447cda452bc8d32ca70f4aa06618aed530dafc9d547bd83454d1ffd2",
			uint64(1),
		},
		{
			"80c933e709b5534f1e3324ae7ce35f13dd5f0850ffdeaf01209674e1ab353ad8",
			uint64(1),
		},
	}

	readContractTests = []struct {
		execHash string
		retValue string
	}{
		{
			"3d7f6918447cda452bc8d32ca70f4aa06618aed530dafc9d547bd83454d1ffd2",
			"",
		},
	}

	suggestGasPriceTests = []struct {
		defaultGasPrice   int
		suggestedGasPrice int64
	}{
		{
			1,
			1,
		},
	}

	estimateGasForActionTests = []struct {
		actionHash   string
		estimatedGas int64
	}{
		{
			"be60e55d583093162cfafa1967bed3454942faae83b956cb60f9936b508bdea3",
			10000,
		},
		{
			"786177f72c5a0196487b34343daaa1852afd16eece9222cee22df615b7594ef1",
			10000,
		},
	}
)

func TestService_GetAccount(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, true)
	require.NoError(err)

	// success
	for _, test := range getAccountTests {
		res, err := svc.GetAccount(test.in)
		require.NoError(err)
		var accountMeta iproto.AccountMeta
		require.NoError(jsonpb.UnmarshalString(res, &accountMeta))
		require.Equal(test.address, accountMeta.Address)
		require.Equal(test.balance, accountMeta.Balance)
		require.Equal(test.nonce, accountMeta.Nonce)
		require.Equal(test.pendingNonce, accountMeta.PendingNonce)
	}
	// failure
	_, err = svc.GetAccount("")
	require.Error(err)
}

func TestService_GetActions(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range getActionsTests {
		res, err := svc.GetActions(test.start, test.count)
		require.NoError(err)
		require.Equal(test.numActions, len(res))
	}
}

func TestService_GetAction(t *testing.T) {
	// TODO this test hard coded action hash skip for now
	t.Skip()
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, true)
	require.NoError(err)

	for _, test := range getActionTests {
		res, err := svc.GetAction(test.in, test.checkPending)
		require.NoError(err)
		var actPb iproto.ActionPb
		require.NoError(jsonpb.UnmarshalString(res, &actPb))
		require.Equal(test.nonce, actPb.Nonce)
		require.Equal(test.senderPubKey, hex.EncodeToString(actPb.SenderPubKey))
	}
}

func TestService_GetActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range getActionsByAddressTests {
		res, err := svc.GetActionsByAddress(test.address, test.start, test.count)
		require.NoError(err)
		require.Equal(test.numActions, len(res))
	}
}

func TestService_GetUnconfirmedActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, true)
	require.NoError(err)

	for _, test := range getUnconfirmedActionsByAddressTests {
		res, err := svc.GetUnconfirmedActionsByAddress(test.address, test.start, test.count)
		require.NoError(err)
		require.Equal(test.numActions, len(res))
	}
}

func TestService_GetActionsByBlock(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range getActionsByBlockTests {
		blk, err := svc.bc.GetBlockByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := blk.HashBlock()
		res, err := svc.GetActionsByBlock(hex.EncodeToString(blkHash[:]), test.start, test.count)
		require.NoError(err)
		require.Equal(test.numActions, len(res))
	}
}

func TestService_GetBlockMetas(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range getBlockMetasTests {
		res, err := svc.GetBlockMetas(test.start, test.count)
		require.NoError(err)
		require.Equal(test.numBlks, len(res))
		var prevBlkPb *iproto.BlockMeta
		for _, blkStr := range res {
			var blkPb iproto.BlockMeta
			require.NoError(jsonpb.UnmarshalString(blkStr, &blkPb))
			if prevBlkPb != nil {
				require.True(blkPb.Timestamp < prevBlkPb.Timestamp)
				require.True(blkPb.Height < prevBlkPb.Height)
				prevBlkPb = &blkPb
			}
		}
	}
}

func TestService_GetBlockMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range getBlockMetaTests {
		blk, err := svc.bc.GetBlockByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := blk.HashBlock()
		res, err := svc.GetBlockMeta(hex.EncodeToString(blkHash[:]))
		require.NoError(err)
		var blkPb iproto.BlockMeta
		require.NoError(jsonpb.UnmarshalString(res, &blkPb))
		require.Equal(test.numActions, blkPb.NumActions)
		require.Equal(test.transferAmount, blkPb.TransferAmount)
	}
}

func TestService_GetChainMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range getChainMetaTests {
		res, err := svc.GetChainMeta()
		require.NoError(err)
		var chainMetaPb iproto.ChainMeta
		require.NoError(jsonpb.UnmarshalString(res, &chainMetaPb))
		require.Equal(test.height, chainMetaPb.Height)
		require.Equal(test.numActions, chainMetaPb.NumActions)
		require.Equal(test.tps, chainMetaPb.Tps)
	}
}

func TestService_SendAction(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	broadcastHandlerCount := 0
	svc := Service{bc: chain, dp: mDp, broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
		broadcastHandlerCount++
		return nil
	}}

	chain.EXPECT().ChainID().Return(uint32(1)).Times(4)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)

	for i, test := range sendActionTests {
		res, err := svc.SendAction(test.action)
		require.NoError(err)
		require.Equal(test.hash, res)
		require.Equal(i+1, broadcastHandlerCount)
	}
}

func TestService_GetReceiptByAction(t *testing.T) {
	// TODO this test hard coded action hash skip for now
	t.Skip()
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range getReceiptByActionTests {
		res, err := svc.GetReceiptByAction(test.in)
		require.NoError(err)
		var receiptPb iproto.ReceiptPb
		require.NoError(jsonpb.UnmarshalString(res, &receiptPb))
		require.Equal(test.status, receiptPb.Status)
	}
}

func TestService_ReadContract(t *testing.T) {
	// TODO this test hard coded action hash skip for now
	t.Skip()
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range readContractTests {
		hash, err := toHash256(test.execHash)
		require.NoError(err)
		exec, err := svc.bc.GetActionByActionHash(hash)
		require.NoError(err)
		execStr, err := marshaler.MarshalToString(exec.Proto())
		require.NoError(err)

		res, err := svc.ReadContract(execStr)
		require.NoError(err)
		require.Equal(test.retValue, res)
	}
}

func TestService_SuggestGasPrice(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range suggestGasPriceTests {
		svc.gs.cfg.GasStation.DefaultGas = test.defaultGasPrice
		res, err := svc.SuggestGasPrice()
		require.NoError(err)
		require.Equal(test.suggestedGasPrice, res)
	}
}

func TestService_EstimateGasForAction(t *testing.T) {
	// TODO this test hard coded action hash skip for now
	t.Skip()
	require := require.New(t)
	cfg := newConfig()

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	svc, err := createService(cfg, false)
	require.NoError(err)

	for _, test := range estimateGasForActionTests {
		hash, err := toHash256(test.actionHash)
		require.NoError(err)
		act, err := svc.bc.GetActionByActionHash(hash)
		require.NoError(err)
		actStr, err := marshaler.MarshalToString(act.Proto())
		require.NoError(err)

		res, err := svc.EstimateGasForAction(actStr)
		require.NoError(err)
		require.Equal(test.estimatedGas, res)
	}
}

func addProducerToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = account.LoadOrCreateAccount(ws, ta.Addrinfo["producer"].String(),
		blockchain.Gen.TotalSupply); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:        ta.Addrinfo["producer"],
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	if _, _, err = ws.RunActions(ctx, 0, nil); err != nil {
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
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
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
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
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
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
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
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].String(),
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

func setupChain(cfg config.Config) (blockchain.Blockchain, error) {
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	if err != nil {
		return nil, err
	}

	// create chain
	genesisConfig := genesis.Default
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.PrecreatedStateFactoryOption(sf),
		blockchain.InMemDaoOption(),
		blockchain.GenesisOption(genesisConfig),
	)
	if bc == nil {
		return nil, errors.New("failed to create blockchain")
	}
	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil), execution.NewProtocol(bc))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesisConfig.Blockchain.ActionGasLimit))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))

	return bc, nil
}

func setupActPool(bc blockchain.Blockchain, cfg config.ActPool) (actpool.ActPool, error) {
	ap, err := actpool.NewActPool(bc, cfg)
	if err != nil {
		return nil, err
	}
	genesisConfig := genesis.Default
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesisConfig.Blockchain.ActionGasLimit))
	ap.AddActionValidators(vote.NewProtocol(bc),
		execution.NewProtocol(bc))

	return ap, nil
}

func newConfig() config.Config {
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.EnableIndex = true
	return cfg
}

func createService(cfg config.Config, needActPool bool) (*Service, error) {
	bc, err := setupChain(cfg)
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

	apiCfg := config.API{TpsWindow: 10, MaxTransferPayloadBytes: 1024}

	svc := &Service{
		bc:  bc,
		ap:  ap,
		cfg: apiCfg,
		gs:  GasStation{bc, apiCfg},
	}

	return svc, nil
}
