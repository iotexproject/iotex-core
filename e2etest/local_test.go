// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/server/itx"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/test/util"
)

const (
	localTestConfigPath = "../config.yaml"
	testDBPath          = "db.test"
	testDBPath2         = "db.test2"
	testTriePath        = "trie.test"
	testTriePath2       = "trie.test2"
)

func TestLocalCommit(t *testing.T) {
	require := require.New(t)

	cfg, err := config.LoadConfigWithPathWithoutValidation(localTestConfigPath)
	require.Nil(err)
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:10000"}

	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.InMemTest = false
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Delegate.Addrs = []string{"127.0.0.1:10000"}

	blockchain.Gen.TotalSupply = uint64(50 << 22)
	blockchain.Gen.BlockReward = uint64(0)

	// create node
	svr := itx.NewServer(*cfg)
	err = svr.Init()
	require.Nil(err)
	err = svr.Start()
	require.Nil(err)
	defer svr.Stop()

	bc := svr.Bc()
	require.NotNil(bc)
	require.Nil(addTestingTsfBlocks(bc))
	t.Log("Create blockchain pass")

	ap := svr.Ap()
	require.NotNil(ap)

	p2 := svr.P2p()
	require.NotNil(p2)

	p1 := network.NewOverlay(&cfg.Network)
	require.NotNil(p1)
	p1.PRC.Addr = "127.0.0.1:10001"
	p1.Init()
	p1.Start()
	defer p1.Stop()

	// check balance
	s, err := bc.StateByAddr(ta.Addrinfo["alfa"].RawAddress)
	require.Nil(err)
	change := s.Balance
	t.Logf("Alfa balance = %d", change)
	require.True(change.String() == "23")

	s, err = bc.StateByAddr(ta.Addrinfo["bravo"].RawAddress)
	require.Nil(err)
	beta := s.Balance
	t.Logf("Bravo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "34")

	s, err = bc.StateByAddr(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	beta = s.Balance
	t.Logf("Charlie balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "47")

	s, err = bc.StateByAddr(ta.Addrinfo["delta"].RawAddress)
	require.Nil(err)
	beta = s.Balance
	t.Logf("Delta balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "69")

	s, err = bc.StateByAddr(ta.Addrinfo["echo"].RawAddress)
	require.Nil(err)
	beta = s.Balance
	t.Logf("Echo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "100")

	s, err = bc.StateByAddr(ta.Addrinfo["foxtrot"].RawAddress)
	require.Nil(err)
	fox := s.Balance
	t.Logf("Foxtrot balance = %d", fox)
	change.Add(change, fox)
	require.True(fox.String() == "52428803")

	s, err = bc.StateByAddr(ta.Addrinfo["miner"].RawAddress)
	require.Nil(err)
	test := s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)

	require.Equal(uint64(100000000), change.Uint64())
	t.Log("Total balance match")

	if beta.Sign() == 0 || fox.Sign() == 0 || test.Sign() == 0 {
		return
	}

	height, err := bc.TipHeight()
	require.Nil(err)
	require.True(height == 4)

	// transfer 1
	// C --> A
	s, err = bc.StateByAddr(ta.Addrinfo["charlie"].RawAddress)
	tsf1 := action.NewTransfer(s.Nonce+1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["charlie"])
	act1 := &pb.ActionPb{&pb.ActionPb_Transfer{tsf1.ConvertToTransferPb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act1); err != nil {
			return false, err
		}
		tsf, _ := ap.PickActs()
		return len(tsf) == 1, nil
	})
	require.Nil(err)

	tsf, _ := ap.PickActs()
	blk1, err := bc.MintNewBlock(tsf, nil, ta.Addrinfo["miner"], "")
	hash1 := blk1.HashBlock()
	require.Nil(err)

	// transfer 2
	// F --> D
	s, err = bc.StateByAddr(ta.Addrinfo["foxtrot"].RawAddress)
	tsf2 := action.NewTransfer(s.Nonce+1, big.NewInt(1), ta.Addrinfo["foxtrot"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["foxtrot"])
	blk2 := blockchain.NewBlock(0, height+2, hash1, []*action.Transfer{tsf2}, nil)
	err = blk2.SignBlock(ta.Addrinfo["miner"])
	require.Nil(err)
	hash2 := blk2.HashBlock()
	act2 := &pb.ActionPb{&pb.ActionPb_Transfer{tsf2.ConvertToTransferPb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act2); err != nil {
			return false, err
		}
		tsf, _ := ap.PickActs()
		return len(tsf) == 2, nil
	})
	require.Nil(err)

	// transfer 3
	// B --> B
	s, err = bc.StateByAddr(ta.Addrinfo["bravo"].RawAddress)
	tsf3 := action.NewTransfer(s.Nonce+1, big.NewInt(1), ta.Addrinfo["bravo"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["bravo"])
	blk3 := blockchain.NewBlock(0, height+3, hash2, []*action.Transfer{tsf3}, nil)
	err = blk3.SignBlock(ta.Addrinfo["miner"])
	require.Nil(err)
	hash3 := blk3.HashBlock()
	act3 := &pb.ActionPb{&pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act3); err != nil {
			return false, err
		}
		tsf, _ := ap.PickActs()
		return len(tsf) == 3, nil
	})
	require.Nil(err)

	// transfer 4
	// test --> E
	s, err = bc.StateByAddr(ta.Addrinfo["miner"].RawAddress)
	tsf4 := action.NewTransfer(s.Nonce+1, big.NewInt(1), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["miner"])
	blk4 := blockchain.NewBlock(0, height+4, hash3, []*action.Transfer{tsf4}, nil)
	err = blk4.SignBlock(ta.Addrinfo["miner"])
	require.Nil(err)
	act4 := &pb.ActionPb{&pb.ActionPb_Transfer{tsf4.ConvertToTransferPb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act4); err != nil {
			return false, err
		}
		tsf, _ := ap.PickActs()
		return len(tsf) == 4, nil
	})
	require.Nil(err)

	p1.Broadcast(blk2.ConvertToBlockPb())
	p1.Broadcast(blk4.ConvertToBlockPb())
	p1.Broadcast(blk1.ConvertToBlockPb())
	p1.Broadcast(blk3.ConvertToBlockPb())

	err = util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		height, err := bc.TipHeight()
		if err != nil {
			return false, err
		}
		return int(height) == 8, nil
	})
	require.Nil(err)
	height, err = bc.TipHeight()
	require.Nil(err)
	require.Equal(8, int(height))

	// check balance
	s, err = bc.StateByAddr(ta.Addrinfo["alfa"].RawAddress)
	require.Nil(err)
	change = s.Balance
	t.Logf("Alfa balance = %d", change)
	require.True(change.String() == "24")

	s, err = bc.StateByAddr(ta.Addrinfo["bravo"].RawAddress)
	require.Nil(err)
	beta = s.Balance
	t.Logf("Bravo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "34")

	s, err = bc.StateByAddr(ta.Addrinfo["charlie"].RawAddress)
	require.Nil(err)
	beta = s.Balance
	t.Logf("Charlie balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "46")

	s, err = bc.StateByAddr(ta.Addrinfo["delta"].RawAddress)
	require.Nil(err)
	beta = s.Balance
	t.Logf("Delta balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "70")

	s, err = bc.StateByAddr(ta.Addrinfo["echo"].RawAddress)
	require.Nil(err)
	beta = s.Balance
	t.Logf("Echo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "101")

	s, err = bc.StateByAddr(ta.Addrinfo["foxtrot"].RawAddress)
	require.Nil(err)
	fox = s.Balance
	t.Logf("Foxtrot balance = %d", fox)
	change.Add(change, fox)
	require.True(fox.String() == "52428802")

	s, err = bc.StateByAddr(ta.Addrinfo["miner"].RawAddress)
	require.Nil(err)
	test = s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)

	require.Equal(uint64(100000000), change.Uint64())
	t.Log("Total balance match")
}

func TestLocalSync(t *testing.T) {
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	assert := assert.New(t)

	cfg, err := config.LoadConfigWithPathWithoutValidation(localTestConfigPath)
	assert.Nil(err)
	cfg.NodeType = config.DelegateType
	cfg.Delegate.Addrs = []string{"127.0.0.1:10000"}
	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.InMemTest = false
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Consensus.Scheme = config.NOOPScheme

	// create node 1
	svr := itx.NewServer(*cfg)
	err = svr.Init()
	assert.Nil(err)
	err = svr.Start()
	assert.Nil(err)
	defer svr.Stop()

	bc := svr.Bc()
	assert.NotNil(bc)
	assert.Nil(addTestingTsfBlocks(bc))
	t.Log("Create blockchain pass")

	blk, err := bc.GetBlockByHeight(1)
	assert.Nil(err)
	hash1 := blk.HashBlock()
	blk, err = bc.GetBlockByHeight(2)
	assert.Nil(err)
	hash2 := blk.HashBlock()
	blk, err = bc.GetBlockByHeight(3)
	assert.Nil(err)
	hash3 := blk.HashBlock()
	blk, err = bc.GetBlockByHeight(4)
	assert.Nil(err)
	hash4 := blk.HashBlock()

	p2 := svr.P2p()
	assert.NotNil(p2)

	util.CleanupPath(t, testTriePath2)
	defer util.CleanupPath(t, testTriePath2)
	util.CleanupPath(t, testDBPath2)
	defer util.CleanupPath(t, testDBPath2)

	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2

	// create node 2
	cfg.NodeType = config.FullNodeType
	cfg.Network.Addr = "127.0.0.1:10001"
	cli := itx.NewServer(*cfg)
	cli.Init()
	cli.Start()
	defer cli.Stop()

	bc1 := cli.Bc()
	assert.NotNil(bc1)

	p1 := cli.P2p()
	assert.NotNil(p1)

	// P1 download 4 blocks from P2
	p1.Tell(cm.NewTCPNode(p2.PRC.Addr), &pb.BlockSync{1, 4})
	check := util.CheckCondition(func() (bool, error) {
		blk1, err := bc1.GetBlockByHeight(1)
		if err != nil {
			return false, nil
		}
		blk2, err := bc1.GetBlockByHeight(2)
		if err != nil {
			return false, nil
		}
		blk3, err := bc1.GetBlockByHeight(3)
		if err != nil {
			return false, nil
		}
		blk4, err := bc1.GetBlockByHeight(4)
		if err != nil {
			return false, nil
		}
		return hash1 == blk1.HashBlock() && hash2 == blk2.HashBlock() && hash3 == blk3.HashBlock() && hash4 == blk4.HashBlock(), nil
	})
	err = util.WaitUntil(time.Millisecond*10, time.Millisecond*2000, check)
	assert.Nil(err)

	// verify 4 received blocks
	blk, err = bc1.GetBlockByHeight(1)
	assert.Nil(err)
	assert.Equal(hash1, blk.HashBlock())
	blk, err = bc1.GetBlockByHeight(2)
	assert.Nil(err)
	assert.Equal(hash2, blk.HashBlock())
	blk, err = bc1.GetBlockByHeight(3)
	assert.Nil(err)
	assert.Equal(hash3, blk.HashBlock())
	blk, err = bc1.GetBlockByHeight(4)
	assert.Nil(err)
	assert.Equal(hash4, blk.HashBlock())
	t.Log("4 blocks received correctly")
}

func TestVoteLocalCommit(t *testing.T) {
	require := require.New(t)

	cfg, err := config.LoadConfigWithPathWithoutValidation(localTestConfigPath)
	require.Nil(err)
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:10000"}

	util.CleanupPath(t, testTriePath)
	defer util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)
	defer util.CleanupPath(t, testDBPath)

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.InMemTest = false
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Delegate.Addrs = []string{"127.0.0.1:10000"}

	blockchain.Gen.TotalSupply = uint64(50 << 22)
	blockchain.Gen.BlockReward = uint64(0)

	// create node
	svr := itx.NewServer(*cfg)
	err = svr.Init()
	require.Nil(err)
	err = svr.Start()
	require.Nil(err)
	defer svr.Stop()

	bc := svr.Bc()
	require.NotNil(bc)
	require.Nil(addTestingTsfBlocks(bc))
	t.Log("Create blockchain pass")

	ap := svr.Ap()
	require.NotNil(ap)

	p2 := svr.P2p()
	require.NotNil(p2)

	p1 := network.NewOverlay(&cfg.Network)
	require.NotNil(p1)
	p1.PRC.Addr = "127.0.0.1:10001"
	p1.Init()
	p1.Start()
	defer p1.Stop()

	height, err := bc.TipHeight()
	require.Nil(err)
	require.True(height == 4)

	// Add block 1
	// Alfa, Bravo and Charlie selfnomination
	vote1, err := newSignedVote(1, ta.Addrinfo["alfa"], ta.Addrinfo["alfa"])
	require.Nil(err)
	vote2, err := newSignedVote(1, ta.Addrinfo["bravo"], ta.Addrinfo["bravo"])
	require.Nil(err)
	vote3, err := newSignedVote(2, ta.Addrinfo["charlie"], ta.Addrinfo["charlie"])
	require.Nil(err)
	act1 := &pb.ActionPb{&pb.ActionPb_Vote{vote1.ConvertToVotePb()}}
	act2 := &pb.ActionPb{&pb.ActionPb_Vote{vote2.ConvertToVotePb()}}
	act3 := &pb.ActionPb{&pb.ActionPb_Vote{vote3.ConvertToVotePb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act1); err != nil {
			return false, err
		}
		if err := p1.Broadcast(act2); err != nil {
			return false, err
		}
		if err := p1.Broadcast(act3); err != nil {
			return false, err
		}
		time.Sleep(time.Second)
		_, votes := ap.PickActs()
		return len(votes) == 3, nil
	})
	require.Nil(err)

	_, votes := ap.PickActs()
	blk1, err := bc.MintNewBlock(nil, votes, ta.Addrinfo["miner"], "")
	hash1 := blk1.HashBlock()
	require.Nil(err)

	// Add block 2
	// Vote A -> D, C -> A
	vote4, err := newSignedVote(2, ta.Addrinfo["alfa"], ta.Addrinfo["delta"])
	require.Nil(err)
	vote5, err := newSignedVote(3, ta.Addrinfo["charlie"], ta.Addrinfo["alfa"])
	require.Nil(err)
	blk2 := blockchain.NewBlock(0, height+2, hash1, nil, []*action.Vote{vote4, vote5})
	err = blk2.SignBlock(ta.Addrinfo["miner"])
	require.Nil(err)
	act4 := &pb.ActionPb{&pb.ActionPb_Vote{vote4.ConvertToVotePb()}}
	act5 := &pb.ActionPb{&pb.ActionPb_Vote{vote5.ConvertToVotePb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act4); err != nil {
			return false, err
		}
		if err := p1.Broadcast(act5); err != nil {
			return false, err
		}
		_, votes := ap.PickActs()
		return len(votes) == 5, nil
	})
	require.Nil(err)

	p1.Broadcast(blk1.ConvertToBlockPb())
	p1.Broadcast(blk2.ConvertToBlockPb())

	err = util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		height, err := bc.TipHeight()
		if err != nil {
			return false, err
		}
		return int(height) == 6, nil
	})
	require.Nil(err)
	height, err = bc.TipHeight()
	require.Nil(err)
	require.Equal(6, int(height))

	sf := svr.Sf()
	h, candidates := sf.Candidates()
	candidatesAddr := make([]string, len(candidates))
	for i, can := range candidates {
		candidatesAddr[i] = can.Address
	}
	require.Equal(6, int(h))
	require.Equal(2, len(candidates))

	sort.Sort(sort.StringSlice(candidatesAddr))
	require.Equal(ta.Addrinfo["alfa"].RawAddress, candidatesAddr[0])
	require.Equal(ta.Addrinfo["bravo"].RawAddress, candidatesAddr[1])

}

func newSignedVote(nonce int, from *iotxaddress.Address, to *iotxaddress.Address) (*action.Vote, error) {
	vote := action.NewVote(uint64(nonce), from.PublicKey, to.PublicKey)
	vote, err := vote.Sign(from)
	if err != nil {
		return nil, err
	}
	return vote, nil
}
