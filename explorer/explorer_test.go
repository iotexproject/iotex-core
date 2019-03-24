// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/iotexproject/iotex-core/pkg/unit"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_consensus"
	"github.com/iotexproject/iotex-core/test/mock/mock_dispatcher"
	"github.com/iotexproject/iotex-core/test/mock/mock_factory"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

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
	// test --> A, B, C, D, E, F
	tsf, err := testutil.SignedTransfer(addr3, priKey0, 1, big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}

	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[addr0] = []action.SealedEnvelope{tsf}
	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
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
	// Charlie --> A, B, D, E, test
	tsf1, err := testutil.SignedTransfer(addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err := testutil.SignedTransfer(addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err := testutil.SignedTransfer(addr0, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	vote1, err := testutil.SignedVote(addr3, priKey3, 5, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	execution1, err := testutil.SignedExecution(addr4, priKey3, 6, big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[addr3] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, vote1, execution1}
	if blk, err = bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
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
	if blk, err = bc.MintNewBlock(
		nil,
		testutil.TimestampNow(),
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
	vote1, err = testutil.SignedVote(addr3, priKey3, 7, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	vote2, err := testutil.SignedVote(addr1, priKey1, 1, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	execution1, err = testutil.SignedExecution(addr4, priKey3, 8, big.NewInt(2), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	execution2, err := testutil.SignedExecution(addr4, priKey1, 2, big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[addr3] = []action.SealedEnvelope{vote1, execution1}
	actionMap[addr1] = []action.SealedEnvelope{vote2, execution2}
	if blk, err = bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func addActsToActPool(ap actpool.ActPool) error {
	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["alfa"].String(), ta.Keyinfo["producer"].PriKey, 2, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	vote1, err := testutil.SignedVote(ta.Addrinfo["producer"].String(), ta.Keyinfo["producer"].PriKey, 3, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 4, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	execution1, err := testutil.SignedExecution(ta.Addrinfo["delta"].String(), ta.Keyinfo["producer"].PriKey, 5, big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
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

func TestExplorerApi(t *testing.T) {
	require := require.New(t)
	cfg := config.Default

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.ActPool.MinGasPriceStr = "0"

	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.Nil(err)

	// create chain
	ctx := context.Background()
	registry := protocol.Registry{}
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(registry.Register(rolldpos.ProtocolID, rp))
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.PrecreatedStateFactoryOption(sf),
		blockchain.InMemDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	require.NotNil(bc)
	vp := vote.NewProtocol(bc)
	require.NoError(registry.Register(vote.ProtocolID, vp))
	ap, err := actpool.NewActPool(bc, cfg.ActPool)
	require.Nil(err)
	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil), execution.NewProtocol(bc))
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(vote.NewProtocol(bc),
		execution.NewProtocol(bc))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	bc.Validator().AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))
	require.NoError(bc.Start(ctx))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	require.NoError(addCreatorToFactory(sf))

	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)
	explorerCfg := config.Explorer{TpsWindow: 10, MaxTransferPayloadBytes: 1024}

	svc := Service{
		bc:  bc,
		ap:  ap,
		cfg: explorerCfg,
		gs:  GasStation{bc, explorerCfg},
	}

	blks, getBlkErr := svc.GetLastBlocksByRange(3, 4)
	require.Nil(getBlkErr)
	require.Equal(3, len(blks))

	blk, err := svc.GetBlockByID(blks[0].ID)
	require.Nil(err)
	require.Equal(blks[0].Height, blk.Height)
	require.Equal(blks[0].Timestamp, blk.Timestamp)
	require.Equal(blks[0].Size, blk.Size)
	require.Equal(int64(0), blk.Votes)
	require.Equal(int64(0), blk.Executions)
	require.Equal(int64(0), blk.Transfers)

	_, err = svc.GetBlockByID("")
	require.Error(err)

	stats, err := svc.GetCoinStatistic()
	require.Nil(err)
	require.Equal(int64(4), stats.Height)
	require.Equal(int64(0), stats.Transfers)
	require.Equal(int64(0), stats.Votes)
	require.Equal(int64(0), stats.Executions)
	require.Equal(int64(11), stats.Aps)

	// success
	balance, err := svc.GetAddressBalance(ta.Addrinfo["charlie"].String())
	require.Nil(err)
	require.Equal("3", balance)

	// error
	_, err = svc.GetAddressBalance("")
	require.Error(err)

	// success
	addressDetails, err := svc.GetAddressDetails(ta.Addrinfo["charlie"].String())
	require.Nil(err)
	require.Equal("3", addressDetails.TotalBalance)
	require.Equal(int64(8), addressDetails.Nonce)
	require.Equal(int64(9), addressDetails.PendingNonce)
	require.Equal(ta.Addrinfo["charlie"].String(), addressDetails.Address)

	// error
	_, err = svc.GetAddressDetails("")
	require.Error(err)

	tip, err := svc.GetBlockchainHeight()
	require.Nil(err)
	require.Equal(4, int(tip))

	err = addActsToActPool(ap)
	require.NoError(err)

	// test GetBlockOrActionByHash
	res, err := svc.GetBlockOrActionByHash("")
	require.NoError(err)
	require.Nil(res.Block)
	require.Nil(res.Transfer)
	require.Nil(res.Vote)
	require.Nil(res.Address)
	require.Nil(res.Execution)

	res, err = svc.GetBlockOrActionByHash(blks[0].ID)
	require.NoError(err)
	require.Nil(res.Transfer)
	require.Nil(res.Vote)
	require.Nil(res.Execution)
	require.Nil(res.Address)
	require.Equal(&blks[0], res.Block)

	res, err = svc.GetBlockOrActionByHash(ta.Addrinfo["charlie"].String())
	require.NoError(err)
	addr, err := svc.GetAddressDetails(ta.Addrinfo["charlie"].String())
	require.NoError(err)

	require.Nil(res.Block)
	require.Nil(res.Transfer)
	require.Nil(res.Execution)
	require.Nil(res.Vote)
	require.Equal(&addr, res.Address)

	svc.gs.cfg.GasStation.DefaultGas = 1
	gasPrice, err := svc.SuggestGasPrice()
	require.Nil(err)
	require.Equal(gasPrice, int64(1))
}

func TestService_StateByAddr(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := state.Account{
		Balance:      big.NewInt(46),
		Nonce:        uint64(0),
		IsCandidate:  false,
		VotingWeight: big.NewInt(100),
		Votee:        "456",
	}

	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	mBc.EXPECT().StateByAddr("123").Times(1).Return(&s, nil)

	state, err := mBc.StateByAddr("123")
	require.Nil(err)
	require.Equal(big.NewInt(46), state.Balance)
	require.Equal(uint64(0), state.Nonce)
	require.Equal(false, state.IsCandidate)
	require.Equal(big.NewInt(100), state.VotingWeight)
	require.Equal("456", state.Votee)
}

func TestService_GetConsensusMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := []string{
		"io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh",
		"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
		"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
		"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
		"io1qyqsyqcy8anpz644uhw85rpjplwfv80s687pvhch5ues2k",
		"io1qyqsyqcy65j0upntgz8wq8sum6chetur8ft68uwnfa2m3k",
		"io1qyqsyqcyvx7pmg9pq5kefh5mkxx7fxfmct2x9fpg080r7m",
	}
	c := mock_consensus.NewMockConsensus(ctrl)
	c.EXPECT().Metrics().Return(scheme.ConsensusMetrics{
		LatestEpoch:         1,
		LatestDelegates:     candidates[:4],
		LatestBlockProducer: candidates[3],
		Candidates:          candidates,
	}, nil)

	svc := Service{c: c}

	m, err := svc.GetConsensusMetrics()
	require.Nil(t, err)
	require.NotNil(t, m)
	require.Equal(t, int64(1), m.LatestEpoch)
	require.Equal(
		t,
		[]string{
			"io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh",
			"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
			"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
			"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
		},
		m.LatestDelegates,
	)
	require.Equal(t, "io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc", m.LatestBlockProducer)
	require.Equal(
		t,
		[]string{
			"io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh",
			"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
			"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
			"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
			"io1qyqsyqcy8anpz644uhw85rpjplwfv80s687pvhch5ues2k",
			"io1qyqsyqcy65j0upntgz8wq8sum6chetur8ft68uwnfa2m3k",
			"io1qyqsyqcyvx7pmg9pq5kefh5mkxx7fxfmct2x9fpg080r7m",
		},
		m.Candidates,
	)
}

func TestService_SendTransfer(t *testing.T) {
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

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	r := explorer.SendTransferRequest{
		Version:      0x1,
		Nonce:        1,
		Sender:       ta.Addrinfo["producer"].String(),
		Recipient:    ta.Addrinfo["alfa"].String(),
		Amount:       big.NewInt(1).String(),
		GasPrice:     big.NewInt(0).String(),
		SenderPubKey: ta.Keyinfo["producer"].PubKey.HexString(),
		Signature:    "",
		Payload:      "",
	}
	response, err := svc.SendTransfer(r)
	require.NotNil(response.Hash)
	require.Nil(err)
	gas, err := svc.EstimateGasForTransfer(r)
	require.Nil(err)
	require.Equal(gas, int64(10000))
	assert.Equal(t, 1, broadcastHandlerCount)
}

func TestService_SendVote(t *testing.T) {
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

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	r := explorer.SendVoteRequest{
		Version:     0x1,
		Nonce:       1,
		Voter:       ta.Addrinfo["producer"].String(),
		Votee:       ta.Addrinfo["alfa"].String(),
		VoterPubKey: ta.Keyinfo["producer"].PubKey.HexString(),
		GasPrice:    big.NewInt(0).String(),
		Signature:   "",
	}

	response, err := svc.SendVote(r)
	require.NotNil(response.Hash)
	require.Nil(err)
	gas, err := svc.EstimateGasForVote()
	require.Nil(err)
	require.Equal(gas, int64(10000))
	assert.Equal(t, 1, broadcastHandlerCount)
}

func TestService_SendSmartContract(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	broadcastHandlerCount := 0
	svc := Service{bc: chain, dp: mDp, broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
		broadcastHandlerCount++
		return nil
	}, gs: GasStation{chain, config.Explorer{}}}

	execution, err := testutil.SignedExecution(ta.Addrinfo["delta"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	require.NoError(err)
	explorerExecution, _ := convertExecutionToExplorerExecution(execution, true)
	explorerExecution.Version = int64(execution.Version())

	exe := execution.Action().(*action.Execution)
	explorerExecution.ExecutorPubKey = exe.ExecutorPublicKey().HexString()
	explorerExecution.Signature = hex.EncodeToString(execution.Signature())
	chain.EXPECT().ExecuteContractRead(gomock.Any(), gomock.Any()).Return(&action.Receipt{GasConsumed: 1000}, nil)

	gas, err := svc.EstimateGasForSmartContract(explorerExecution)
	require.Nil(err)
	require.Equal(gas, int64(1000))

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	response, err := svc.SendSmartContract(explorerExecution)
	require.NotNil(response.Hash)
	require.Nil(err)
	assert.Equal(t, 1, broadcastHandlerCount)
}

func TestServicePutSubChainBlock(t *testing.T) {
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

	request := explorer.PutSubChainBlockRequest{}
	response, err := svc.PutSubChainBlock(request)
	require.Equal("", response.Hash)
	require.NotNil(err)

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	roots := []explorer.PutSubChainBlockMerkelRoot{
		{
			Name:  "a",
			Value: hex.EncodeToString([]byte("xddd")),
		},
	}
	r := explorer.PutSubChainBlockRequest{
		Version:       0x1,
		Nonce:         1,
		SenderAddress: ta.Addrinfo["producer"].String(),
		SenderPubKey:  ta.Keyinfo["producer"].PubKey.HexString(),
		GasPrice:      big.NewInt(0).String(),
		Signature:     "",
		Roots:         roots,
	}

	response, err = svc.PutSubChainBlock(r)
	require.NotNil(response.Hash)
	require.Nil(err)
	assert.Equal(t, 1, broadcastHandlerCount)
}

func TestServiceSendAction(t *testing.T) {
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

	request := explorer.SendActionRequest{}
	_, err := svc.SendAction(request)
	require.NotNil(err)

	request.Payload = "abc"
	_, err = svc.SendAction(request)
	require.NotNil(err)

	roots := make(map[string]hash.Hash256)
	roots["10002"] = hash.BytesToHash256([]byte("10002"))
	pb := action.NewPutBlock(
		1,
		ta.Addrinfo["producer"].String(),
		100,
		roots,
		10000,
		big.NewInt(0),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(pb).
		SetGasLimit(10000).SetNonce(1).Build()
	selp, err := action.Sign(elp, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	var marshaler jsonpb.Marshaler
	payload, err := marshaler.MarshalToString(selp.Proto())
	require.NoError(err)
	request.Payload = payload
	require.NoError(err)

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	_, err = svc.SendAction(request)
	require.NoError(err)
	assert.Equal(t, 1, broadcastHandlerCount)
}

func TestServiceGetPeers(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	svc := Service{
		dp: mDp,
		neighborsHandler: func(_ context.Context) ([]peerstore.PeerInfo, error) {
			return []peerstore.PeerInfo{{}, {}, {}}, nil
		},
		networkInfoHandler: func() peerstore.PeerInfo {
			return peerstore.PeerInfo{}
		},
	}

	response, err := svc.GetPeers()
	require.Nil(err)
	require.Equal("{<peer.ID > []}", response.Self.Address)
	require.Len(response.Peers, 3)
}

func TestTransferPayloadBytesLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	svc := Service{cfg: config.Explorer{MaxTransferPayloadBytes: 8}, dp: mDp}
	var payload [9]byte
	req := explorer.SendTransferRequest{
		Payload: hex.EncodeToString(payload[:]),
	}
	res, err := svc.SendTransfer(req)
	assert.Equal(t, explorer.SendTransferResponse{}, res)
	assert.Error(t, err)
	assert.Equal(
		t,
		"transfer payload contains 9 bytes, and is longer than 8 bytes limit: invalid transfer",
		err.Error(),
	)
	assert.Equal(t, ErrTransfer, errors.Cause(err))
}

func TestExplorerCandidateMetrics(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := []string{
		"io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh",
		"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
		"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
		"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
		"io1qyqsyqcy8anpz644uhw85rpjplwfv80s687pvhch5ues2k",
		"io1qyqsyqcy65j0upntgz8wq8sum6chetur8ft68uwnfa2m3k",
		"io1qyqsyqcyvx7pmg9pq5kefh5mkxx7fxfmct2x9fpg080r7m",
	}
	c := mock_consensus.NewMockConsensus(ctrl)
	c.EXPECT().Metrics().Return(scheme.ConsensusMetrics{
		LatestEpoch:         1,
		LatestDelegates:     candidates[:4],
		LatestBlockProducer: candidates[3],
		Candidates:          candidates,
	}, nil)
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
		{Address: candidates[0], Votes: big.NewInt(0)},
		{Address: candidates[1], Votes: big.NewInt(0)},
		{Address: candidates[2], Votes: big.NewInt(0)},
		{Address: candidates[3], Votes: big.NewInt(0)},
		{Address: candidates[4], Votes: big.NewInt(0)},
		{Address: candidates[5], Votes: big.NewInt(0)},
		{Address: candidates[6], Votes: big.NewInt(0)},
	}, nil)

	svc := Service{c: c, bc: bc}

	metrics, err := svc.GetCandidateMetrics()
	require.NoError(err)
	require.True(7 == len(metrics.Candidates))
	require.True(0 == metrics.LatestHeight)
	require.True(1 == metrics.LatestEpoch)
}

func TestExplorerGetReceiptByExecutionID(t *testing.T) {
	require := require.New(t)

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.EnableAsyncIndexWrite = false

	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.Nil(err)

	// create chain
	ctx := context.Background()
	registry := protocol.Registry{}
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(registry.Register(rolldpos.ProtocolID, rp))
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.PrecreatedStateFactoryOption(sf),
		blockchain.InMemDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	vp := vote.NewProtocol(bc)
	require.NoError(registry.Register(vote.ProtocolID, vp))
	require.NoError(bc.Start(ctx))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	sf.AddActionHandlers(execution.NewProtocol(bc))
	require.NoError(addCreatorToFactory(sf))

	svc := Service{
		bc: bc,
		cfg: config.Explorer{
			TpsWindow:               10,
			MaxTransferPayloadBytes: 1024,
		},
	}

	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	// data, _ := hex.DecodeString("6060604052600436106100565763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166341c0e1b581146100585780637bf786f81461006b578063fbf788d61461009c575b005b341561006357600080fd5b6100566100ca565b341561007657600080fd5b61008a600160a060020a03600435166100f1565b60405190815260200160405180910390f35b34156100a757600080fd5b610056600160a060020a036004351660243560ff60443516606435608435610103565b60005433600160a060020a03908116911614156100ef57600054600160a060020a0316ff5b565b60016020526000908152604090205481565b600160a060020a0385166000908152600160205260408120548190861161012957600080fd5b3087876040516c01000000000000000000000000600160a060020a03948516810282529290931690910260148301526028820152604801604051809103902091506001828686866040516000815260200160405260006040516020015260405193845260ff90921660208085019190915260408085019290925260608401929092526080909201915160208103908084039060008661646e5a03f115156101cf57600080fd5b505060206040510351600054600160a060020a039081169116146101f257600080fd5b50600160a060020a03808716600090815260016020526040902054860390301631811161026257600160a060020a0387166000818152600160205260409081902088905582156108fc0290839051600060405180830381858888f19350505050151561025d57600080fd5b6102b7565b6000547f2250e2993c15843b32621c89447cc589ee7a9f049c026986e545d3c2c0c6f97890600160a060020a0316604051600160a060020a03909116815260200160405180910390a186600160a060020a0316ff5b505050505050505600a165627a7a72305820533e856fc37e3d64d1706bcc7dfb6b1d490c8d566ea498d9d01ec08965a896ca0029")

	execution, err := testutil.SignedExecution(action.EmptyAddress, ta.Keyinfo["producer"].PriKey, 1, big.NewInt(0), 1000000, big.NewInt(testutil.TestGasPriceInt64), data)
	require.NoError(err)

	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[ta.Addrinfo["producer"].String()] = []action.SealedEnvelope{execution}
	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)

	require.NoError(err)
	require.Nil(bc.CommitBlock(blk))

	eHash := execution.Hash()
	eHashStr := hex.EncodeToString(eHash[:])
	receipt, err := svc.GetReceiptByExecutionID(eHashStr)
	require.NoError(err)
	require.Equal(eHashStr, receipt.Hash)
}

func TestService_CreateDeposit(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().ChainID().Return(uint32(1)).Times(2)
	dp := mock_dispatcher.NewMockDispatcher(ctrl)
	dp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	broadcastHandlerCount := 0
	svc := Service{
		cfg: cfg.Explorer,
		bc:  bc,
		broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
			broadcastHandlerCount++
			return nil
		},
		dp: dp,
	}

	deposit := action.NewCreateDeposit(
		10,
		2,
		big.NewInt(10000),
		// Test explorer only, so that it doesn't matter the address is not on sub-chain
		ta.Addrinfo["alfa"].String(),
		1000,
		big.NewInt(100),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(deposit).
		SetGasLimit(1000).
		SetGasPrice(big.NewInt(100)).
		SetNonce(10).Build()
	selp, err := action.Sign(elp, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	res, error := svc.CreateDeposit(explorer.CreateDepositRequest{
		Version:      int64(deposit.Version()),
		Nonce:        int64(deposit.Nonce()),
		ChainID:      int64(deposit.ChainID()),
		SenderPubKey: deposit.SenderPublicKey().HexString(),
		Recipient:    deposit.Recipient(),
		Amount:       deposit.Amount().String(),
		Signature:    hex.EncodeToString(selp.Signature()),
		GasLimit:     int64(deposit.GasLimit()),
		GasPrice:     deposit.GasPrice().String(),
	})
	require.NoError(error)
	hash := deposit.Hash()
	require.Equal(hex.EncodeToString(hash[:]), res.Hash)
	assert.Equal(t, 1, broadcastHandlerCount)
}

func TestService_SettleDeposit(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().ChainID().Return(uint32(1)).Times(2)
	dp := mock_dispatcher.NewMockDispatcher(ctrl)
	dp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	broadcastHandlerCount := 0
	svc := Service{
		cfg: cfg.Explorer,
		bc:  bc,
		broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
			broadcastHandlerCount++
			return nil
		},
		dp: dp,
	}

	deposit := action.NewSettleDeposit(
		10,
		big.NewInt(10000),
		100000,
		// Test explorer only, so that it doesn't matter the address is not on sub-chain
		ta.Addrinfo["alfa"].String(),
		1000,
		big.NewInt(100),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(deposit).
		SetGasLimit(1000).
		SetGasPrice(big.NewInt(100)).
		SetNonce(10).Build()
	selp, err := action.Sign(elp, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	res, error := svc.SettleDeposit(explorer.SettleDepositRequest{
		Version:      int64(deposit.Version()),
		Nonce:        int64(deposit.Nonce()),
		SenderPubKey: deposit.SenderPublicKey().HexString(),
		Recipient:    deposit.Recipient(),
		Amount:       deposit.Amount().String(),
		Index:        int64(deposit.Index()),
		Signature:    hex.EncodeToString(selp.Signature()),
		GasLimit:     int64(deposit.GasLimit()),
		GasPrice:     deposit.GasPrice().String(),
	})
	require.NoError(error)
	hash := deposit.Hash()
	require.Equal(hex.EncodeToString(hash[:]), res.Hash)
	assert.Equal(t, 1, broadcastHandlerCount)
}

func TestService_GetDeposits(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	ctrl := gomock.NewController(t)
	cfg := config.Default
	ctx := context.Background()
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	bc.EXPECT().GetFactory().Return(sf).AnyTimes()
	subChainAddr := ta.Addrinfo["producer"]
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	require.NoError(ws.PutState(
		mainchain.SubChainsInOperationKey,
		mainchain.SubChainsInOperation{
			mainchain.InOperation{
				ID:   2,
				Addr: subChainAddr.Bytes(),
			},
		},
	))
	require.NoError(ws.PutState(
		hash.BytesToHash160(subChainAddr.Bytes()),
		&mainchain.SubChain{
			DepositCount:   2,
			OwnerPublicKey: ta.Keyinfo["producer"].PubKey,
		},
	))
	depositAddr1 := ta.Addrinfo["alfa"]
	require.NoError(ws.PutState(
		mainchain.DepositAddress(subChainAddr.Bytes(), 0),
		&mainchain.Deposit{
			Amount:    big.NewInt(100),
			Addr:      depositAddr1.Bytes(),
			Confirmed: false,
		},
	))
	depositAddr2 := ta.Addrinfo["bravo"]
	require.NoError(ws.PutState(
		mainchain.DepositAddress(subChainAddr.Bytes(), 1),
		&mainchain.Deposit{
			Amount:    big.NewInt(200),
			Addr:      depositAddr2.Bytes(),
			Confirmed: false,
		},
	))
	require.NoError(sf.Commit(ws))

	defer func() {
		require.NoError(sf.Stop(ctx))
		ctrl.Finish()
	}()

	p := mainchain.NewProtocol(bc)
	svc := Service{
		mainChain: p,
	}

	_, err = svc.GetDeposits(3, 0, 1)
	assert.True(strings.Contains(err.Error(), "is not found in operation"))

	deposits, err := svc.GetDeposits(2, 0, 1)
	assert.NoError(err)
	assert.Equal(1, len(deposits))
	assert.Equal("100", deposits[0].Amount)

	deposits, err = svc.GetDeposits(2, 1, 2)
	assert.NoError(err)
	assert.Equal(2, len(deposits))
	assert.Equal("200", deposits[0].Amount)
	assert.Equal("100", deposits[1].Amount)

	deposits, err = svc.GetDeposits(2, 1, 3)
	assert.NoError(err)
	assert.Equal(2, len(deposits))

	deposits, err = svc.GetDeposits(2, 3, 2)
	assert.NoError(err)
	assert.Equal(2, len(deposits))

	deposits, err = svc.GetDeposits(2, 0, 2)
	assert.NoError(err)
	assert.Equal(1, len(deposits))
}

func TestService_GetStateRootHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	sf := mock_factory.NewMockFactory(ctrl)
	bc.EXPECT().GetFactory().Return(sf).AnyTimes()
	rootHash := hash.Hash256b([]byte("test"))
	sf.EXPECT().RootHashByHeight(gomock.Any()).Return(rootHash, nil).Times(1)

	defer ctrl.Finish()

	svc := Service{bc: bc}
	rootHashStr, err := svc.GetStateRootHash(1)
	assert.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(rootHash[:]), rootHashStr)
}

func addCreatorToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = accountutil.LoadOrCreateAccount(
		ws,
		ta.Addrinfo["producer"].String(),
		unit.ConvertIotxToRau(10000000000),
	); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: ta.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	if _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}
