// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"fmt"
	"math/big"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_consensus"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/test/util"
	"github.com/iotexproject/iotex-core/trie"
)

const (
	testingConfigPath = "../config.yaml"
	testTriePath      = "trie.test"
	testDBPath        = "db.test"
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

func TestExplorerApi(t *testing.T) {
	require := require.New(t)
	config, err := config.LoadConfigWithPathWithoutValidation(testingConfigPath)
	require.Nil(err)
	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	config.Chain.TrieDBPath = testTriePath
	config.Chain.InMemTest = true
	config.Chain.ChainDBPath = testDBPath

	tr, _ := trie.NewTrie(testTriePath, true)
	sf := state.NewFactory(tr)
	sf.CreateState(ta.Addrinfo["miner"].RawAddress, blockchain.Gen.TotalSupply)
	// Disable block reward to make bookkeeping easier
	blockchain.Gen.BlockReward = uint64(0)

	// create chain
	bc := blockchain.CreateBlockchain(config, sf)
	require.NotNil(bc)
	height, err := bc.TipHeight()
	require.Nil(err)
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingBlocks(bc))
	bc.Stop()

	svc := Service{
		bc:        bc,
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

	votes, err = svc.GetVotesByBlockID(blks[1].ID, 0, 0)
	require.Nil(err)
	require.Equal(0, len(votes))

	votes, err = svc.GetVotesByBlockID(blks[1].ID, 0, 10)
	require.Nil(err)
	require.Equal(1, len(votes))

	transfer, err := svc.GetTransferByID(transfers[0].ID)
	require.Nil(err)
	require.Equal(transfers[0].Sender, transfer.Sender)
	require.Equal(transfers[0].Recipient, transfer.Recipient)
	require.Equal(transfers[0].BlockID, transfer.BlockID)

	vote, err := svc.GetVoteByID(votes[0].ID)
	require.Nil(err)
	require.Equal(votes[0].Nounce, vote.Nounce)
	require.Equal(votes[0].BlockID, vote.BlockID)
	require.Equal(votes[0].Timestamp, vote.Timestamp)
	require.Equal(votes[0].ID, vote.ID)
	require.Equal(votes[0].Votee, vote.Votee)
	require.Equal(votes[0].Voter, vote.Voter)

	blk, err := svc.GetBlockByID(blks[0].ID)
	require.Nil(err)
	require.Equal(blks[0].Height, blk.Height)
	require.Equal(blks[0].Timestamp, blk.Timestamp)
	require.Equal(blks[0].Size, blk.Size)
	require.Equal(int64(0), blk.Votes)
	require.Equal(int64(1), blk.Transfers)

	stats, err := svc.GetCoinStatistic()
	require.Nil(err)
	require.Equal(int64(blockchain.Gen.TotalSupply), stats.Supply)
	require.Equal(int64(4), stats.Height)
	require.Equal(int64(19), stats.Transfers)
	require.Equal(int64(24), stats.Votes)
	require.Equal(int64(12), stats.Aps)

	balance, err := svc.GetAddressBalance(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal(int64(6), balance)

	addressDetails, err := svc.GetAddressDetails(ta.Addrinfo["charlie"].RawAddress)
	require.Equal(int64(6), addressDetails.TotalBalance)
	require.Equal(int64(2), addressDetails.Nonce)
	require.Equal(ta.Addrinfo["charlie"].RawAddress, addressDetails.Address)

	tip, err := svc.GetBlockchainHeight()
	require.Nil(err)
	require.Equal(4, int(tip))
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

	delegates := []net.Addr{
		common.NewTCPNode("127.0.0.1:40000"),
		common.NewTCPNode("127.0.0.1:40001"),
		common.NewTCPNode("127.0.0.1:40002"),
		common.NewTCPNode("127.0.0.1:40003"),
	}
	c := mock_consensus.NewMockConsensus(ctrl)
	c.EXPECT().Metrics().Return(scheme.ConsensusMetrics{
		LatestEpoch:         1,
		LatestDelegates:     delegates,
		LatestBlockProducer: delegates[3],
	}, nil)

	svc := Service{c: c}

	m, err := svc.GetConsensusMetrics()
	require.Nil(t, err)
	require.NotNil(t, m)
	require.Equal(t, int64(1), m.LatestEpoch)
	require.Equal(
		t,
		[]string{"127.0.0.1:40000", "127.0.0.1:40001", "127.0.0.1:40002", "127.0.0.1:40003"},
		m.LatestDelegates,
	)
	require.Equal(t, "127.0.0.1:40003", m.LatestBlockProducer)
}
