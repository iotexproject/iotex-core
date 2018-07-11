// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"encoding/hex"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/network/node"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/server/itx"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/test/util"
)

const (
	testDBPath    = "db.test"
	testDBPath2   = "db.test2"
	testTriePath  = "trie.test"
	testTriePath2 = "trie.test2"
)

func TestLocalCommit(t *testing.T) {
	require := require.New(t)

	util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)

	blockchain.Gen.TotalSupply = uint64(50 << 22)
	blockchain.Gen.BlockReward = uint64(0)

	cfg, err := newTestConfig()
	require.Nil(err)

	// create node
	ctx := context.Background()
	svr := itx.NewServer(cfg)
	require.Nil(svr.Start(ctx))

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
	p1.RPC.Addr = "127.0.0.1:10001"
	p1.Start(ctx)

	defer func() {
		require.Nil(p1.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		util.CleanupPath(t, testTriePath)
		util.CleanupPath(t, testDBPath)
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
	act1 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf1.ConvertToTransferPb()}}
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
	act2 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf2.ConvertToTransferPb()}}
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
	act3 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}
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
	act4 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf4.ConvertToTransferPb()}}
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

	err = util.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
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
	require := require.New(t)

	util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)

	cfg, err := newTestConfig()
	require.Nil(err)

	// create node 1
	ctx := context.Background()
	svr := itx.NewServer(cfg)
	require.Nil(svr.Start(ctx))

	bc := svr.Bc()
	require.NotNil(bc)
	require.Nil(addTestingTsfBlocks(bc))
	t.Log("Create blockchain pass")

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

	p2 := svr.P2p()
	require.NotNil(p2)

	util.CleanupPath(t, testTriePath2)
	util.CleanupPath(t, testDBPath2)

	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2

	// create node 2
	cfg.NodeType = config.FullNodeType
	cfg.Network.Addr = "127.0.0.1:10001"
	cli := itx.NewServer(cfg)
	require.Nil(cli.Start(ctx))

	bc1 := cli.Bc()
	require.NotNil(bc1)

	p1 := cli.P2p()
	require.NotNil(p1)

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(p1.Stop(ctx))
		require.Nil(p2.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		util.CleanupPath(t, testTriePath)
		util.CleanupPath(t, testDBPath)
		util.CleanupPath(t, testTriePath2)
		util.CleanupPath(t, testDBPath2)
	}()

	// P1 download 4 blocks from P2
	p1.Tell(node.NewTCPNode(p2.Self().String()), &pb.BlockSync{Start: 1, End: 4})
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
	err = util.WaitUntil(time.Millisecond*10, time.Second*5, check)
	require.Nil(err)

	// verify 4 received blocks
	blk, err = bc1.GetBlockByHeight(1)
	require.Nil(err)
	require.Equal(hash1, blk.HashBlock())
	blk, err = bc1.GetBlockByHeight(2)
	require.Nil(err)
	require.Equal(hash2, blk.HashBlock())
	blk, err = bc1.GetBlockByHeight(3)
	require.Nil(err)
	require.Equal(hash3, blk.HashBlock())
	blk, err = bc1.GetBlockByHeight(4)
	require.Nil(err)
	require.Equal(hash4, blk.HashBlock())
	t.Log("4 blocks received correctly")
}

func TestVoteLocalCommit(t *testing.T) {
	require := require.New(t)

	util.CleanupPath(t, testTriePath)
	util.CleanupPath(t, testDBPath)

	cfg, err := newTestConfig()
	require.Nil(err)

	blockchain.Gen.TotalSupply = uint64(50 << 22)
	blockchain.Gen.BlockReward = uint64(0)

	// create node
	ctx := context.Background()
	svr := itx.NewServer(cfg)
	require.Nil(svr.Start(ctx))

	bc := svr.Bc()
	require.NotNil(bc)
	require.Nil(addTestingTsfBlocks(bc))
	t.Log("Create blockchain pass")

	ap := svr.Ap()
	require.NotNil(ap)

	p1 := network.NewOverlay(&cfg.Network)
	require.NotNil(p1)
	p1.RPC.Addr = "127.0.0.1:10001"
	p1.Start(ctx)

	defer func() {
		require.Nil(p1.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		util.CleanupPath(t, testTriePath)
		util.CleanupPath(t, testDBPath)
	}()

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
	act1 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote1.ConvertToVotePb()}}
	act2 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote2.ConvertToVotePb()}}
	act3 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote3.ConvertToVotePb()}}
	err = util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
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

	p1.Broadcast(blk1.ConvertToBlockPb())
	err = util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		height, err := bc.TipHeight()
		if err != nil {
			return false, err
		}
		return int(height) == 5, nil
	})

	// Add block 2
	// Vote A -> B, C -> A
	vote4, err := newSignedVote(2, ta.Addrinfo["alfa"], ta.Addrinfo["bravo"])
	require.Nil(err)
	vote5, err := newSignedVote(3, ta.Addrinfo["charlie"], ta.Addrinfo["alfa"])
	require.Nil(err)
	blk2 := blockchain.NewBlock(0, height+2, hash1, nil, []*action.Vote{vote4, vote5})
	err = blk2.SignBlock(ta.Addrinfo["miner"])
	hash2 := blk2.HashBlock()
	require.Nil(err)
	act4 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote4.ConvertToVotePb()}}
	act5 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote5.ConvertToVotePb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act4); err != nil {
			return false, err
		}
		if err := p1.Broadcast(act5); err != nil {
			return false, err
		}
		_, votes := ap.PickActs()
		return len(votes) == 2, nil
	})
	require.Nil(err)

	p1.Broadcast(blk2.ConvertToBlockPb())

	err = util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		height, err := bc.TipHeight()
		if err != nil {
			return false, err
		}
		return int(height) == 6, nil
	})
	require.Nil(err)
	tipheight, err := bc.TipHeight()
	require.Nil(err)
	require.Equal(6, int(tipheight))

	h, candidates := svr.Bc().Candidates()
	candidatesAddr := make([]string, len(candidates))
	for i, can := range candidates {
		candidatesAddr[i] = can.Address
	}
	require.Equal(6, int(h))
	require.Equal(2, len(candidates))

	sort.Sort(sort.StringSlice(candidatesAddr))
	require.Equal(ta.Addrinfo["alfa"].RawAddress, candidatesAddr[0])
	require.Equal(ta.Addrinfo["bravo"].RawAddress, candidatesAddr[1])

	// Add block 3
	// D self nomination
	vote6 := action.NewVote(uint64(2), ta.Addrinfo["delta"].PublicKey, ta.Addrinfo["delta"].PublicKey)
	vote6, err = vote6.Sign(ta.Addrinfo["delta"])
	require.Nil(err)
	blk3 := blockchain.NewBlock(0, height+3, hash2, nil, []*action.Vote{vote6})
	err = blk3.SignBlock(ta.Addrinfo["miner"])
	hash3 := blk3.HashBlock()
	require.Nil(err)
	act6 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote6.ConvertToVotePb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act6); err != nil {
			return false, err
		}
		_, votes := ap.PickActs()
		return len(votes) == 1, nil
	})
	require.Nil(err)

	p1.Broadcast(blk3.ConvertToBlockPb())

	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		height, err := bc.TipHeight()
		if err != nil {
			return false, err
		}
		return int(height) == 7, nil
	})
	require.Nil(err)
	tipheight, err = bc.TipHeight()
	require.Nil(err)
	require.Equal(7, int(tipheight))

	h, candidates = svr.Bc().Candidates()
	candidatesAddr = make([]string, len(candidates))
	for i, can := range candidates {
		candidatesAddr[i] = can.Address
	}
	require.Equal(7, int(h))
	require.Equal(2, len(candidates))

	sort.Sort(sort.StringSlice(candidatesAddr))
	require.Equal(ta.Addrinfo["bravo"].RawAddress, candidatesAddr[0])
	require.Equal(ta.Addrinfo["delta"].RawAddress, candidatesAddr[1])

	// Add block 4
	// Unvote B
	vote7 := action.NewVote(uint64(3), ta.Addrinfo["bravo"].PublicKey, []byte{})
	vote7, err = vote7.Sign(ta.Addrinfo["bravo"])
	require.Nil(err)
	blk4 := blockchain.NewBlock(0, height+4, hash3, nil, []*action.Vote{vote7})
	err = blk4.SignBlock(ta.Addrinfo["miner"])
	require.Nil(err)
	act7 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote7.ConvertToVotePb()}}
	err = util.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p1.Broadcast(act7); err != nil {
			return false, err
		}
		_, votes := ap.PickActs()
		return len(votes) == 1, nil
	})
	require.Nil(err)

	p1.Broadcast(blk4.ConvertToBlockPb())

	err = util.WaitUntil(10*time.Millisecond, 5*time.Second, func() (bool, error) {
		height, err := bc.TipHeight()
		if err != nil {
			return false, err
		}
		return int(height) == 8, nil
	})
	require.Nil(err)
	tipheight, err = bc.TipHeight()
	require.Nil(err)
	require.Equal(8, int(tipheight))

	h, candidates = svr.Bc().Candidates()
	candidatesAddr = make([]string, len(candidates))
	for i, can := range candidates {
		candidatesAddr[i] = can.Address
	}
	require.Equal(8, int(h))
	require.Equal(2, len(candidates))

	sort.Sort(sort.StringSlice(candidatesAddr))
	require.Equal(ta.Addrinfo["alfa"].RawAddress, candidatesAddr[0])
	require.Equal(ta.Addrinfo["delta"].RawAddress, candidatesAddr[1])
}

func newSignedVote(nonce int, from *iotxaddress.Address, to *iotxaddress.Address) (*action.Vote, error) {
	vote := action.NewVote(uint64(nonce), from.PublicKey, to.PublicKey)
	vote, err := vote.Sign(from)
	if err != nil {
		return nil, err
	}
	return vote, nil
}

func newTestConfig() (*config.Config, error) {
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Addr = "127.0.0.1:10000"
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:10000"}
	cfg.Delegate.Addrs = []string{"127.0.0.1:10000"}
	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = hex.EncodeToString(addr.PublicKey)
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(addr.PrivateKey)
	return &cfg, nil
}
