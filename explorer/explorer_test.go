// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_consensus"
	"github.com/iotexproject/iotex-core/test/mock/mock_dispatcher"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/test/util"
)

const (
	testTriePath = "trie.test"
	testDBPath   = "db.test"
)

const (
	senderRawAddr    = "io1qyqsyqcyy3vtzaghs2z30m8kzux8hqggpf0gmqu8xy959d"
	recipientRawAddr = "io1qyqsyqcy2a97s6h6lcrzuy7adyhyusnegn780rwcd0ssc2"
	senderPubKey     = "32d83ff52f2b34b297918a7ea3e032d5a1a3a635300a4fd7451b7c873650dedd2175e20536edc118df678f840820541b23287dd2a011a2b1865d91acfe859574f2c83f7d9542aa01"
	recipientPubKey  = "ba7aa38cc6f68da81832433a3c4911abea6730c381d9d6bac35ea36504060860d56567058fd96b0a746c3b3792f8a5a2b4295e6565a300ef92f10d5aa50953a1e15398463774c604"
)

func addTestingBlocks(bc blockchain.Blockchain) error {
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf := action.NewTransfer(0, big.NewInt(10), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["charlie"].RawAddress)
	tsf, _ = tsf.Sign(ta.Addrinfo["miner"])
	blk, err := bc.MintNewBlock([]*action.Transfer{tsf}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie --> A, B, D, E, test
	tsf1 := action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, _ = tsf1.Sign(ta.Addrinfo["charlie"])
	tsf2 := action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, _ = tsf2.Sign(ta.Addrinfo["charlie"])
	tsf3 := action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf3, _ = tsf3.Sign(ta.Addrinfo["charlie"])
	tsf4 := action.NewTransfer(0, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf4, _ = tsf4.Sign(ta.Addrinfo["charlie"])
	vote1 := action.NewVote(1, ta.Addrinfo["charlie"].PublicKey, ta.Addrinfo["delta"].PublicKey)
	vote1, _ = vote1.Sign(ta.Addrinfo["charlie"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4}, []*action.Vote{vote1}, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	blk, err = bc.MintNewBlock(nil, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	vote1 = action.NewVote(2, ta.Addrinfo["charlie"].PublicKey, ta.Addrinfo["alfa"].PublicKey)
	vote2 := action.NewVote(3, ta.Addrinfo["alfa"].PublicKey, ta.Addrinfo["charlie"].PublicKey)
	vote1, _ = vote1.Sign(ta.Addrinfo["charlie"])
	vote2, _ = vote2.Sign(ta.Addrinfo["alfa"])
	blk, err = bc.MintNewBlock(nil, []*action.Vote{vote1, vote2}, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	return nil
}

func addActsToActPool(ap actpool.ActPool) error {
	tsf1 := action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, _ = tsf1.Sign(ta.Addrinfo["miner"])
	vote1 := action.NewVote(2, ta.Addrinfo["miner"].PublicKey, ta.Addrinfo["miner"].PublicKey)
	vote1, _ = vote1.Sign(ta.Addrinfo["miner"])
	tsf2 := action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, _ = tsf2.Sign(ta.Addrinfo["miner"])
	if err := ap.AddTsf(tsf1); err != nil {
		return err
	}
	if err := ap.AddVote(vote1); err != nil {
		return err
	}
	if err := ap.AddTsf(tsf2); err != nil {
		return err
	}
	return nil
}

func TestExplorerApi(t *testing.T) {
	require := require.New(t)
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	sf, err := state.NewFactory(&cfg, state.InMemTrieOption())
	require.Nil(err)
	_, err = sf.CreateState(ta.Addrinfo["miner"].RawAddress, blockchain.Gen.TotalSupply)
	require.NoError(err)
	// Disable block reward to make bookkeeping easier
	blockchain.Gen.BlockReward = uint64(0)

	// create chain
	ctx := context.Background()
	bc := blockchain.NewBlockchain(&cfg, blockchain.PrecreatedStateFactoryOption(sf), blockchain.InMemDaoOption())
	require.NotNil(bc)
	ap, err := actpool.NewActPool(bc, cfg.ActPool)
	height, err := bc.TipHeight()
	require.Nil(err)
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)

	svc := Service{
		bc:        bc,
		ap:        ap,
		tpsWindow: 10,
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
	require.Equal(5, len(transfers))
	require.Nil(err)

	votes, err = svc.GetLastVotesByRange(4, 0, 10)
	require.Equal(10, len(votes))
	require.Nil(err)
	votes, err = svc.GetLastVotesByRange(3, 0, 50)
	require.Equal(22, len(votes))
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

	_, err = svc.GetVotesByBlockID("", 0, 10)
	require.Error(err)

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

	blk, err := svc.GetBlockByID(blks[0].ID)
	require.Nil(err)
	require.Equal(blks[0].Height, blk.Height)
	require.Equal(blks[0].Timestamp, blk.Timestamp)
	require.Equal(blks[0].Size, blk.Size)
	require.Equal(int64(0), blk.Votes)
	require.Equal(int64(1), blk.Transfers)

	_, err = svc.GetBlockByID("")
	require.Error(err)

	stats, err := svc.GetCoinStatistic()
	require.Nil(err)
	require.Equal(int64(blockchain.Gen.TotalSupply), stats.Supply)
	require.Equal(int64(4), stats.Height)
	require.Equal(int64(19), stats.Transfers)
	require.Equal(int64(24), stats.Votes)
	require.Equal(int64(12), stats.Aps)

	// success
	balance, err := svc.GetAddressBalance(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal(int64(6), balance)

	// error
	_, err = svc.GetAddressBalance("")
	require.Error(err)

	// success
	addressDetails, err := svc.GetAddressDetails(ta.Addrinfo["charlie"].RawAddress)
	require.Equal(int64(6), addressDetails.TotalBalance)
	require.Equal(int64(2), addressDetails.Nonce)
	require.Equal(int64(3), addressDetails.PendingNonce)
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
	transfers, err = svc.GetUnconfirmedTransfersByAddress(ta.Addrinfo["miner"].RawAddress, 0, 3)
	require.Nil(err)
	require.Equal(2, len(transfers))
	require.Equal(int64(1), transfers[0].Nonce)
	require.Equal(int64(3), transfers[1].Nonce)
	votes, err = svc.GetUnconfirmedVotesByAddress(ta.Addrinfo["miner"].RawAddress, 0, 3)
	require.Nil(err)
	require.Equal(1, len(votes))
	require.Equal(int64(2), votes[0].Nonce)
	transfers, err = svc.GetUnconfirmedTransfersByAddress(ta.Addrinfo["miner"].RawAddress, 1, 1)
	require.Nil(err)
	require.Equal(1, len(transfers))
	require.Equal(int64(3), transfers[0].Nonce)
	votes, err = svc.GetUnconfirmedVotesByAddress(ta.Addrinfo["miner"].RawAddress, 1, 1)
	require.Equal(0, len(votes))

	// error
	transfers, err = svc.GetUnconfirmedTransfersByAddress("", 0, 3)
	require.Error(err)
	votes, err = svc.GetUnconfirmedVotesByAddress("", 0, 3)
	require.Error(err)
}

func TestService_StateByAddr(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := state.State{
		Balance:      big.NewInt(46),
		Nonce:        uint64(0),
		Address:      "123",
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
	require.Equal("123", state.Address)
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

func TestService_CreateRawTransfer(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	svc := Service{bc: mBc}

	request := explorer.CreateRawTransferRequest{}
	response, err := svc.CreateRawTransfer(request)
	require.Equal(explorer.CreateRawTransferResponse{}, response)
	require.NotNil(err)

	request = explorer.CreateRawTransferRequest{Sender: senderRawAddr, Recipient: recipientRawAddr, Amount: 1, Nonce: 1}
	mBc.EXPECT().CreateRawTransfer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
		Return(&action.Transfer{Nonce: uint64(1), Amount: big.NewInt(1), Sender: senderRawAddr, Recipient: recipientPubKey})
	_, err = svc.CreateRawTransfer(request)
	require.Nil(err)
}

func TestService_SendTransfer(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	bcb := func(msg proto.Message) error {
		return nil
	}
	svc := Service{dp: mDp, broadcastcb: bcb}

	request := explorer.SendTransferRequest{}
	response, err := svc.SendTransfer(request)
	require.Equal(false, response.TransferSent)
	require.NotNil(err)

	tsfJSON := explorer.Transfer{Nonce: 1, Amount: 1, Sender: senderRawAddr, Recipient: recipientRawAddr, SenderPubKey: senderPubKey}
	stsf, err := json.Marshal(tsfJSON)
	require.Nil(err)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any()).Times(1)
	response, err = svc.SendTransfer(explorer.SendTransferRequest{hex.EncodeToString(stsf[:])})
	require.Equal(true, response.TransferSent)
	require.Nil(err)
}

func TestService_CreateRawVote(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	svc := Service{bc: mBc}

	request := explorer.CreateRawVoteRequest{}
	response, err := svc.CreateRawVote(request)
	require.Equal(explorer.CreateRawVoteResponse{}, response)
	require.NotNil(err)

	request = explorer.CreateRawVoteRequest{Voter: senderPubKey, Votee: recipientPubKey, Nonce: 1}
	selfPubKey, err := keypair.StringToPubKeyBytes(senderPubKey)
	require.Nil(err)
	votePubKey, err := keypair.StringToPubKeyBytes(recipientPubKey)
	require.Nil(err)
	mBc.EXPECT().CreateRawVote(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
		Return(&action.Vote{&pb.VotePb{Nonce: uint64(1), SelfPubkey: selfPubKey, VotePubkey: votePubKey}})
	_, err = svc.CreateRawVote(request)
	require.Nil(err)
}

func TestService_SendVote(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	bcb := func(msg proto.Message) error {
		return nil
	}
	svc := Service{dp: mDp, broadcastcb: bcb}

	request := explorer.SendVoteRequest{}
	response, err := svc.SendVote(request)
	require.Equal(false, response.VoteSent)
	require.NotNil(err)

	voteJSON := explorer.Vote{Nonce: 1, VoterPubKey: senderPubKey, VoteePubKey: recipientPubKey}
	svote, err := json.Marshal(voteJSON)
	require.Nil(err)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any()).Times(1)
	response, err = svc.SendVote(explorer.SendVoteRequest{hex.EncodeToString(svote[:])})
	require.Equal(true, response.VoteSent)
	require.Nil(err)
}
