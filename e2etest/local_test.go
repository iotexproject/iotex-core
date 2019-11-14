// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/crypto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	dBPath    = "db.test"
	dBPath2   = "db.test2"
	triePath  = "trie.test"
	triePath2 = "trie.test2"
)

func TestLocalCommit(t *testing.T) {
	require := require.New(t)

	cfg, err := newTestConfig()
	require.Nil(err)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	testDBPath := testDBFile.Name()
	indexDBFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	indexDBPath := indexDBFile.Name()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = indexDBPath

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
		cfg,
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
		require.Nil(bc.Stop(ctx))
	}()

	// check balance
	s, err := bc.Factory().AccountState(identityset.Address(28).String())
	require.Nil(err)
	change := s.Balance
	t.Logf("Alfa balance = %d", change)
	require.True(change.String() == "23")

	s, err = bc.Factory().AccountState(identityset.Address(29).String())
	require.Nil(err)
	beta := s.Balance
	t.Logf("Bravo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "34")

	s, err = bc.Factory().AccountState(identityset.Address(30).String())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Charlie balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "47")

	s, err = bc.Factory().AccountState(identityset.Address(31).String())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Delta balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "69")

	s, err = bc.Factory().AccountState(identityset.Address(32).String())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Echo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "100")

	s, err = bc.Factory().AccountState(identityset.Address(33).String())
	require.Nil(err)
	fox := s.Balance
	t.Logf("Foxtrot balance = %d", fox)
	change.Add(change, fox)
	require.True(fox.String() == "5242883")

	s, err = bc.Factory().AccountState(identityset.Address(27).String())
	require.Nil(err)
	test := s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)

	require.Equal(
		unit.ConvertIotxToRau(90000000),
		change,
	)
	t.Log("Total balance match")

	if beta.Sign() == 0 || fox.Sign() == 0 || test.Sign() == 0 {
		return
	}
	require.EqualValues(5, bc.TipHeight())

	// create local chain
	testTrieFile2, _ := ioutil.TempFile(os.TempDir(), triePath2)
	testTriePath2 := testTrieFile2.Name()
	testDBFile2, _ := ioutil.TempFile(os.TempDir(), dBPath2)
	testDBPath2 := testDBFile2.Name()
	indexDBFile2, _ := ioutil.TempFile(os.TempDir(), dBPath2)
	indexDBPath2 := indexDBFile2.Name()
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = indexDBPath2
	require.NoError(copyDB(testTriePath, testTriePath2))
	require.NoError(copyDB(testDBPath, testDBPath2))
	require.NoError(copyDB(indexDBPath, indexDBPath2))
	registry := protocol.Registry{}
	chain := blockchain.NewBlockchain(
		cfg,
		nil,
		blockchain.DefaultStateFactoryOption(),
		blockchain.BoltDBDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	require.NoError(registry.Register(rolldpos.ProtocolID, rolldposProtocol))
	rewardingProtocol := rewarding.NewProtocol(chain, rolldposProtocol)
	registry.Register(rewarding.ProtocolID, rewardingProtocol)
	acc := account.NewProtocol()
	registry.Register(account.ProtocolID, acc)
	chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain.Factory().Nonce))
	chain.Validator().AddActionValidators(acc, rewardingProtocol)
	chain.Factory().AddActionHandlers(acc, rewardingProtocol)
	require.NoError(chain.Start(ctx))
	require.EqualValues(5, chain.TipHeight())
	defer func() {
		require.NoError(chain.Stop(ctx))
	}()

	p2pCtx := p2p.WitContext(ctx, p2p.Context{ChainID: cfg.Chain.ID})
	// transfer 1
	// C --> A
	s, _ = bc.Factory().AccountState(identityset.Address(30).String())
	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(30), s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	act1 := tsf1.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act1); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 1, nil
	})
	require.Nil(err)

	actionMap := svr.ChainService(chainID).ActionPool().PendingActionMap()
	blk1, err := chain.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.Nil(err)
	require.Nil(chain.CommitBlock(blk1))

	// transfer 2
	// F --> D
	s, _ = bc.Factory().AccountState(identityset.Address(33).String())
	tsf2, err := testutil.SignedTransfer(identityset.Address(31).String(), identityset.PrivateKey(33), s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[identityset.Address(33).String()] = []action.SealedEnvelope{tsf2}
	blk2, err := chain.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.Nil(err)
	require.Nil(chain.CommitBlock(blk2))
	// broadcast to P2P
	act2 := tsf2.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act2); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 2, nil
	})
	require.Nil(err)

	// transfer 3
	// B --> B
	s, _ = bc.Factory().AccountState(identityset.Address(29).String())
	tsf3, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[identityset.Address(29).String()] = []action.SealedEnvelope{tsf3}
	blk3, err := chain.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.Nil(err)
	require.Nil(chain.CommitBlock(blk3))
	// broadcast to P2P
	act3 := tsf3.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act3); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 3, nil
	})
	require.Nil(err)

	// transfer 4
	// test --> E
	s, _ = bc.Factory().AccountState(identityset.Address(27).String())
	tsf4, err := testutil.SignedTransfer(identityset.Address(32).String(), identityset.PrivateKey(27), s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[identityset.Address(27).String()] = []action.SealedEnvelope{tsf4}
	blk4, err := chain.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.Nil(err)
	require.Nil(chain.CommitBlock(blk4))
	// broadcast to P2P
	act4 := tsf4.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act4); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 4, nil
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
	require.NoError(err)
	require.True(9 == bc.TipHeight())
	// State may not be committed when the block is committed already
	time.Sleep(2 * time.Second)

	// check balance
	s, err = bc.Factory().AccountState(identityset.Address(28).String())
	require.Nil(err)
	change = s.Balance
	t.Logf("Alfa balance = %d", change)
	require.True(change.String() == "24")

	s, err = bc.Factory().AccountState(identityset.Address(29).String())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Bravo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "34")

	s, err = bc.Factory().AccountState(identityset.Address(30).String())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Charlie balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "46")

	s, err = bc.Factory().AccountState(identityset.Address(31).String())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Delta balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "70")

	s, err = bc.Factory().AccountState(identityset.Address(32).String())
	require.Nil(err)
	beta = s.Balance
	t.Logf("Echo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "101")

	s, err = bc.Factory().AccountState(identityset.Address(33).String())
	require.Nil(err)
	fox = s.Balance
	t.Logf("Foxtrot balance = %d", fox)
	change.Add(change, fox)
	require.True(fox.String() == "5242882")

	s, err = bc.Factory().AccountState(identityset.Address(27).String())
	require.Nil(err)
	test = s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)

	require.Equal(
		unit.ConvertIotxToRau(90000000),
		change,
	)
	t.Log("Total balance match")
}

func TestLocalSync(t *testing.T) {
	require := require.New(t)

	cfg, err := newTestConfig()
	require.Nil(err)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	testDBPath := testDBFile.Name()
	indexDBFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	indexDBPath := indexDBFile.Name()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = indexDBPath

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

	blk, err := bc.BlockDAO().GetBlockByHeight(1)
	require.Nil(err)
	hash1 := blk.HashBlock()
	blk, err = bc.BlockDAO().GetBlockByHeight(2)
	require.Nil(err)
	hash2 := blk.HashBlock()
	blk, err = bc.BlockDAO().GetBlockByHeight(3)
	require.Nil(err)
	hash3 := blk.HashBlock()
	blk, err = bc.BlockDAO().GetBlockByHeight(4)
	require.Nil(err)
	hash4 := blk.HashBlock()
	blk, err = bc.BlockDAO().GetBlockByHeight(5)
	require.Nil(err)
	hash5 := blk.HashBlock()
	require.NotNil(svr.P2PAgent())

	testDBFile2, _ := ioutil.TempFile(os.TempDir(), dBPath2)
	testDBPath2 := testDBFile2.Name()
	testTrieFile2, _ := ioutil.TempFile(os.TempDir(), triePath2)
	testTriePath2 := testTrieFile2.Name()
	indexDBFile2, _ := ioutil.TempFile(os.TempDir(), dBPath2)
	indexDBPath2 := indexDBFile2.Name()

	cfg, err = newTestConfig()
	require.Nil(err)
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = indexDBPath2

	// Create client
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
		blk1, err := cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(1)
		if err != nil {
			return false, nil
		}
		blk2, err := cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(2)
		if err != nil {
			return false, nil
		}
		blk3, err := cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(3)
		if err != nil {
			return false, nil
		}
		blk4, err := cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(4)
		if err != nil {
			return false, nil
		}
		blk5, err := cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(5)
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
	blk, err = cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(1)
	require.Nil(err)
	require.Equal(hash1, blk.HashBlock())
	blk, err = cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(2)
	require.Nil(err)
	require.Equal(hash2, blk.HashBlock())
	blk, err = cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(3)
	require.Nil(err)
	require.Equal(hash3, blk.HashBlock())
	blk, err = cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(4)
	require.Nil(err)
	require.Equal(hash4, blk.HashBlock())
	blk, err = cli.ChainService(chainID).Blockchain().BlockDAO().GetBlockByHeight(5)
	require.Nil(err)
	require.Equal(hash5, blk.HashBlock())
	t.Log("4 blocks received correctly")
}

func TestStartExistingBlockchain(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	testDBPath := testDBFile.Name()
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	testIndexPath := testIndexFile.Name()
	// Disable block reward to make bookkeeping easier
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()

	svr, err := itx.NewServer(cfg)
	require.Nil(err)
	require.NoError(svr.Start(ctx))
	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	require.NotNil(bc)

	defer func() {
		require.NoError(svr.Stop(ctx))
	}()

	require.NoError(addTestingTsfBlocks(bc))
	require.Equal(uint64(5), bc.TipHeight())

	// Delete state db and recover to tip
	testutil.CleanupPath(t, testTriePath)
	require.NoError(svr.Stop(ctx))
	require.NoError(svr.ChainService(cfg.Chain.ID).Blockchain().Start(ctx))
	// Refresh state DB
	require.NoError(bc.RecoverChainAndState(0))
	require.NoError(svr.ChainService(cfg.Chain.ID).Blockchain().Stop(ctx))
	svr, err = itx.NewServer(cfg)
	require.NoError(err)
	// Build states from height 1 to tip
	require.NoError(svr.Start(ctx))
	bc = svr.ChainService(chainID).Blockchain()
	height, _ := bc.Factory().Height()
	require.Equal(bc.TipHeight(), height)

	// Recover to height 3 from empty state DB
	testutil.CleanupPath(t, testTriePath)
	require.NoError(bc.RecoverChainAndState(3))
	require.NoError(svr.Stop(ctx))
	svr, err = itx.NewServer(cfg)
	require.NoError(err)
	// Build states from height 1 to 3
	require.NoError(svr.Start(ctx))
	bc = svr.ChainService(chainID).Blockchain()
	height, _ = bc.Factory().Height()
	require.Equal(bc.TipHeight(), height)
	require.Equal(uint64(3), height)

	// Recover to height 2 from an existing state DB with Height 3
	require.NoError(bc.RecoverChainAndState(2))
	require.NoError(svr.Stop(ctx))
	svr, err = itx.NewServer(cfg)
	require.NoError(err)
	// Build states from height 1 to 2
	require.NoError(svr.Start(ctx))
	bc = svr.ChainService(chainID).Blockchain()
	height, _ = bc.Factory().Height()
	require.Equal(bc.TipHeight(), height)
	require.Equal(uint64(2), height)
}

func newTestConfig() (config.Config, error) {
	cfg := config.Default
	cfg.Chain.TrieDBPath = triePath
	cfg.Chain.ChainDBPath = dBPath
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()
	cfg.API.Port = testutil.RandomPort()
	cfg.Genesis.EnableGravityChainVoting = false
	sk, err := crypto.GenerateKey()

	if err != nil {
		return config.Config{}, err
	}
	cfg.Chain.ProducerPrivKey = sk.HexString()
	return cfg, nil
}
