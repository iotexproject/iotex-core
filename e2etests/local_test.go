// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etests

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain"
	cm "github.com/iotexproject/iotex-core/common"
	cfg "github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/server/itx"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

const (
	localTestConfigPath = "../config.yaml"
	testDBPath          = "db.test"
	testDB2Path         = "db2.test"
)

func TestLocalCommit(t *testing.T) {
	assert := assert.New(t)
	os.Remove(testDBPath)
	defer os.Remove(testDBPath)

	config, err := cfg.LoadConfigWithPathWithoutValidation(localTestConfigPath)
	assert.Nil(err)
	config.Network.BootstrapNodes = []string{"127.0.0.1:10000"}
	config.Chain.ChainDBPath = testDBPath
	config.Consensus.Scheme = "NOOP"
	config.Delegate.Addrs = []string{"127.0.0.1:10000"}

	blockchain.Gen.TotalSupply = uint64(50 << 22)
	blockchain.Gen.Coinbase = uint64(0)

	// create node
	svr := itx.NewServer(*config)
	svr.Init()
	svr.Start()
	defer svr.Stop()

	bc := svr.Bc()
	assert.NotNil(bc)
	assert.Nil(addTestingBlocks(bc))
	t.Log("Create blockchain pass")

	tp := svr.Tp()
	assert.NotNil(tp)

	p2 := svr.P2p()
	assert.NotNil(p2)

	p1 := network.NewOverlay(&config.Network)
	assert.NotNil(p1)
	p1.PRC.Addr = "127.0.0.1:10001"
	p1.Init()
	p1.Start()
	defer p1.Stop()

	// check UTXO
	change := bc.BalanceOf(ta.Addrinfo["alfa"].Address)
	t.Logf("Alfa balance = %d", change)

	beta := bc.BalanceOf(ta.Addrinfo["bravo"].Address)
	t.Logf("Bravo balance = %d", beta)
	change += beta

	beta = bc.BalanceOf(ta.Addrinfo["charlie"].Address)
	t.Logf("Charlie balance = %d", beta)
	change += beta

	beta = bc.BalanceOf(ta.Addrinfo["delta"].Address)
	t.Logf("Delta balance = %d", beta)
	change += beta

	beta = bc.BalanceOf(ta.Addrinfo["echo"].Address)
	t.Logf("Echo balance = %d", beta)
	change += beta

	fox := bc.BalanceOf(ta.Addrinfo["foxtrot"].Address)
	t.Logf("Foxtrot balance = %d", fox)
	change += fox

	test := bc.BalanceOf(ta.Addrinfo["miner"].Address)
	t.Logf("test balance = %d", test)
	change += test

	assert.Equal(uint64(50<<22), change)
	t.Log("Total balance match")

	if beta == 0 || fox == 0 || test == 0 {
		return
	}

	height := bc.TipHeight()

	// transaction 1
	// C --> A
	payee := []*blockchain.Payee{}
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["alfa"].Address, 1})
	tx := bc.CreateTransaction(ta.Addrinfo["charlie"], 1, payee)
	bc.Reset()
	p1.Broadcast(tx.ConvertToTxPb())
	time.Sleep(time.Second)

	blk1 := bc.MintNewBlock(tp.Txs(), ta.Addrinfo["miner"].Address, "")
	hash1 := blk1.HashBlock()

	// transaction 2
	// F --> D
	payee = nil
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["delta"].Address, 1})
	tx2 := bc.CreateTransaction(ta.Addrinfo["foxtrot"], 1, payee)
	blk2 := blockchain.NewBlock(0, height+2, hash1, []*blockchain.Tx{tx2})
	hash2 := blk2.HashBlock()
	bc.Reset()
	p2.Broadcast(tx2.ConvertToTxPb())

	// transaction 3
	// B --> B
	payee = nil
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["bravo"].Address, 1})
	tx3 := bc.CreateTransaction(ta.Addrinfo["bravo"], 1, payee)
	blk3 := blockchain.NewBlock(0, height+3, hash2, []*blockchain.Tx{tx3})
	hash3 := blk3.HashBlock()
	bc.Reset()
	p1.Broadcast(tx3.ConvertToTxPb())

	// transaction 4
	// test --> E
	payee = nil
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["echo"].Address, 1})
	tx4 := bc.CreateTransaction(ta.Addrinfo["miner"], 1, payee)
	blk4 := blockchain.NewBlock(0, height+4, hash3, []*blockchain.Tx{tx4})
	bc.Reset()
	p2.Broadcast(tx4.ConvertToTxPb())

	// send block 2-4-1-3 out of order
	p2.Broadcast(blk2.ConvertToBlockPb())
	p1.Broadcast(blk4.ConvertToBlockPb())
	p1.Broadcast(blk1.ConvertToBlockPb())
	p2.Broadcast(blk3.ConvertToBlockPb())
	time.Sleep(time.Second)

	t.Log("----- Block height = ", bc.TipHeight())

	// check UTXO
	change = bc.BalanceOf(ta.Addrinfo["alfa"].Address)
	t.Logf("Alfa balance = %d", change)

	beta = bc.BalanceOf(ta.Addrinfo["bravo"].Address)
	t.Logf("Bravo balance = %d", beta)
	change += beta

	beta = bc.BalanceOf(ta.Addrinfo["charlie"].Address)
	t.Logf("Charlie balance = %d", beta)
	change += beta

	beta = bc.BalanceOf(ta.Addrinfo["delta"].Address)
	t.Logf("Delta balance = %d", beta)
	change += beta

	beta = bc.BalanceOf(ta.Addrinfo["echo"].Address)
	t.Logf("Echo balance = %d", beta)
	change += beta

	beta = bc.BalanceOf(ta.Addrinfo["foxtrot"].Address)
	t.Logf("Foxtrot balance = %d", beta)
	change += beta

	beta = bc.BalanceOf(ta.Addrinfo["miner"].Address)
	t.Logf("test balance = %d", beta)
	change += beta

	assert.Equal(uint64(50<<22), change)
	t.Log("Total balance match")
}

func TestLocalSync(t *testing.T) {
	assert := assert.New(t)
	os.Remove(testDBPath)
	defer os.Remove(testDBPath)
	os.Remove(testDB2Path)
	defer os.Remove(testDB2Path)

	config, err := cfg.LoadConfigWithPathWithoutValidation(localTestConfigPath)
	assert.Nil(err)
	config.NodeType = cfg.DelegateType
	config.Delegate.Addrs = []string{"127.0.0.1:10000"}
	config.Chain.ChainDBPath = testDBPath
	config.Consensus.Scheme = "NOOP"

	// create node 1
	svr := itx.NewServer(*config)
	svr.Init()
	svr.Start()
	defer svr.Stop()

	bc := svr.Bc()
	assert.NotNil(bc)
	assert.Nil(addTestingBlocks(bc))
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
	config.NodeType = cfg.FullNodeType
	config.Network.Addr = "127.0.0.1:10001"
	config.Chain.ChainDBPath = testDB2Path
	cli := itx.NewServer(*config)
	cli.Init()
	cli.Start()
	defer cli.Stop()

	bc1 := cli.Bc()
	assert.NotNil(bc1)

	p1 := cli.P2p()
	assert.NotNil(p1)

	// P1 download 4 blocks from P2
	p1.Tell(cm.NewTCPNode(p2.PRC.Addr), &iproto.BlockSync{1, 4})
	time.Sleep(time.Second)

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
