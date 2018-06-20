// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etests

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/server/itx"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/test/util"
)

const (
	localTestConfigPath = "../config.yaml"
	testTriePath        = "trie.test"
)

func TestLocalCommit(t *testing.T) {
	//t.Skip()
	//assert := assert.New(t)
	//
	//cfg, err := config.LoadConfigWithPathWithoutValidation(localTestConfigPath)
	//assert.Nil(err)
	//cfg.Network.BootstrapNodes = []string{"127.0.0.1:10000"}
	//// disable account-based testing
	//cfg.Chain.TrieDBPath = ""
	//cfg.Chain.InMemTest = true
	//cfg.Consensus.Scheme = config.NOOPScheme
	//cfg.Delegate.Addrs = []string{"127.0.0.1:10000"}
	//
	//blockchain.Gen.TotalSupply = uint64(50 << 22)
	//blockchain.Gen.BlockReward = uint64(0)
	//
	//// create node
	//svr := itx.NewServer(*cfg)
	//err = svr.Init()
	//assert.Nil(err)
	//err = svr.Start()
	//assert.Nil(err)
	//defer svr.Stop()
	//
	//bc := svr.Bc()
	//assert.NotNil(bc)
	//assert.Nil(addTestingBlocks(bc))
	//t.Log("Create blockchain pass")
	//
	////tp := svr.Tp()
	////assert.NotNil(tp)
	//
	//p2 := svr.P2p()
	//assert.NotNil(p2)
	//
	//p1 := network.NewOverlay(&cfg.Network)
	//assert.NotNil(p1)
	//p1.PRC.Addr = "127.0.0.1:10001"
	//p1.Init()
	//p1.Start()
	//defer p1.Stop()
	//
	//// check UTXO
	//change := bc.BalanceOf(ta.Addrinfo["alfa"].RawAddress)
	//t.Logf("Alfa balance = %d", change)
	//
	//beta := bc.BalanceOf(ta.Addrinfo["bravo"].RawAddress)
	//t.Logf("Bravo balance = %d", beta)
	//change.Add(change, beta)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["charlie"].RawAddress)
	//t.Logf("Charlie balance = %d", beta)
	//change.Add(change, beta)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["delta"].RawAddress)
	//t.Logf("Delta balance = %d", beta)
	//change.Add(change, beta)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["echo"].RawAddress)
	//t.Logf("Echo balance = %d", beta)
	//change.Add(change, beta)
	//
	//fox := bc.BalanceOf(ta.Addrinfo["foxtrot"].RawAddress)
	//t.Logf("Foxtrot balance = %d", fox)
	//change.Add(change, fox)
	//
	//test := bc.BalanceOf(ta.Addrinfo["miner"].RawAddress)
	//t.Logf("test balance = %d", test)
	//change.Add(change, test)
	//
	//assert.Equal(uint64(50<<22), change.Uint64())
	//t.Log("Total balance match")
	//
	//if beta.Sign() == 0 || fox.Sign() == 0 || test.Sign() == 0 {
	//	return
	//}
	//
	//height, err := bc.TipHeight()
	//assert.Nil(err)
	//
	//// transaction 1
	//// C --> A
	//payee := []*blockchain.Payee{}
	//payee = append(payee, &blockchain.Payee{ta.Addrinfo["alfa"].RawAddress, 1})
	//tx := bc.CreateTransaction(ta.Addrinfo["charlie"], 1, payee)
	//bc.ResetUTXO()
	//check := util.CheckCondition(func() (bool, error) {
	//	p1.Broadcast(tx.ConvertToTxPb())
	//	hash := tx.Hash()
	//	if tx, _ := tp.FetchTx(&hash); tx != nil {
	//		return true, nil
	//	}
	//	return false, nil
	//})
	//err = util.WaitUntil(time.Millisecond*10, time.Millisecond*2000, check)
	//assert.Nil(err)
	//
	//blk1, err := bc.MintNewBlock(tp.PickTxs(), nil, nil, ta.Addrinfo["miner"], "")
	//assert.Nil(err)
	//hash1 := blk1.HashBlock()
	//
	//// transaction 2
	//// F --> D
	//payee = nil
	//payee = append(payee, &blockchain.Payee{ta.Addrinfo["delta"].RawAddress, 1})
	//tx2 := bc.CreateTransaction(ta.Addrinfo["foxtrot"], 1, payee)
	//blk2 := blockchain.NewBlock(0, height+2, hash1, []*trx.Tx{tx2}, nil, nil)
	//hash2 := blk2.HashBlock()
	//bc.ResetUTXO()
	//p2.Broadcast(tx2.ConvertToTxPb())
	//
	//// transaction 3
	//// B --> B
	//payee = nil
	//payee = append(payee, &blockchain.Payee{ta.Addrinfo["bravo"].RawAddress, 1})
	//tx3 := bc.CreateTransaction(ta.Addrinfo["bravo"], 1, payee)
	//blk3 := blockchain.NewBlock(0, height+3, hash2, []*trx.Tx{tx3}, nil, nil)
	//hash3 := blk3.HashBlock()
	//bc.ResetUTXO()
	//p1.Broadcast(tx3.ConvertToTxPb())
	//
	//// transaction 4
	//// test --> E
	//payee = nil
	//payee = append(payee, &blockchain.Payee{ta.Addrinfo["echo"].RawAddress, 1})
	//tx4 := bc.CreateTransaction(ta.Addrinfo["miner"], 1, payee)
	//blk4 := blockchain.NewBlock(0, height+4, hash3, []*trx.Tx{tx4}, nil, nil)
	//bc.ResetUTXO()
	//p2.Broadcast(tx4.ConvertToTxPb())
	//
	//// send block 2-4-1-3 out of order
	//p1.Broadcast(blk2.ConvertToBlockPb())
	//p1.Broadcast(blk4.ConvertToBlockPb())
	//p1.Broadcast(blk1.ConvertToBlockPb())
	//p1.Broadcast(blk3.ConvertToBlockPb())
	//time.Sleep(time.Second)
	//
	//// TipHeight should be 8 here
	////check = util.CheckCondition(func() (bool, error) {
	////	height, _ = bc.TipHeight()
	////	if height == 8 {
	////		return true, nil
	////	}
	////	return false, nil
	////})
	////err = util.WaitUntil(time.Millisecond*10, time.Second * 5, check)
	////assert.Nil(err)
	//
	//height, err = bc.TipHeight()
	//assert.Nil(err)
	//t.Log("----- Block height = ", height)
	//
	//// check UTXO
	//change = bc.BalanceOf(ta.Addrinfo["alfa"].RawAddress)
	//t.Logf("Alfa balance = %d", change)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["bravo"].RawAddress)
	//t.Logf("Bravo balance = %d", beta)
	//change.Add(change, beta)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["charlie"].RawAddress)
	//t.Logf("Charlie balance = %d", beta)
	//change.Add(change, beta)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["delta"].RawAddress)
	//t.Logf("Delta balance = %d", beta)
	//change.Add(change, beta)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["echo"].RawAddress)
	//t.Logf("Echo balance = %d", beta)
	//change.Add(change, beta)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["foxtrot"].RawAddress)
	//t.Logf("Foxtrot balance = %d", beta)
	//change.Add(change, beta)
	//
	//beta = bc.BalanceOf(ta.Addrinfo["miner"].RawAddress)
	//t.Logf("test balance = %d", beta)
	//change.Add(change, beta)
	//
	//assert.Equal(uint64(50<<22), change.Uint64())
	//t.Log("Total balance match")
}

func TestLocalSync(t *testing.T) {
	l := logger.Logger().Level(zerolog.DebugLevel)
	logger.SetLogger(&l)
	assert := assert.New(t)

	cfg, err := config.LoadConfigWithPathWithoutValidation(localTestConfigPath)
	assert.Nil(err)
	cfg.NodeType = config.DelegateType
	cfg.Delegate.Addrs = []string{"127.0.0.1:10000"}
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	cfg.Chain.InMemTest = true
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
	p1.Tell(cm.NewTCPNode(p2.PRC.Addr), &iproto.BlockSync{1, 4})
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

func TestLocalCommitTsf(t *testing.T) {
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
}
