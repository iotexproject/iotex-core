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
	"math/big"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"strings"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocols/multichain/mainchain"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_consensus"
	"github.com/iotexproject/iotex-core/test/mock/mock_dispatcher"
	"github.com/iotexproject/iotex-core/test/mock/mock_network"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	testTriePath = "trie.test"
	testDBPath   = "db.test"
)

func addTestingBlocks(bc blockchain.Blockchain) error {
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf, _ := action.NewTransfer(1, big.NewInt(10), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err := action.Sign(tsf, ta.Addrinfo["producer"].PrivateKey); err != nil {
		return err
	}
	blk, err := bc.MintNewBlock([]action.Action{tsf}, ta.Addrinfo["producer"],
		nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie --> A, B, D, E, test
	tsf1, _ := action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf1, ta.Addrinfo["charlie"].PrivateKey)
	tsf2, _ := action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf2, ta.Addrinfo["charlie"].PrivateKey)
	tsf3, _ := action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf3, ta.Addrinfo["charlie"].PrivateKey)
	tsf4, _ := action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf4, ta.Addrinfo["charlie"].PrivateKey)
	vote1, _ := action.NewVote(5, ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(vote1, ta.Addrinfo["charlie"].PrivateKey)
	execution1, _ := action.NewExecution(ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress, 6, big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	_ = action.Sign(execution1, ta.Addrinfo["charlie"].PrivateKey)
	if blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4, vote1, execution1}, ta.Addrinfo["producer"],
		nil, nil, ""); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	if blk, err = bc.MintNewBlock(nil, ta.Addrinfo["producer"], nil,
		nil, ""); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	vote1, _ = action.NewVote(7, ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	vote2, _ := action.NewVote(1, ta.Addrinfo["alfa"].RawAddress, ta.Addrinfo["charlie"].RawAddress, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(vote1, ta.Addrinfo["charlie"].PrivateKey)
	_ = action.Sign(vote2, ta.Addrinfo["alfa"].PrivateKey)
	execution1, _ = action.NewExecution(ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress, 8, big.NewInt(2), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	execution2, _ := action.NewExecution(ta.Addrinfo["alfa"].RawAddress, ta.Addrinfo["delta"].RawAddress, 2, big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	_ = action.Sign(execution1, ta.Addrinfo["charlie"].PrivateKey)
	_ = action.Sign(execution2, ta.Addrinfo["alfa"].PrivateKey)
	if blk, err = bc.MintNewBlock([]action.Action{vote1, vote2, execution1, execution2}, ta.Addrinfo["producer"],
		nil, nil, ""); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func addActsToActPool(ap actpool.ActPool) error {
	tsf1, _ := action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf1, ta.Addrinfo["producer"].PrivateKey)
	vote1, _ := action.NewVote(3, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["producer"].RawAddress, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(vote1, ta.Addrinfo["producer"].PrivateKey)
	tsf2, _ := action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	_ = action.Sign(tsf2, ta.Addrinfo["producer"].PrivateKey)
	execution1, _ := action.NewExecution(ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["delta"].RawAddress, 5, big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	_ = action.Sign(execution1, ta.Addrinfo["producer"].PrivateKey)
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
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))
	// Disable block reward to make bookkeeping easier
	blockchain.Gen.BlockReward = big.NewInt(0)

	// create chain
	ctx := context.Background()
	bc := blockchain.NewBlockchain(cfg, blockchain.PrecreatedStateFactoryOption(sf), blockchain.InMemDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	ap, err := actpool.NewActPool(bc, cfg.ActPool)
	require.Nil(err)
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

	transfers, err := svc.GetTransfersByAddress(ta.Addrinfo["charlie"].RawAddress, 0, 10)
	require.Nil(err)
	require.Equal(5, len(transfers))

	votes, err := svc.GetVotesByAddress(ta.Addrinfo["charlie"].RawAddress, 0, 10)
	require.Nil(err)
	require.Equal(3, len(votes))

	votes, err = svc.GetVotesByAddress(ta.Addrinfo["charlie"].RawAddress, 0, 2)
	require.Nil(err)
	require.Equal(2, len(votes))

	votes, err = svc.GetVotesByAddress(ta.Addrinfo["alfa"].RawAddress, 0, 10)
	require.Nil(err)
	require.Equal(2, len(votes))

	votes, err = svc.GetVotesByAddress(ta.Addrinfo["delta"].RawAddress, 0, 10)
	require.Nil(err)
	require.Equal(1, len(votes))

	executions, err := svc.GetExecutionsByAddress(ta.Addrinfo["charlie"].RawAddress, 0, 10)
	require.Nil(err)
	require.Equal(2, len(executions))

	executions, err = svc.GetExecutionsByAddress(ta.Addrinfo["alfa"].RawAddress, 0, 10)
	require.Nil(err)
	require.Equal(1, len(executions))

	transfers, err = svc.GetLastTransfersByRange(4, 1, 3, true)
	require.Equal(3, len(transfers))
	require.Nil(err)
	transfers, err = svc.GetLastTransfersByRange(4, 4, 5, true)
	require.Equal(5, len(transfers))
	require.Nil(err)

	transfers, err = svc.GetLastTransfersByRange(4, 1, 3, false)
	require.Equal(3, len(transfers))
	require.Nil(err)
	transfers, err = svc.GetLastTransfersByRange(4, 4, 5, false)
	require.Equal(1, len(transfers))
	require.Nil(err)

	votes, err = svc.GetLastVotesByRange(4, 0, 10)
	require.Equal(10, len(votes))
	require.Nil(err)
	votes, err = svc.GetLastVotesByRange(3, 0, 50)
	require.Equal(22, len(votes))
	require.Nil(err)

	executions, err = svc.GetLastExecutionsByRange(4, 0, 3)
	require.Equal(3, len(executions))
	require.Nil(err)
	executions, err = svc.GetLastExecutionsByRange(3, 0, 50)
	require.Equal(1, len(executions))
	require.Nil(err)

	blks, getBlkErr := svc.GetLastBlocksByRange(3, 4)
	require.Nil(getBlkErr)
	require.Equal(4, len(blks))

	transfers, err = svc.GetTransfersByBlockID(blks[2].ID, 0, 10)
	require.Nil(err)
	require.Equal(2, len(transfers))

	// fail
	_, err = svc.GetTransfersByBlockID("", 0, 10)
	require.Error(err)

	votes, err = svc.GetVotesByBlockID(blks[1].ID, 0, 0)
	require.Nil(err)
	require.Equal(0, len(votes))

	votes, err = svc.GetVotesByBlockID(blks[1].ID, 0, 10)
	require.Nil(err)
	require.Equal(1, len(votes))

	// fail
	_, err = svc.GetVotesByBlockID("", 0, 10)
	require.Error(err)

	// fail
	_, err = svc.GetExecutionsByBlockID("", 0, 10)
	require.Error(err)

	executions, err = svc.GetExecutionsByBlockID(blks[1].ID, 0, 10)
	require.Nil(err)
	require.Equal(1, len(executions))

	transfer, err := svc.GetTransferByID(transfers[0].ID)
	require.Nil(err)
	require.Equal(transfers[0].Sender, transfer.Sender)
	require.Equal(transfers[0].Recipient, transfer.Recipient)
	require.Equal(transfers[0].BlockID, transfer.BlockID)

	// error
	_, err = svc.GetTransferByID("")
	require.Error(err)

	vote, err := svc.GetVoteByID(votes[0].ID)
	require.Nil(err)
	require.Equal(votes[0].Nonce, vote.Nonce)
	require.Equal(votes[0].BlockID, vote.BlockID)
	require.Equal(votes[0].Timestamp, vote.Timestamp)
	require.Equal(votes[0].ID, vote.ID)
	require.Equal(votes[0].Votee, vote.Votee)
	require.Equal(votes[0].Voter, vote.Voter)

	// fail
	_, err = svc.GetVoteByID("")
	require.Error(err)

	execution, err := svc.GetExecutionByID(executions[0].ID)
	require.Nil(err)
	require.Equal(executions[0].Nonce, execution.Nonce)
	require.Equal(executions[0].BlockID, execution.BlockID)
	require.Equal(executions[0].Timestamp, execution.Timestamp)
	require.Equal(executions[0].ID, execution.ID)
	require.Equal(executions[0].Executor, execution.Executor)
	require.Equal(executions[0].Contract, execution.Contract)

	// fail
	_, err = svc.GetExecutionByID("")
	require.Error(err)

	blk, err := svc.GetBlockByID(blks[0].ID)
	require.Nil(err)
	require.Equal(blks[0].Height, blk.Height)
	require.Equal(blks[0].Timestamp, blk.Timestamp)
	require.Equal(blks[0].Size, blk.Size)
	require.Equal(int64(0), blk.Votes)
	require.Equal(int64(0), blk.Executions)
	require.Equal(int64(1), blk.Transfers)

	_, err = svc.GetBlockByID("")
	require.Error(err)

	stats, err := svc.GetCoinStatistic()
	require.Nil(err)
	require.Equal(blockchain.Gen.TotalSupply.String(), stats.Supply)
	require.Equal(int64(4), stats.Height)
	require.Equal(int64(9), stats.Transfers)
	require.Equal(int64(24), stats.Votes)
	require.Equal(int64(3), stats.Executions)
	require.Equal(int64(15), stats.Aps)

	// success
	balance, err := svc.GetAddressBalance(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal("6", balance)

	// error
	_, err = svc.GetAddressBalance("")
	require.Error(err)

	// success
	addressDetails, err := svc.GetAddressDetails(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal("6", addressDetails.TotalBalance)
	require.Equal(int64(8), addressDetails.Nonce)
	require.Equal(int64(9), addressDetails.PendingNonce)
	require.Equal(ta.Addrinfo["charlie"].RawAddress, addressDetails.Address)

	// error
	_, err = svc.GetAddressDetails("")
	require.Error(err)

	tip, err := svc.GetBlockchainHeight()
	require.Nil(err)
	require.Equal(4, int(tip))

	err = addActsToActPool(ap)
	require.Nil(err)

	// success
	transfers, err = svc.GetUnconfirmedTransfersByAddress(ta.Addrinfo["producer"].RawAddress, 0, 3)
	require.Nil(err)
	require.Equal(2, len(transfers))
	require.Equal(int64(2), transfers[0].Nonce)
	require.Equal(int64(4), transfers[1].Nonce)
	votes, err = svc.GetUnconfirmedVotesByAddress(ta.Addrinfo["producer"].RawAddress, 0, 3)
	require.Nil(err)
	require.Equal(1, len(votes))
	require.Equal(int64(3), votes[0].Nonce)
	executions, err = svc.GetUnconfirmedExecutionsByAddress(ta.Addrinfo["producer"].RawAddress, 0, 3)
	require.Nil(err)
	require.Equal(1, len(executions))
	require.Equal(int64(5), executions[0].Nonce)
	transfers, err = svc.GetUnconfirmedTransfersByAddress(ta.Addrinfo["producer"].RawAddress, 1, 1)
	require.Nil(err)
	require.Equal(1, len(transfers))
	require.Equal(int64(4), transfers[0].Nonce)
	votes, err = svc.GetUnconfirmedVotesByAddress(ta.Addrinfo["producer"].RawAddress, 1, 1)
	require.Nil(err)
	require.Equal(0, len(votes))
	executions, err = svc.GetUnconfirmedExecutionsByAddress(ta.Addrinfo["producer"].RawAddress, 1, 1)
	require.Nil(err)
	require.Equal(0, len(executions))

	// error
	_, err = svc.GetUnconfirmedTransfersByAddress("", 0, 3)
	require.Error(err)
	_, err = svc.GetUnconfirmedVotesByAddress("", 0, 3)
	require.Error(err)
	_, err = svc.GetUnconfirmedExecutionsByAddress("", 0, 3)
	require.Error(err)

	// test GetBlockOrActionByHash
	res, err := svc.GetBlockOrActionByHash("")
	require.NoError(err)
	require.Nil(res.Block)
	require.Nil(res.Transfer)
	require.Nil(res.Vote)
	require.Nil(res.Execution)

	res, err = svc.GetBlockOrActionByHash(blks[0].ID)
	require.NoError(err)
	require.Nil(res.Transfer)
	require.Nil(res.Vote)
	require.Nil(res.Execution)
	require.Equal(&blks[0], res.Block)

	res, err = svc.GetBlockOrActionByHash(transfers[0].ID)
	require.NoError(err)
	require.Nil(res.Block)
	require.Nil(res.Vote)
	require.Nil(res.Execution)
	require.Equal(&transfers[0], res.Transfer)

	votes, err = svc.GetLastVotesByRange(3, 0, 50)
	require.NoError(err)
	res, err = svc.GetBlockOrActionByHash(votes[0].ID)
	require.NoError(err)
	require.Nil(res.Block)
	require.Nil(res.Transfer)
	require.Nil(res.Execution)
	require.Equal(&votes[0], res.Vote)

	executions, err = svc.GetExecutionsByAddress(ta.Addrinfo["charlie"].RawAddress, 0, 10)
	require.NoError(err)
	res, err = svc.GetBlockOrActionByHash(executions[0].ID)
	require.NoError(err)
	require.Nil(res.Block)
	require.Nil(res.Transfer)
	require.Nil(res.Vote)
	require.Equal(&executions[0], res.Execution)

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
	p2p := mock_network.NewMockOverlay(ctrl)
	svc := Service{bc: chain, dp: mDp, p2p: p2p}

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Times(1)

	r := explorer.SendTransferRequest{
		Version:      0x1,
		Nonce:        1,
		Sender:       ta.Addrinfo["producer"].RawAddress,
		Recipient:    ta.Addrinfo["alfa"].RawAddress,
		Amount:       big.NewInt(1).String(),
		GasPrice:     big.NewInt(0).String(),
		SenderPubKey: keypair.EncodePublicKey(ta.Addrinfo["producer"].PublicKey),
		Signature:    "",
		Payload:      "",
	}
	response, err := svc.SendTransfer(r)
	require.NotNil(response.Hash)
	require.Nil(err)
	gas, err := svc.EstimateGasForTransfer(r)
	require.Nil(err)
	require.Equal(gas, int64(10000))
}

func TestService_SendVote(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	p2p := mock_network.NewMockOverlay(ctrl)
	svc := Service{bc: chain, dp: mDp, p2p: p2p}

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Times(1)

	r := explorer.SendVoteRequest{
		Version:     0x1,
		Nonce:       1,
		Voter:       ta.Addrinfo["producer"].RawAddress,
		Votee:       ta.Addrinfo["alfa"].RawAddress,
		VoterPubKey: keypair.EncodePublicKey(ta.Addrinfo["producer"].PublicKey),
		GasPrice:    big.NewInt(0).String(),
		Signature:   "",
	}

	response, err := svc.SendVote(r)
	require.NotNil(response.Hash)
	require.Nil(err)
	gas, err := svc.EstimateGasForVote()
	require.Nil(err)
	require.Equal(gas, int64(10000))
}

func TestService_SendSmartContract(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	p2p := mock_network.NewMockOverlay(ctrl)
	svc := Service{bc: chain, dp: mDp, p2p: p2p, gs: GasStation{chain, config.Explorer{}}}

	execution, _ := action.NewExecution(ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["delta"].RawAddress, 1, big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice), []byte{1})
	_ = action.Sign(execution, ta.Addrinfo["producer"].PrivateKey)
	explorerExecution, _ := convertExecutionToExplorerExecution(execution, true)
	explorerExecution.Version = int64(execution.Version())
	explorerExecution.ExecutorPubKey = keypair.EncodePublicKey(execution.ExecutorPublicKey())
	explorerExecution.Signature = hex.EncodeToString(execution.Signature())
	chain.EXPECT().ExecuteContractRead(gomock.Any()).Return(&action.Receipt{GasConsumed: 1000}, nil)

	gas, err := svc.EstimateGasForSmartContract(explorerExecution)
	require.Nil(err)
	require.Equal(gas, int64(1000))

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Times(1)

	response, err := svc.SendSmartContract(explorerExecution)
	require.NotNil(response.Hash)
	require.Nil(err)
}

func TestServicePutSubChainBlock(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	p2p := mock_network.NewMockOverlay(ctrl)
	svc := Service{bc: chain, dp: mDp, p2p: p2p}

	request := explorer.PutSubChainBlockRequest{}
	response, err := svc.PutSubChainBlock(request)
	require.Equal("", response.Hash)
	require.NotNil(err)

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Times(1)

	roots := []explorer.PutSubChainBlockMerkelRoot{
		{
			Name:  "a",
			Value: hex.EncodeToString([]byte("xddd")),
		},
	}
	r := explorer.PutSubChainBlockRequest{
		Version:       0x1,
		Nonce:         1,
		SenderAddress: ta.Addrinfo["producer"].RawAddress,
		SenderPubKey:  keypair.EncodePublicKey(ta.Addrinfo["producer"].PublicKey),
		GasPrice:      big.NewInt(0).String(),
		Signature:     "",
		Roots:         roots,
	}

	response, err = svc.PutSubChainBlock(r)
	require.NotNil(response.Hash)
	require.Nil(err)
}

func TestServiceSendAction(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	p2p := mock_network.NewMockOverlay(ctrl)
	svc := Service{bc: chain, dp: mDp, p2p: p2p}

	request := explorer.SendActionRequest{}
	_, err := svc.SendAction(request)
	require.NotNil(err)

	request.Payload = "abc"
	_, err = svc.SendAction(request)
	require.NotNil(err)

	roots := make(map[string]hash.Hash32B)
	roots["10002"] = byteutil.BytesTo32B([]byte("10002"))
	pb := action.NewPutBlock(
		1,
		"",
		ta.Addrinfo["producer"].RawAddress,
		100,
		roots,
		10000,
		big.NewInt(0),
	)
	pl := iproto.SendActionRequest{Action: pb.Proto()}
	d, err := proto.Marshal(&pl)
	require.NoError(err)
	request.Payload = hex.EncodeToString(d)

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Times(1)

	_, err = svc.SendAction(request)
	require.NoError(err)
}

func TestServiceGetPeers(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	p2p := mock_network.NewMockOverlay(ctrl)
	svc := Service{dp: mDp, p2p: p2p}

	p2p.EXPECT().GetPeers().Return([]net.Addr{
		&node.Node{Addr: "127.0.0.1:10002"},
		&node.Node{Addr: "127.0.0.1:10003"},
		&node.Node{Addr: "127.0.0.1:10004"},
	})
	p2p.EXPECT().Self().Return(&node.Node{Addr: "127.0.0.1:10001"})

	response, err := svc.GetPeers()
	require.Nil(err)
	require.Equal("127.0.0.1:10001", response.Self.Address)
	require.Len(response.Peers, 3)
	require.Equal("127.0.0.1:10003", response.Peers[1].Address)
}

func TestTransferPayloadBytesLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	p2p := mock_network.NewMockOverlay(ctrl)
	svc := Service{cfg: config.Explorer{MaxTransferPayloadBytes: 8}, dp: mDp, p2p: p2p}
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
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))
	// Disable block reward to make bookkeeping easier
	blockchain.Gen.BlockReward = big.NewInt(0)

	// create chain
	ctx := context.Background()
	bc := blockchain.NewBlockchain(cfg, blockchain.PrecreatedStateFactoryOption(sf), blockchain.InMemDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	require.Nil(err)
	err = bc.Stop(ctx)
	require.NoError(err)

	svc := Service{
		bc: bc,
		cfg: config.Explorer{
			TpsWindow:               10,
			MaxTransferPayloadBytes: 1024,
		},
	}

	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	// data, _ := hex.DecodeString("6060604052600436106100565763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166341c0e1b581146100585780637bf786f81461006b578063fbf788d61461009c575b005b341561006357600080fd5b6100566100ca565b341561007657600080fd5b61008a600160a060020a03600435166100f1565b60405190815260200160405180910390f35b34156100a757600080fd5b610056600160a060020a036004351660243560ff60443516606435608435610103565b60005433600160a060020a03908116911614156100ef57600054600160a060020a0316ff5b565b60016020526000908152604090205481565b600160a060020a0385166000908152600160205260408120548190861161012957600080fd5b3087876040516c01000000000000000000000000600160a060020a03948516810282529290931690910260148301526028820152604801604051809103902091506001828686866040516000815260200160405260006040516020015260405193845260ff90921660208085019190915260408085019290925260608401929092526080909201915160208103908084039060008661646e5a03f115156101cf57600080fd5b505060206040510351600054600160a060020a039081169116146101f257600080fd5b50600160a060020a03808716600090815260016020526040902054860390301631811161026257600160a060020a0387166000818152600160205260409081902088905582156108fc0290839051600060405180830381858888f19350505050151561025d57600080fd5b6102b7565b6000547f2250e2993c15843b32621c89447cc589ee7a9f049c026986e545d3c2c0c6f97890600160a060020a0316604051600160a060020a03909116815260200160405180910390a186600160a060020a0316ff5b505050505050505600a165627a7a72305820533e856fc37e3d64d1706bcc7dfb6b1d490c8d566ea498d9d01ec08965a896ca0029")
	execution, err := action.NewExecution(
		ta.Addrinfo["producer"].RawAddress, action.EmptyAddress, 1, big.NewInt(0), testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice), data)
	require.NoError(err)
	require.NoError(action.Sign(execution, ta.Addrinfo["producer"].PrivateKey))
	blk, err := bc.MintNewBlock([]action.Action{execution}, ta.Addrinfo["producer"], nil, nil, "")
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
	p2p := mock_network.NewMockOverlay(ctrl)
	p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	svc := Service{
		cfg: cfg.Explorer,
		bc:  bc,
		p2p: p2p,
		dp:  dp,
	}

	deposit := action.NewCreateDeposit(
		10,
		big.NewInt(10000),
		ta.Addrinfo["producer"].RawAddress,
		// Test explorer only, so that it doesn't matter the address is not on sub-chain
		ta.Addrinfo["alfa"].RawAddress,
		1000,
		big.NewInt(100),
	)
	require.NoError(action.Sign(deposit, ta.Addrinfo["producer"].PrivateKey))

	res, error := svc.CreateDeposit(explorer.CreateDepositRequest{
		Version:      int64(deposit.Version()),
		Nonce:        int64(deposit.Nonce()),
		Sender:       deposit.Sender(),
		SenderPubKey: keypair.EncodePublicKey(deposit.SenderPublicKey()),
		Recipient:    deposit.Recipient(),
		Amount:       deposit.Amount().String(),
		Signature:    hex.EncodeToString(deposit.Signature()),
		GasLimit:     int64(deposit.GasLimit()),
		GasPrice:     deposit.GasPrice().String(),
	})
	require.NoError(error)
	hash := deposit.Hash()
	require.Equal(hex.EncodeToString(hash[:]), res.Hash)
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
	p2p := mock_network.NewMockOverlay(ctrl)
	p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	svc := Service{
		cfg: cfg.Explorer,
		bc:  bc,
		p2p: p2p,
		dp:  dp,
	}

	deposit := action.NewSettleDeposit(
		10,
		big.NewInt(10000),
		100000,
		ta.Addrinfo["producer"].RawAddress,
		// Test explorer only, so that it doesn't matter the address is not on sub-chain
		ta.Addrinfo["alfa"].RawAddress,
		1000,
		big.NewInt(100),
	)
	require.NoError(action.Sign(deposit, ta.Addrinfo["producer"].PrivateKey))

	res, error := svc.SettleDeposit(explorer.SettleDepositRequest{
		Version:      int64(deposit.Version()),
		Nonce:        int64(deposit.Nonce()),
		Sender:       deposit.Sender(),
		SenderPubKey: keypair.EncodePublicKey(deposit.SenderPublicKey()),
		Recipient:    deposit.Recipient(),
		Amount:       deposit.Amount().String(),
		Index:        int64(deposit.Index()),
		Signature:    hex.EncodeToString(deposit.Signature()),
		GasLimit:     int64(deposit.GasLimit()),
		GasPrice:     deposit.GasPrice().String(),
	})
	require.NoError(error)
	hash := deposit.Hash()
	require.Equal(hex.EncodeToString(hash[:]), res.Hash)
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
	subChainAddr, err := address.IotxAddressToAddress(ta.Addrinfo["producer"].RawAddress)
	require.NoError(err)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	require.NoError(ws.PutState(
		mainchain.SubChainsInOperationKey,
		&state.SortedSlice{
			mainchain.InOperation{
				ID:   2,
				Addr: subChainAddr.Bytes(),
			},
		},
	))
	require.NoError(ws.PutState(
		byteutil.BytesTo20B(subChainAddr.Payload()),
		&mainchain.SubChain{
			DepositCount: 2,
		},
	))
	depositAddr1, err := address.IotxAddressToAddress(ta.Addrinfo["alfa"].RawAddress)
	require.NoError(err)
	require.NoError(ws.PutState(
		mainchain.DepositAddress(subChainAddr.Bytes(), 0),
		&mainchain.Deposit{
			Amount:    big.NewInt(100),
			Addr:      depositAddr1.Bytes(),
			Confirmed: false,
		},
	))
	depositAddr2, err := address.IotxAddressToAddress(ta.Addrinfo["bravo"].RawAddress)
	require.NoError(err)
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

func addCreatorToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = ws.LoadOrCreateAccountState(ta.Addrinfo["producer"].RawAddress, blockchain.Gen.TotalSupply); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := state.WithRunActionsCtx(context.Background(),
		state.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	if _, _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}
