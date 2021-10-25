// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	dBPath     = "db.test"
	dBPath2    = "db.test2"
	triePath   = "trie.test"
	triePath2  = "trie.test2"
	disabledIP = "169.254."
)

func TestLocalCommit(t *testing.T) {
	require := require.New(t)

	cfg, err := newTestConfig()
	require.NoError(err)
	testTriePath, err := testutil.PathOfTempFile(triePath)
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile(dBPath)
	require.NoError(err)
	indexDBPath, err := testutil.PathOfTempFile(dBPath)
	require.NoError(err)
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = indexDBPath

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))
	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	sf := svr.ChainService(chainID).StateFactory()
	ap := svr.ChainService(chainID).ActionPool()
	require.NotNil(bc)
	require.NotNil(sf)
	require.NotNil(ap)

	i27State, err := accountutil.AccountState(sf, identityset.Address(27).String())
	require.NoError(err)
	require.NoError(addTestingTsfBlocks(bc, ap))
	require.NotNil(svr.ChainService(chainID).ActionPool())
	require.NotNil(svr.P2PAgent())

	// create client
	cfg, err = newTestConfig()
	require.NoError(err)
	addrs, err := svr.P2PAgent().Self()
	require.NoError(err)
	cfg.Network.BootstrapNodes = []string{validNetworkAddr(addrs)}
	p := p2p.NewAgent(
		cfg.Network,
		cfg.Genesis.Hash(),
		func(_ context.Context, _ uint32, _ string, _ proto.Message) {
		},
		func(_ context.Context, _ uint32, _ peerstore.PeerInfo, _ proto.Message) {
		},
	)
	require.NotNil(p)
	require.NoError(p.Start(ctx))

	defer func() {
		require.NoError(p.Stop(ctx))
		require.NoError(svr.Stop(ctx))
		require.NoError(bc.Stop(ctx))
	}()

	// check balance
	s, err := accountutil.AccountState(sf, identityset.Address(28).String())
	require.NoError(err)
	change := s.Balance
	t.Logf("Alfa balance = %d", change)
	require.True(change.String() == "23")

	s, err = accountutil.AccountState(sf, identityset.Address(29).String())
	require.NoError(err)
	beta := s.Balance
	t.Logf("Bravo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "34")

	s, err = accountutil.AccountState(sf, identityset.Address(30).String())
	require.NoError(err)
	beta = s.Balance
	t.Logf("Charlie balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "47")

	s, err = accountutil.AccountState(sf, identityset.Address(31).String())
	require.NoError(err)
	beta = s.Balance
	t.Logf("Delta balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "69")

	s, err = accountutil.AccountState(sf, identityset.Address(32).String())
	require.NoError(err)
	beta = s.Balance
	t.Logf("Echo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "100")

	s, err = accountutil.AccountState(sf, identityset.Address(33).String())
	require.NoError(err)
	fox := s.Balance
	t.Logf("Foxtrot balance = %d", fox)
	change.Add(change, fox)
	require.True(fox.String() == "5242883")

	s, err = accountutil.AccountState(sf, identityset.Address(27).String())
	require.NoError(err)
	test := s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)
	change.Sub(change, i27State.Balance)

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
	testTriePath2, err := testutil.PathOfTempFile(triePath2)
	require.NoError(err)
	testDBPath2, err := testutil.PathOfTempFile(dBPath2)
	require.NoError(err)
	indexDBPath2, err := testutil.PathOfTempFile(dBPath2)
	require.NoError(err)
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = indexDBPath2
	require.NoError(copyDB(testTriePath, testTriePath2))
	require.NoError(copyDB(testDBPath, testDBPath2))
	require.NoError(copyDB(indexDBPath, indexDBPath2))
	registry := protocol.NewRegistry()
	sf2, err := factory.NewStateDB(cfg, factory.CachedStateDBOption(), factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap2, err := actpool.NewActPool(sf2, cfg.ActPool)
	require.NoError(err)
	chain := blockchain.NewBlockchain(
		cfg,
		nil,
		factory.NewMinter(sf2, ap2),
		blockchain.BoltDBDaoOption(sf2),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf2,
			protocol.NewGenericValidator(sf2, accountutil.AccountState),
		)),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	require.NoError(rolldposProtocol.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(rewardingProtocol.Register(registry))
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	require.NoError(chain.Start(ctx))
	require.EqualValues(5, chain.TipHeight())
	defer func() {
		require.NoError(chain.Stop(ctx))
	}()

	p2pCtx := p2p.WitContext(ctx, p2p.Context{ChainID: cfg.Chain.ID})
	// transfer 1
	// C --> A
	s, _ = accountutil.AccountState(sf, identityset.Address(30).String())
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(30), s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(context.Background(), tsf1))
	act1 := tsf1.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act1); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 1, nil
	})
	require.NoError(err)

	blk1, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(chain.CommitBlock(blk1))

	// transfer 2
	// F --> D
	s, _ = accountutil.AccountState(sf, identityset.Address(33).String())
	tsf2, err := action.SignedTransfer(identityset.Address(31).String(), identityset.PrivateKey(33), s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(context.Background(), tsf2))
	blk2, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(chain.CommitBlock(blk2))
	// broadcast to P2P
	act2 := tsf2.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act2); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 2, nil
	})
	require.NoError(err)

	// transfer 3
	// B --> B
	s, _ = accountutil.AccountState(sf, identityset.Address(29).String())
	tsf3, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(context.Background(), tsf3))
	blk3, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(chain.CommitBlock(blk3))
	// broadcast to P2P
	act3 := tsf3.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act3); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 3, nil
	})
	require.NoError(err)

	// transfer 4
	// test --> E
	s, _ = accountutil.AccountState(sf, identityset.Address(27).String())
	tsf4, err := action.SignedTransfer(identityset.Address(32).String(), identityset.PrivateKey(27), s.Nonce+1, big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(context.Background(), tsf4))
	blk4, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(chain.CommitBlock(blk4))
	// broadcast to P2P
	act4 := tsf4.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(p2pCtx, act4); err != nil {
			return false, err
		}
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 4, nil
	})
	require.NoError(err)
	// wait 4 blocks being picked and committed
	require.NoError(p.BroadcastOutbound(p2pCtx, blk2.ConvertToBlockPb()))
	require.NoError(p.BroadcastOutbound(p2pCtx, blk4.ConvertToBlockPb()))
	require.NoError(p.BroadcastOutbound(p2pCtx, blk1.ConvertToBlockPb()))
	require.NoError(p.BroadcastOutbound(p2pCtx, blk3.ConvertToBlockPb()))
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 9, nil
	})
	require.NoError(err)
	require.True(9 == bc.TipHeight())
	// State may not be committed when the block is committed already
	time.Sleep(2 * time.Second)

	// check balance
	s, err = accountutil.AccountState(sf, identityset.Address(28).String())
	require.NoError(err)
	change = s.Balance
	t.Logf("Alfa balance = %d", change)
	require.True(change.String() == "24")

	s, err = accountutil.AccountState(sf, identityset.Address(29).String())
	require.NoError(err)
	beta = s.Balance
	t.Logf("Bravo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "34")

	s, err = accountutil.AccountState(sf, identityset.Address(30).String())
	require.NoError(err)
	beta = s.Balance
	t.Logf("Charlie balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "46")

	s, err = accountutil.AccountState(sf, identityset.Address(31).String())
	require.NoError(err)
	beta = s.Balance
	t.Logf("Delta balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "70")

	s, err = accountutil.AccountState(sf, identityset.Address(32).String())
	require.NoError(err)
	beta = s.Balance
	t.Logf("Echo balance = %d", beta)
	change.Add(change, beta)
	require.True(beta.String() == "101")

	s, err = accountutil.AccountState(sf, identityset.Address(33).String())
	require.NoError(err)
	fox = s.Balance
	t.Logf("Foxtrot balance = %d", fox)
	change.Add(change, fox)
	require.True(fox.String() == "5242882")

	s, err = accountutil.AccountState(sf, identityset.Address(27).String())
	require.NoError(err)
	test = s.Balance
	t.Logf("test balance = %d", test)
	change.Add(change, test)
	change.Sub(change, i27State.Balance)

	require.Equal(
		unit.ConvertIotxToRau(90000000),
		change,
	)
	t.Log("Total balance match")
}

func TestLocalSync(t *testing.T) {
	require := require.New(t)

	cfg, err := newTestConfig()
	require.NoError(err)
	testTriePath, err := testutil.PathOfTempFile(triePath)
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile(dBPath)
	require.NoError(err)
	indexDBPath, err := testutil.PathOfTempFile(dBPath)
	require.NoError(err)
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = indexDBPath

	// Create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))

	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	sf := svr.ChainService(chainID).StateFactory()
	ap := svr.ChainService(chainID).ActionPool()
	dao := svr.ChainService(chainID).BlockDAO()
	require.NotNil(bc)
	require.NotNil(sf)
	require.NotNil(ap)
	require.NotNil(dao)
	require.NotNil(svr.P2PAgent())
	require.NoError(addTestingTsfBlocks(bc, ap))

	blk, err := dao.GetBlockByHeight(1)
	require.NoError(err)
	hash1 := blk.HashBlock()
	blk, err = dao.GetBlockByHeight(2)
	require.NoError(err)
	hash2 := blk.HashBlock()
	blk, err = dao.GetBlockByHeight(3)
	require.NoError(err)
	hash3 := blk.HashBlock()
	blk, err = dao.GetBlockByHeight(4)
	require.NoError(err)
	hash4 := blk.HashBlock()
	blk, err = dao.GetBlockByHeight(5)
	require.NoError(err)
	hash5 := blk.HashBlock()
	require.NotNil(svr.P2PAgent())

	testDBPath2, err := testutil.PathOfTempFile(dBPath2)
	require.NoError(err)
	testTriePath2, err := testutil.PathOfTempFile(triePath2)
	require.NoError(err)
	indexDBPath2, err := testutil.PathOfTempFile(dBPath2)
	require.NoError(err)

	cfg, err = newTestConfig()
	require.NoError(err)
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = indexDBPath2

	addrs, err := svr.P2PAgent().Self()
	require.NoError(err)
	// Create client
	cfg.Network.BootstrapNodes = []string{validNetworkAddr(addrs)}

	cfg.BlockSync.Interval = 1 * time.Second
	cli, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(cli.Start(ctx))
	require.NotNil(cli.ChainService(chainID).Blockchain())
	require.NotNil(cli.P2PAgent())

	defer func() {
		require.NoError(cli.Stop(ctx))
		require.NoError(svr.Stop(ctx))
	}()

	err = testutil.WaitUntil(time.Millisecond*100, time.Second*60, func() (bool, error) {
		peers, err := svr.P2PAgent().Neighbors(context.Background())
		return len(peers) >= 1, err
	})
	require.NoError(err)

	err = svr.P2PAgent().BroadcastOutbound(
		p2p.WitContext(ctx, p2p.Context{ChainID: cfg.Chain.ID}),
		blk.ConvertToBlockPb(),
	)
	require.NoError(err)
	check := testutil.CheckCondition(func() (bool, error) {
		blk1, err := cli.ChainService(chainID).BlockDAO().GetBlockByHeight(1)
		if err != nil {
			return false, nil
		}
		blk2, err := cli.ChainService(chainID).BlockDAO().GetBlockByHeight(2)
		if err != nil {
			return false, nil
		}
		blk3, err := cli.ChainService(chainID).BlockDAO().GetBlockByHeight(3)
		if err != nil {
			return false, nil
		}
		blk4, err := cli.ChainService(chainID).BlockDAO().GetBlockByHeight(4)
		if err != nil {
			return false, nil
		}
		blk5, err := cli.ChainService(chainID).BlockDAO().GetBlockByHeight(5)
		if err != nil {
			return false, nil
		}
		return hash1 == blk1.HashBlock() &&
			hash2 == blk2.HashBlock() &&
			hash3 == blk3.HashBlock() &&
			hash4 == blk4.HashBlock() &&
			hash5 == blk5.HashBlock(), nil
	})
	require.NoError(testutil.WaitUntil(time.Millisecond*100, time.Second*60, check))

	// verify 4 received blocks
	blk, err = cli.ChainService(chainID).BlockDAO().GetBlockByHeight(1)
	require.NoError(err)
	require.Equal(hash1, blk.HashBlock())
	blk, err = cli.ChainService(chainID).BlockDAO().GetBlockByHeight(2)
	require.NoError(err)
	require.Equal(hash2, blk.HashBlock())
	blk, err = cli.ChainService(chainID).BlockDAO().GetBlockByHeight(3)
	require.NoError(err)
	require.Equal(hash3, blk.HashBlock())
	blk, err = cli.ChainService(chainID).BlockDAO().GetBlockByHeight(4)
	require.NoError(err)
	require.Equal(hash4, blk.HashBlock())
	blk, err = cli.ChainService(chainID).BlockDAO().GetBlockByHeight(5)
	require.NoError(err)
	require.Equal(hash5, blk.HashBlock())
	t.Log("4 blocks received correctly")
}

func TestStartExistingBlockchain(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	testDBPath, err := testutil.PathOfTempFile(dBPath)
	require.NoError(err)
	testTriePath, err := testutil.PathOfTempFile(triePath)
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile(dBPath)
	require.NoError(err)
	// Disable block reward to make bookkeeping easier
	cfg := config.Default
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()

	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))
	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	sf := svr.ChainService(chainID).StateFactory()
	ap := svr.ChainService(chainID).ActionPool()
	dao := svr.ChainService(chainID).BlockDAO()
	require.NotNil(bc)
	require.NotNil(sf)
	require.NotNil(ap)
	require.NotNil(dao)

	defer func() {
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
		testutil.CleanupPath(t, testIndexPath)
	}()

	require.NoError(addTestingTsfBlocks(bc, ap))
	require.Equal(uint64(5), bc.TipHeight())

	require.NoError(svr.Stop(ctx))
	// Delete state db and recover to tip
	testutil.CleanupPath(t, testTriePath)

	require.NoError(svr.ChainService(cfg.Chain.ID).Blockchain().Start(ctx))
	height, _ := svr.ChainService(cfg.Chain.ID).StateFactory().Height()
	require.Equal(bc.TipHeight(), height)
	require.Equal(uint64(5), height)
	require.NoError(svr.ChainService(cfg.Chain.ID).Blockchain().Stop(ctx))

	// Recover to height 3 from empty state DB
	cfg.DB.DbPath = cfg.Chain.ChainDBPath
	cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
	dao = blockdao.NewBlockDAO(nil, cfg.DB)
	require.NoError(dao.Start(genesis.WithGenesisContext(ctx, cfg.Genesis)))
	require.NoError(dao.DeleteBlockToTarget(3))
	require.NoError(dao.Stop(ctx))

	// Build states from height 1 to 3
	testutil.CleanupPath(t, testTriePath)
	svr, err = itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))
	bc = svr.ChainService(chainID).Blockchain()
	sf = svr.ChainService(chainID).StateFactory()
	dao = svr.ChainService(chainID).BlockDAO()
	height, _ = sf.Height()
	require.Equal(bc.TipHeight(), height)
	require.Equal(uint64(3), height)

	// Recover to height 2 from an existing state DB with Height 3
	require.NoError(svr.Stop(ctx))
	cfg.DB.DbPath = cfg.Chain.ChainDBPath
	cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
	dao = blockdao.NewBlockDAO(nil, cfg.DB)
	require.NoError(dao.Start(genesis.WithGenesisContext(ctx, cfg.Genesis)))
	require.NoError(dao.DeleteBlockToTarget(2))
	require.NoError(dao.Stop(ctx))
	testutil.CleanupPath(t, testTriePath)
	svr, err = itx.NewServer(cfg)
	require.NoError(err)
	// Build states from height 1 to 2
	require.NoError(svr.Start(ctx))
	bc = svr.ChainService(chainID).Blockchain()
	sf = svr.ChainService(chainID).StateFactory()
	height, _ = sf.Height()
	require.Equal(bc.TipHeight(), height)
	require.Equal(uint64(2), height)
	require.NoError(svr.Stop(ctx))
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

func validNetworkAddr(addrs []multiaddr.Multiaddr) (ret string) {
	for _, addr := range addrs {
		if !strings.Contains(addr.String(), disabledIP) {
			return addr.String()
		}
	}
	return
}
