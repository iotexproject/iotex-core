// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	testDBPath    = "db.test"
	testDBPath2   = "db.test2"
	testTriePath  = "trie.test"
	testTriePath2 = "trie.test2"
)

func TestLocalCommit(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	blockchain.Gen.BlockReward = big.NewInt(0)

	cfg, err := newTestConfig()
	require.Nil(err)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)
	require.NoError(svr.Start(ctx))
	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	require.NotNil(bc)
	require.NoError(addTestingTsfBlocks(bc))
	require.NotNil(svr.ChainService(chainID).ActionPool())
	require.NotNil(svr.P2P())

	// create client
	cfg.Network.BootstrapNodes = []string{svr.P2P().Self().String()}
	p := network.NewOverlay(cfg.Network)
	require.NotNil(p)
	require.NoError(p.Start(ctx))

	defer func() {
		require.Nil(p.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

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
	require.True(fox.String() == "5242883")

	s, err = bc.StateByAddr(ta.Addrinfo["producer"].RawAddress)
	require.Nil(err)
	test := s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)

	require.Equal(blockchain.ConvertIotxToRau(3000000000).String(), change.String())
	t.Log("Total balance match")

	if beta.Sign() == 0 || fox.Sign() == 0 || test.Sign() == 0 {
		return
	}
	height := bc.TipHeight()
	require.Nil(err)
	require.True(height == 5)

	// create local chain
	testutil.CleanupPath(t, testTriePath2)
	testutil.CleanupPath(t, testDBPath2)
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	chain := blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	require.NotNil(chain)
	require.NoError(chain.Start(ctx))
	require.Nil(addTestingTsfBlocks(chain))
	height = chain.TipHeight()
	require.Nil(err)
	require.True(height == 5)
	defer func() {
		require.NoError(chain.Stop(ctx))
		testutil.CleanupPath(t, testTriePath2)
		testutil.CleanupPath(t, testDBPath2)
	}()

	// transfer 1
	// C --> A
	s, _ = bc.StateByAddr(ta.Addrinfo["charlie"].RawAddress)
	tsf1, _ := action.NewTransfer(s.Nonce+1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(0))
	_ = action.Sign(tsf1, ta.Addrinfo["charlie"].PrivateKey)
	act1 := tsf1.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.Broadcast(cfg.Chain.ID, act1); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	})
	require.Nil(err)

	acts := svr.ChainService(chainID).ActionPool().PickActs()
	blk1, err := chain.MintNewBlock(acts, ta.Addrinfo["producer"], nil,
		nil, "")
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk1, true))
	require.Nil(chain.CommitBlock(blk1))

	// transfer 2
	// F --> D
	s, _ = bc.StateByAddr(ta.Addrinfo["foxtrot"].RawAddress)
	tsf2, _ := action.NewTransfer(s.Nonce+1, big.NewInt(1), ta.Addrinfo["foxtrot"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), big.NewInt(0))
	_ = action.Sign(tsf2, ta.Addrinfo["foxtrot"].PrivateKey)
	blk2, err := chain.MintNewBlock([]action.Action{tsf2}, ta.Addrinfo["producer"], nil, nil, "")
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk2, true))
	require.Nil(chain.CommitBlock(blk2))
	// broadcast to P2P
	act2 := tsf2.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.Broadcast(cfg.Chain.ID, act2); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 2, nil
	})
	require.Nil(err)

	// transfer 3
	// B --> B
	s, _ = bc.StateByAddr(ta.Addrinfo["bravo"].RawAddress)
	tsf3, _ := action.NewTransfer(s.Nonce+1, big.NewInt(1), ta.Addrinfo["bravo"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(0))
	_ = action.Sign(tsf3, ta.Addrinfo["bravo"].PrivateKey)
	blk3, err := chain.MintNewBlock([]action.Action{tsf3}, ta.Addrinfo["producer"], nil, nil, "")
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk3, true))
	require.Nil(chain.CommitBlock(blk3))
	// broadcast to P2P
	act3 := tsf3.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.Broadcast(cfg.Chain.ID, act3); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 3, nil
	})
	require.Nil(err)

	// transfer 4
	// test --> E
	s, _ = bc.StateByAddr(ta.Addrinfo["producer"].RawAddress)
	tsf4, _ := action.NewTransfer(s.Nonce+1, big.NewInt(1), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), big.NewInt(0))
	_ = action.Sign(tsf4, ta.Addrinfo["producer"].PrivateKey)
	blk4, err := chain.MintNewBlock([]action.Action{tsf4}, ta.Addrinfo["producer"], nil, nil, "")
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk4, true))
	require.Nil(chain.CommitBlock(blk4))
	// broadcast to P2P
	act4 := tsf4.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.Broadcast(cfg.Chain.ID, act4); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 4, nil
	})
	require.Nil(err)
	// wait 4 blocks being picked and committed
	err = p.Broadcast(cfg.Chain.ID, blk2.ConvertToBlockPb())
	require.NoError(err)
	err = p.Broadcast(cfg.Chain.ID, blk4.ConvertToBlockPb())
	require.NoError(err)
	err = p.Broadcast(cfg.Chain.ID, blk1.ConvertToBlockPb())
	require.NoError(err)
	err = p.Broadcast(cfg.Chain.ID, blk3.ConvertToBlockPb())
	require.NoError(err)
	err = testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 9, nil
	})
	require.Nil(err)
	height = bc.TipHeight()
	require.Equal(9, int(height))

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
	require.True(fox.String() == "5242882")

	s, err = bc.StateByAddr(ta.Addrinfo["producer"].RawAddress)
	require.Nil(err)
	test = s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)

	require.Equal(blockchain.ConvertIotxToRau(3000000000).String(), change.String())
	t.Log("Total balance match")
}

func TestLocalSync(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	testutil.CleanupPath(t, testTriePath2)
	testutil.CleanupPath(t, testDBPath2)

	cfg, err := newTestConfig()
	require.Nil(err)

	// Create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)
	require.Nil(svr.Start(ctx))

	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	require.NotNil(bc)
	require.Nil(addTestingTsfBlocks(bc))

	blk, err := bc.GetBlockByHeight(1)
	require.Nil(err)
	hash1 := blk.HashBlock()
	blk, err = bc.GetBlockByHeight(2)
	require.Nil(err)
	hash2 := blk.HashBlock()
	blk, err = bc.GetBlockByHeight(3)
	require.Nil(err)
	hash3 := blk.HashBlock()
	blk, err = bc.GetBlockByHeight(4)
	require.Nil(err)
	hash4 := blk.HashBlock()
	blk, err = bc.GetBlockByHeight(5)
	require.Nil(err)
	hash5 := blk.HashBlock()

	p2 := svr.P2P()
	require.NotNil(p2)

	testutil.CleanupPath(t, testTriePath2)
	testutil.CleanupPath(t, testDBPath2)

	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2

	// Create client
	cfg.NodeType = config.FullNodeType
	cfg.Network.BootstrapNodes = []string{svr.P2P().Self().String()}
	cfg.BlockSync.Interval = 1 * time.Second
	cli, err := itx.NewServer(cfg)
	require.Nil(err)
	require.Nil(cli.Start(ctx))
	require.NotNil(cli.ChainService(chainID).Blockchain())
	require.NotNil(cli.P2P())

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
		testutil.CleanupPath(t, testTriePath2)
		testutil.CleanupPath(t, testDBPath2)
	}()

	err = testutil.WaitUntil(time.Millisecond*10, time.Second*5, func() (bool, error) { return len(svr.P2P().GetPeers()) >= 1, nil })
	require.Nil(err)

	err = svr.P2P().Broadcast(cfg.Chain.ID, blk.ConvertToBlockPb())
	require.NoError(err)
	check := testutil.CheckCondition(func() (bool, error) {
		blk1, err := cli.ChainService(chainID).Blockchain().GetBlockByHeight(1)
		if err != nil {
			return false, nil
		}
		blk2, err := cli.ChainService(chainID).Blockchain().GetBlockByHeight(2)
		if err != nil {
			return false, nil
		}
		blk3, err := cli.ChainService(chainID).Blockchain().GetBlockByHeight(3)
		if err != nil {
			return false, nil
		}
		blk4, err := cli.ChainService(chainID).Blockchain().GetBlockByHeight(4)
		if err != nil {
			return false, nil
		}
		blk5, err := cli.ChainService(chainID).Blockchain().GetBlockByHeight(5)
		if err != nil {
			return false, nil
		}
		return hash1 == blk1.HashBlock() &&
			hash2 == blk2.HashBlock() &&
			hash3 == blk3.HashBlock() &&
			hash4 == blk4.HashBlock() &&
			hash5 == blk5.HashBlock(), nil
	})
	err = testutil.WaitUntil(time.Millisecond*10, time.Second*5, check)
	require.Nil(err)

	// verify 4 received blocks
	blk, err = cli.ChainService(chainID).Blockchain().GetBlockByHeight(1)
	require.Nil(err)
	require.Equal(hash1, blk.HashBlock())
	blk, err = cli.ChainService(chainID).Blockchain().GetBlockByHeight(2)
	require.Nil(err)
	require.Equal(hash2, blk.HashBlock())
	blk, err = cli.ChainService(chainID).Blockchain().GetBlockByHeight(3)
	require.Nil(err)
	require.Equal(hash3, blk.HashBlock())
	blk, err = cli.ChainService(chainID).Blockchain().GetBlockByHeight(4)
	require.Nil(err)
	require.Equal(hash4, blk.HashBlock())
	blk, err = cli.ChainService(chainID).Blockchain().GetBlockByHeight(5)
	require.Nil(err)
	require.Equal(hash5, blk.HashBlock())
	t.Log("4 blocks received correctly")
}

func TestVoteLocalCommit(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	cfg, err := newTestConfig()
	cfg.Chain.NumCandidates = 2
	require.Nil(err)

	blockchain.Gen.BlockReward = big.NewInt(0)

	// create node
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)
	require.Nil(svr.Start(ctx))

	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	require.NotNil(bc)
	require.Nil(addTestingTsfBlocks(bc))
	require.NotNil(svr.ChainService(chainID).ActionPool())

	cfg.Network.BootstrapNodes = []string{svr.P2P().Self().String()}
	p := network.NewOverlay(cfg.Network)
	require.NotNil(p)
	require.NoError(p.Start(ctx))

	defer func() {
		require.Nil(p.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	height := bc.TipHeight()
	require.True(height == 5)

	// create local chain
	testutil.CleanupPath(t, testTriePath2)
	testutil.CleanupPath(t, testDBPath2)
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	chain := blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	require.NotNil(chain)
	require.NoError(chain.Start(ctx))
	require.Nil(addTestingTsfBlocks(chain))
	height = chain.TipHeight()
	require.True(height == 5)
	defer func() {
		require.NoError(chain.Stop(ctx))
		testutil.CleanupPath(t, testTriePath2)
		testutil.CleanupPath(t, testDBPath2)
	}()

	// Add block 1
	// Alfa, Bravo and Charlie selfnomination
	tsf1, err := action.NewTransfer(7, blockchain.ConvertIotxToRau(200000000), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	require.Nil(err)
	require.NoError(action.Sign(tsf1, ta.Addrinfo["producer"].PrivateKey))
	tsf2, err := action.NewTransfer(8, blockchain.ConvertIotxToRau(200000000), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	require.Nil(err)
	require.NoError(action.Sign(tsf2, ta.Addrinfo["producer"].PrivateKey))
	tsf3, err := action.NewTransfer(9, blockchain.ConvertIotxToRau(200000000), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	require.Nil(err)
	require.NoError(action.Sign(tsf3, ta.Addrinfo["producer"].PrivateKey))
	tsf4, err := action.NewTransfer(10, blockchain.ConvertIotxToRau(200000000), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	require.Nil(err)
	require.NoError(action.Sign(tsf4, ta.Addrinfo["producer"].PrivateKey))
	vote1, err := testutil.SignedVote(ta.Addrinfo["alfa"], ta.Addrinfo["alfa"], uint64(1), uint64(100000), big.NewInt(0))
	require.Nil(err)
	vote2, err := testutil.SignedVote(ta.Addrinfo["bravo"], ta.Addrinfo["bravo"], uint64(1), uint64(100000), big.NewInt(0))
	require.Nil(err)
	vote3, err := testutil.SignedVote(ta.Addrinfo["charlie"], ta.Addrinfo["charlie"], uint64(6), uint64(100000), big.NewInt(0))
	require.Nil(err)
	act1 := vote1.Proto()
	act2 := vote2.Proto()
	act3 := vote3.Proto()
	acttsf1 := tsf1.Proto()
	acttsf2 := tsf2.Proto()
	acttsf3 := tsf3.Proto()
	acttsf4 := tsf4.Proto()

	err = testutil.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		if err := p.Broadcast(chainID, act1); err != nil {
			return false, err
		}
		if err := p.Broadcast(chainID, act2); err != nil {
			return false, err
		}
		if err := p.Broadcast(chainID, act3); err != nil {
			return false, err
		}
		if err := p.Broadcast(chainID, acttsf1); err != nil {
			return false, err
		}
		if err := p.Broadcast(chainID, acttsf2); err != nil {
			return false, err
		}
		if err := p.Broadcast(chainID, acttsf3); err != nil {
			return false, err
		}
		if err := p.Broadcast(chainID, acttsf4); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 7, nil
	})
	require.Nil(err)

	acts := svr.ChainService(chainID).ActionPool().PickActs()
	blk1, err := chain.MintNewBlock(acts, ta.Addrinfo["producer"], nil,
		nil, "")
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk1, true))
	require.Nil(chain.CommitBlock(blk1))

	require.NoError(p.Broadcast(chainID, blk1.ConvertToBlockPb()))
	err = testutil.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 6, nil
	})
	require.NoError(err)
	tipheight := bc.TipHeight()
	require.Equal(6, int(tipheight))

	// Add block 2
	// Vote A -> B, C -> A
	vote4, err := testutil.SignedVote(ta.Addrinfo["alfa"], ta.Addrinfo["bravo"], uint64(2), uint64(100000), big.NewInt(0))
	require.Nil(err)
	vote5, err := testutil.SignedVote(ta.Addrinfo["charlie"], ta.Addrinfo["alfa"], uint64(7), uint64(100000), big.NewInt(0))
	require.Nil(err)
	blk2, err := chain.MintNewBlock([]action.Action{vote4, vote5}, ta.Addrinfo["producer"], nil, nil, "")
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk2, true))
	require.Nil(chain.CommitBlock(blk2))
	// broadcast to P2P
	act4 := vote4.Proto()
	act5 := vote5.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.Broadcast(chainID, act4); err != nil {
			return false, err
		}
		if err := p.Broadcast(chainID, act5); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 2, nil
	})
	require.Nil(err)

	require.NoError(p.Broadcast(chainID, blk2.ConvertToBlockPb()))
	err = testutil.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 7, nil
	})
	require.Nil(err)
	tipheight = bc.TipHeight()
	require.Equal(7, int(tipheight))

	candidates, err := bc.CandidatesByHeight(tipheight)
	require.NoError(err)
	candidatesAddr := make([]string, len(candidates))
	for i, can := range candidates {
		candidatesAddr[i] = can.Address
	}
	require.Equal(2, len(candidates))

	sort.Sort(sort.StringSlice(candidatesAddr))
	require.Equal(ta.Addrinfo["alfa"].RawAddress, candidatesAddr[0])
	require.Equal(ta.Addrinfo["bravo"].RawAddress, candidatesAddr[1])

	// Add block 3
	// D self nomination
	vote6, err := action.NewVote(uint64(5), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["delta"].RawAddress, uint64(100000), big.NewInt(0))
	require.NoError(err)
	require.NoError(action.Sign(vote6, ta.Addrinfo["delta"].PrivateKey))
	blk3, err := chain.MintNewBlock([]action.Action{vote6}, ta.Addrinfo["producer"],
		nil, nil, "")
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk3, true))
	require.Nil(chain.CommitBlock(blk3))
	// broadcast to P2P
	act6 := vote6.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.Broadcast(chainID, act6); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	})
	require.Nil(err)

	err = p.Broadcast(chainID, blk3.ConvertToBlockPb())
	require.NoError(err)

	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 8, nil
	})
	require.Nil(err)
	tipheight = bc.TipHeight()
	require.Equal(8, int(tipheight))

	candidates, err = bc.CandidatesByHeight(tipheight)
	require.NoError(err)
	candidatesAddr = make([]string, len(candidates))
	for i, can := range candidates {
		candidatesAddr[i] = can.Address
	}
	require.Equal(2, len(candidates))

	sort.Sort(sort.StringSlice(candidatesAddr))
	require.Equal(ta.Addrinfo["bravo"].RawAddress, candidatesAddr[0])
	require.Equal(ta.Addrinfo["delta"].RawAddress, candidatesAddr[1])

	// Add block 4
	// Unvote B
	vote7, err := action.NewVote(uint64(2), ta.Addrinfo["bravo"].RawAddress, "", uint64(100000), big.NewInt(0))
	require.NoError(err)
	require.NoError(action.Sign(vote7, ta.Addrinfo["bravo"].PrivateKey))
	blk4, err := chain.MintNewBlock([]action.Action{vote7}, ta.Addrinfo["producer"],
		nil, nil, "")
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk4, true))
	require.Nil(chain.CommitBlock(blk4))
	// broadcast to P2P
	act7 := vote7.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.Broadcast(chainID, act7); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	})
	require.Nil(err)

	err = p.Broadcast(chainID, blk4.ConvertToBlockPb())
	require.NoError(err)

	err = testutil.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 9, nil
	})
	require.Nil(err)
	tipheight = bc.TipHeight()
	require.Equal(9, int(tipheight))

	candidates, err = bc.CandidatesByHeight(tipheight)
	require.NoError(err)
	candidatesAddr = make([]string, len(candidates))
	for i, can := range candidates {
		candidatesAddr[i] = can.Address
	}
	require.Equal(2, len(candidates))

	sort.Sort(sort.StringSlice(candidatesAddr))
	require.Equal(ta.Addrinfo["alfa"].RawAddress, candidatesAddr[0])
	require.Equal(ta.Addrinfo["delta"].RawAddress, candidatesAddr[1])
}

func TestBlockchainRecovery(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	blockchain.Gen.BlockReward = big.NewInt(0)

	cfg, err := newTestConfig()
	require.Nil(err)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)
	require.NoError(svr.Start(ctx))

	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	require.NotNil(bc)
	require.NoError(addTestingTsfBlocks(bc))

	defer func() {
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	// stop server and delete state db
	require.NoError(svr.Stop(ctx))
	testutil.CleanupPath(t, testTriePath)

	// restart server
	svr, err = itx.NewServer(cfg)
	require.Nil(err)
	require.NoError(svr.Start(ctx))

	blockchainHeight := svr.ChainService(cfg.Chain.ID).Blockchain().TipHeight()
	factoryHeight, err := svr.ChainService(cfg.Chain.ID).Blockchain().GetFactory().Height()
	require.NoError(err)
	require.Equal(blockchainHeight, factoryHeight)
}

func newTestConfig() (config.Config, error) {
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = 0
	cfg.Explorer.Enabled = true
	cfg.Explorer.Port = 0

	pk, sk, err := crypto.EC283.NewKeyPair()
	if err != nil {
		return config.Config{}, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(pk)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(sk)
	return cfg, nil
}
