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

	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/p2p"
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
	require.NotNil(svr.P2PAgent())

	// create client
	cfg, err = newTestConfig()
	require.Nil(err)
	cfg.Network.BootstrapNodes = []string{svr.P2PAgent().Self()[0].String()}
	p := p2p.NewAgent(
		cfg.Network,
		func(_ context.Context, _ uint32, _ proto.Message) {

		},
		func(_ context.Context, _ uint32, _ peerstore.PeerInfo, _ proto.Message) {

		},
	)
	require.NotNil(p)
	require.NoError(p.Start(ctx))

	defer func() {
		require.Nil(p.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	// check balance
	s, err := bc.StateByAddr(ta.Addrinfo["alfa"].Bech32())
	require.Nil(err)
	change := s.Balance
	t.Logf("Alfa balance = %d", change)
	require.True(change.String() == "23")

	s, err = bc.StateByAddr(ta.Addrinfo["bravo"].Bech32())
	require.Nil(err)
	beta := s.Balance
	t.Logf("Bravo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "34")

	s, err = bc.StateByAddr(ta.Addrinfo["charlie"].Bech32())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Charlie balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "47")

	s, err = bc.StateByAddr(ta.Addrinfo["delta"].Bech32())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Delta balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "69")

	s, err = bc.StateByAddr(ta.Addrinfo["echo"].Bech32())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Echo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "100")

	s, err = bc.StateByAddr(ta.Addrinfo["foxtrot"].Bech32())
	require.Nil(err)
	fox := s.Balance
	t.Logf("Foxtrot balance = %d", fox)
	change.Add(change, fox)
	require.True(fox.String() == "5242883")

	s, err = bc.StateByAddr(ta.Addrinfo["producer"].Bech32())
	require.Nil(err)
	test := s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)

	require.Equal(
		big.NewInt(0).Add(
			blockchain.ConvertIotxToRau(3000000000),
			big.NewInt(0).Mul(
				blockchain.Gen.BlockReward,
				big.NewInt(int64(bc.TipHeight())),
			),
		),
		change,
	)
	t.Log("Total balance match")

	if beta.Sign() == 0 || fox.Sign() == 0 || test.Sign() == 0 {
		return
	}
	require.True(5 == bc.TipHeight())

	// create local chain
	testutil.CleanupPath(t, testTriePath2)
	testutil.CleanupPath(t, testDBPath2)
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	require.NoError(copyDB(testTriePath, testTriePath2))
	require.NoError(copyDB(testDBPath, testDBPath2))
	chain := blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain))
	chain.Validator().AddActionValidators(account.NewProtocol())
	require.NotNil(chain)
	chain.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(chain))
	require.NoError(chain.Start(ctx))
	require.True(5 == bc.TipHeight())
	defer func() {
		require.NoError(chain.Stop(ctx))
		testutil.CleanupPath(t, testTriePath2)
		testutil.CleanupPath(t, testDBPath2)
	}()

	p2pCtx := p2p.WitContext(ctx, p2p.Context{ChainID: cfg.Chain.ID})
	// transfer 1
	// C --> A
	s, _ = bc.StateByAddr(ta.Addrinfo["charlie"].Bech32())
	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["charlie"].Bech32(), ta.Addrinfo["alfa"].Bech32(), ta.Keyinfo["charlie"].PriKey, s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	act1 := tsf1.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act1); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	})
	require.Nil(err)

	actionMap := svr.ChainService(chainID).ActionPool().PendingActionMap()
	blk1, err := chain.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk1, true))
	require.Nil(chain.CommitBlock(blk1))

	// transfer 2
	// F --> D
	s, _ = bc.StateByAddr(ta.Addrinfo["foxtrot"].Bech32())
	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["foxtrot"].Bech32(), ta.Addrinfo["delta"].Bech32(), ta.Keyinfo["foxtrot"].PriKey, s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[tsf2.SrcAddr()] = []action.SealedEnvelope{tsf2}
	blk2, err := chain.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk2, true))
	require.Nil(chain.CommitBlock(blk2))
	// broadcast to P2P
	act2 := tsf2.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act2); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 2, nil
	})
	require.Nil(err)

	// transfer 3
	// B --> B
	s, _ = bc.StateByAddr(ta.Addrinfo["bravo"].Bech32())
	tsf3, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["bravo"].PriKey, s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[tsf3.SrcAddr()] = []action.SealedEnvelope{tsf3}
	blk3, err := chain.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk3, true))
	require.Nil(chain.CommitBlock(blk3))
	// broadcast to P2P
	act3 := tsf3.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act3); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 3, nil
	})
	require.Nil(err)

	// transfer 4
	// test --> E
	s, _ = bc.StateByAddr(ta.Addrinfo["producer"].Bech32())
	tsf4, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["echo"].Bech32(), ta.Keyinfo["producer"].PriKey, s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[tsf4.SrcAddr()] = []action.SealedEnvelope{tsf4}
	blk4, err := chain.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk4, true))
	require.Nil(chain.CommitBlock(blk4))
	// broadcast to P2P
	act4 := tsf4.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act4); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 4, nil
	})
	require.Nil(err)
	// wait 4 blocks being picked and committed
	err = p.BroadcastOutbound(p2pCtx, blk2.ConvertToBlockPb())
	require.NoError(err)
	err = p.BroadcastOutbound(p2pCtx, blk4.ConvertToBlockPb())
	require.NoError(err)
	err = p.BroadcastOutbound(p2pCtx, blk1.ConvertToBlockPb())
	require.NoError(err)
	err = p.BroadcastOutbound(p2pCtx, blk3.ConvertToBlockPb())
	require.NoError(err)
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 9, nil
	})
	require.Nil(err)
	require.True(9 == bc.TipHeight())

	// check balance
	s, err = bc.StateByAddr(ta.Addrinfo["alfa"].Bech32())
	require.Nil(err)
	change = s.Balance
	t.Logf("Alfa balance = %d", change)
	require.True(change.String() == "24")

	s, err = bc.StateByAddr(ta.Addrinfo["bravo"].Bech32())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Bravo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "34")

	s, err = bc.StateByAddr(ta.Addrinfo["charlie"].Bech32())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Charlie balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "46")

	s, err = bc.StateByAddr(ta.Addrinfo["delta"].Bech32())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Delta balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "70")

	s, err = bc.StateByAddr(ta.Addrinfo["echo"].Bech32())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Echo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "101")

	s, err = bc.StateByAddr(ta.Addrinfo["foxtrot"].Bech32())
	require.Nil(err)
	fox = s.Balance
	t.Logf("Foxtrot balance = %d", fox)
	change.Add(change, fox)
	require.True(fox.String() == "5242882")

	s, err = bc.StateByAddr(ta.Addrinfo["producer"].Bech32())
	require.Nil(err)
	test = s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)

	require.Equal(
		big.NewInt(0).Add(
			blockchain.ConvertIotxToRau(3000000000),
			big.NewInt(0).Mul(
				blockchain.Gen.BlockReward,
				big.NewInt(int64(bc.TipHeight())),
			),
		),
		change,
	)
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
	require.NotNil(svr.P2PAgent())
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
	require.NotNil(svr.P2PAgent())

	testutil.CleanupPath(t, testTriePath2)
	testutil.CleanupPath(t, testDBPath2)

	cfg, err = newTestConfig()
	require.Nil(err)
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2

	// Create client
	cfg.NodeType = config.FullNodeType
	cfg.Network.BootstrapNodes = []string{svr.P2PAgent().Self()[0].String()}
	cfg.BlockSync.Interval = 1 * time.Second
	cli, err := itx.NewServer(cfg)
	require.Nil(err)
	require.Nil(cli.Start(ctx))
	require.NotNil(cli.ChainService(chainID).Blockchain())
	require.NotNil(cli.P2PAgent())

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
		testutil.CleanupPath(t, testTriePath2)
		testutil.CleanupPath(t, testDBPath2)
	}()

	err = testutil.WaitUntil(time.Millisecond*100, time.Second*60, func() (bool, error) {
		peers, err := svr.P2PAgent().Neighbors(context.Background())
		return len(peers) >= 1, err
	})
	require.Nil(err)

	err = svr.P2PAgent().BroadcastOutbound(
		p2p.WitContext(ctx, p2p.Context{ChainID: cfg.Chain.ID}),
		blk.ConvertToBlockPb(),
	)
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
	err = testutil.WaitUntil(time.Millisecond*100, time.Second*60, check)
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

	cfg, err = newTestConfig()
	require.NoError(err)
	cfg.Network.BootstrapNodes = []string{svr.P2PAgent().Self()[0].String()}
	p := p2p.NewAgent(
		cfg.Network,
		func(_ context.Context, _ uint32, _ proto.Message) {},
		func(_ context.Context, _ uint32, _ peerstore.PeerInfo, _ proto.Message) {},
	)
	require.NotNil(p)
	require.NoError(p.Start(ctx))

	defer func() {
		require.Nil(p.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()
	require.True(5 == bc.TipHeight())

	// create local chain
	testutil.CleanupPath(t, testTriePath2)
	testutil.CleanupPath(t, testDBPath2)
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	require.NoError(copyDB(testTriePath, testTriePath2))
	require.NoError(copyDB(testDBPath, testDBPath2))
	chain := blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain))
	chain.Validator().AddActionValidators(account.NewProtocol(),
		vote.NewProtocol(chain))
	require.NotNil(chain)
	chain.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(chain))
	require.NoError(chain.Start(ctx))
	require.True(5 == bc.TipHeight())
	defer func() {
		require.NoError(chain.Stop(ctx))
		testutil.CleanupPath(t, testTriePath2)
		testutil.CleanupPath(t, testDBPath2)
	}()

	// Add block 1
	// Alfa, Bravo and Charlie selfnomination
	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["alfa"].Bech32(), ta.Keyinfo["producer"].PriKey, 7, blockchain.ConvertIotxToRau(200000000), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["producer"].PriKey, 8, blockchain.ConvertIotxToRau(200000000), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	require.NoError(err)

	tsf3, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["charlie"].Bech32(), ta.Keyinfo["producer"].PriKey, 9, blockchain.ConvertIotxToRau(200000000), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	require.NoError(err)

	tsf4, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["delta"].Bech32(), ta.Keyinfo["producer"].PriKey, 10, blockchain.ConvertIotxToRau(200000000), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	require.NoError(err)

	vote1, err := testutil.SignedVote(ta.Addrinfo["alfa"].Bech32(), ta.Addrinfo["alfa"].Bech32(), ta.Keyinfo["alfa"].PriKey, 1, 100000, big.NewInt(0))
	require.NoError(err)

	vote2, err := testutil.SignedVote(ta.Addrinfo["bravo"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["bravo"].PriKey, 1, 100000, big.NewInt(0))
	require.NoError(err)

	vote3, err := testutil.SignedVote(ta.Addrinfo["charlie"].Bech32(), ta.Addrinfo["charlie"].Bech32(), ta.Keyinfo["charlie"].PriKey, 6, 100000, big.NewInt(0))
	require.NoError(err)

	act1 := vote1.Proto()
	act2 := vote2.Proto()
	act3 := vote3.Proto()
	acttsf1 := tsf1.Proto()
	acttsf2 := tsf2.Proto()
	acttsf3 := tsf3.Proto()
	acttsf4 := tsf4.Proto()

	p2pCtx := p2p.WitContext(ctx, p2p.Context{ChainID: cfg.Chain.ID})
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act1); err != nil {
			return false, err
		}
		if err := p.BroadcastOutbound(p2pCtx, act2); err != nil {
			return false, err
		}
		if err := p.BroadcastOutbound(p2pCtx, act3); err != nil {
			return false, err
		}
		if err := p.BroadcastOutbound(p2pCtx, acttsf1); err != nil {
			return false, err
		}
		if err := p.BroadcastOutbound(p2pCtx, acttsf2); err != nil {
			return false, err
		}
		if err := p.BroadcastOutbound(p2pCtx, acttsf3); err != nil {
			return false, err
		}
		if err := p.BroadcastOutbound(p2pCtx, acttsf4); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 7, nil
	})
	require.Nil(err)

	actionMap := svr.ChainService(chainID).ActionPool().PendingActionMap()
	blk1, err := chain.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk1, true))
	require.Nil(chain.CommitBlock(blk1))

	require.NoError(p.BroadcastOutbound(p2pCtx, blk1.ConvertToBlockPb()))
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 6, nil
	})
	require.NoError(err)
	tipheight := bc.TipHeight()
	require.Equal(6, int(tipheight))

	// Add block 2
	// Vote A -> B, C -> A
	vote4, err := testutil.SignedVote(ta.Addrinfo["alfa"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["alfa"].PriKey, uint64(2), uint64(100000), big.NewInt(0))
	require.Nil(err)
	vote5, err := testutil.SignedVote(ta.Addrinfo["charlie"].Bech32(), ta.Addrinfo["alfa"].Bech32(), ta.Keyinfo["charlie"].PriKey, uint64(7), uint64(100000), big.NewInt(0))
	require.Nil(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[vote4.SrcAddr()] = []action.SealedEnvelope{vote4}
	actionMap[vote5.SrcAddr()] = []action.SealedEnvelope{vote5}
	blk2, err := chain.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk2, true))
	require.Nil(chain.CommitBlock(blk2))
	// broadcast to P2P
	act4 := vote4.Proto()
	act5 := vote5.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act4); err != nil {
			return false, err
		}
		if err := p.BroadcastOutbound(p2pCtx, act5); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 2, nil
	})
	require.Nil(err)

	require.NoError(p.BroadcastOutbound(p2pCtx, blk2.ConvertToBlockPb()))
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
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
	require.Equal(ta.Addrinfo["alfa"].Bech32(), candidatesAddr[0])
	require.Equal(ta.Addrinfo["bravo"].Bech32(), candidatesAddr[1])

	// Add block 3
	// D self nomination
	vote6, err := testutil.SignedVote(ta.Addrinfo["delta"].Bech32(), ta.Addrinfo["delta"].Bech32(), ta.Keyinfo["delta"].PriKey, 5, 100000, big.NewInt(0))
	require.NoError(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[vote6.SrcAddr()] = []action.SealedEnvelope{vote6}
	blk3, err := chain.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk3, true))
	require.Nil(chain.CommitBlock(blk3))
	// broadcast to P2P
	act6 := vote6.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act6); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	})
	require.Nil(err)

	err = p.BroadcastOutbound(p2pCtx, blk3.ConvertToBlockPb())
	require.NoError(err)

	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
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
	require.Equal(ta.Addrinfo["bravo"].Bech32(), candidatesAddr[0])
	require.Equal(ta.Addrinfo["delta"].Bech32(), candidatesAddr[1])

	// Add block 4
	// Unvote B
	vote7, err := action.NewVote(uint64(2), ta.Addrinfo["bravo"].Bech32(), "", uint64(100000), big.NewInt(0))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(vote7).SetNonce(2).SetDestinationAddress("").SetGasLimit(100000).SetGasPrice(big.NewInt(0)).Build()
	selp, err := action.Sign(elp, ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["bravo"].PriKey)
	require.NoError(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[selp.SrcAddr()] = []action.SealedEnvelope{selp}
	blk4, err := chain.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	require.Nil(err)
	require.Nil(chain.ValidateBlock(blk4, true))
	require.Nil(chain.CommitBlock(blk4))
	// broadcast to P2P
	act7 := selp.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act7); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	})
	require.Nil(err)

	err = p.BroadcastOutbound(p2pCtx, blk4.ConvertToBlockPb())
	require.NoError(err)

	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
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
	require.Equal(ta.Addrinfo["alfa"].Bech32(), candidatesAddr[0])
	require.Equal(ta.Addrinfo["delta"].Bech32(), candidatesAddr[1])
}

func TestBlockchainRecovery(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

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

	bc = svr.ChainService(chainID).Blockchain()
	require.NotNil(bc)
	blockchainHeight := bc.TipHeight()
	factoryHeight, _ := bc.GetFactory().Height()
	require.Equal(blockchainHeight, factoryHeight)
	require.True(5 == blockchainHeight)
}

func newTestConfig() (config.Config, error) {
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()
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
