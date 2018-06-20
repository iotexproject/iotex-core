// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
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
	vote1 := action.NewVote(0, ta.Addrinfo["charlie"].PublicKey, ta.Addrinfo["charlie"].PublicKey)
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
	vote1 = action.NewVote(0, ta.Addrinfo["charlie"].PublicKey, ta.Addrinfo["charlie"].PublicKey)
	vote2 := action.NewVote(0, ta.Addrinfo["alfa"].PublicKey, ta.Addrinfo["alfa"].PublicKey)
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
	require.Equal(len(transfers), 5)

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

	blks, getBlkErr := svc.GetLastBlocksByRange(3, 4)
	require.Nil(getBlkErr)
	require.Equal(len(blks), 4)

	transfers, err = svc.GetTransfersByBlockID(blks[2].ID, 0, 10)
	require.Nil(err)
	require.Equal(len(transfers), 2)

	transfer, err := svc.GetTransferByID(transfers[0].ID)
	require.Nil(err)
	require.Equal(transfer.Sender, transfers[0].Sender)
	require.Equal(transfer.Recipient, transfers[0].Recipient)

	blk, err := svc.GetBlockByID(blks[0].ID)
	require.Nil(err)
	require.Equal(blk.Height, blks[0].Height)
	require.Equal(blk.Timestamp, blks[0].Timestamp)
	require.Equal(blk.Size, blks[0].Size)
	require.Equal(blk.Votes, int64(0))
	require.Equal(blk.Transfers, int64(1))

	stats, err := svc.GetCoinStatistic()
	require.Nil(err)
	require.Equal(stats.Supply, int64(blockchain.Gen.TotalSupply))
	require.Equal(stats.Height, int64(4))
	require.Equal(stats.Transfers, int64(19))
	require.Equal(stats.Votes, int64(24))
	require.Equal(stats.Aps, int64(12))

	balance, err := svc.GetAddressBalance(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	require.Equal(balance, int64(6))

	addressDetails, err := svc.GetAddressDetails(ta.Addrinfo["charlie"].RawAddress)
	require.Equal(addressDetails.TotalBalance, int64(6))
	require.Equal(addressDetails.Nonce, int64(0))
	require.Equal(addressDetails.Address, ta.Addrinfo["charlie"].RawAddress)
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
