// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mohae/deepcopy"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/p2p"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

const (
	_dBPath     = "db.test"
	_dBPath2    = "db.test2"
	_triePath   = "trie.test"
	_triePath2  = "trie.test2"
	_blobPath   = "blob.test"
	_disabledIP = "169.254."
)

func TestLocalCommit(t *testing.T) {
	require := require.New(t)

	cfg, err := newTestConfig()
	require.NoError(err)
	initDBPaths(require, &cfg)
	defer func() {
		clearDBPaths(&cfg)
	}()
	testTriePath := cfg.Chain.TrieDBPath
	testDBPath := cfg.Chain.ChainDBPath
	indexDBPath := cfg.Chain.IndexDBPath
	cfg.ActPool.Store = nil

	// create server
	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))
	defer func() {
		require.NoError(svr.Stop(ctx))
	}()
	cs := svr.ChainService(cfg.Chain.ID)
	bc := cs.Blockchain()
	sf := cs.StateFactory()
	ap := cs.ActionPool()
	require.NotNil(bc)
	require.NotNil(sf)
	require.NotNil(ap)

	i27State, err := accountutil.AccountState(ctx, sf, identityset.Address(27))
	require.NoError(err)
	require.NoError(addTestingTsfBlocks(bc, ap))
	require.EqualValues(5, bc.TipHeight())

	// check balance
	change := &big.Int{}
	for _, v := range []struct {
		addrIndex int
		balance   string
	}{
		{28, "23"},
		{29, "34"},
		{30, "47"},
		{31, "69"},
		{32, "100"},
		{33, "5242883"},
	} {
		s, err := accountutil.AccountState(ctx, sf, identityset.Address(v.addrIndex))
		require.NoError(err)
		require.Equal(v.balance, s.Balance.String())
		change.Add(change, s.Balance)
	}
	s, err := accountutil.AccountState(ctx, sf, identityset.Address(27))
	require.NoError(err)
	change.Add(change, s.Balance)
	change.Sub(change, i27State.Balance)
	require.Equal(unit.ConvertIotxToRau(90000000), change)

	// create client
	cfg, err = newTestConfig()
	require.NoError(err)
	initDBPaths(require, &cfg)
	addrs, err := svr.P2PAgent().Self()
	require.NoError(err)
	cfg.Network.BootstrapNodes = []string{validNetworkAddr(addrs)}
	p := p2p.NewAgent(
		cfg.Network,
		cfg.Chain.ID,
		cfg.Genesis.Hash(),
		func(proto.Message) (bool, error) {
			return false, nil
		},
		func(_ context.Context, _ uint32, _ string, _ proto.Message) {
		},
		func(_ context.Context, _ uint32, _ peer.AddrInfo, _ proto.Message) {
		},
	)
	require.NotNil(p)
	require.NoError(p.Start(ctx))
	defer func() {
		require.NoError(p.Stop(ctx))
	}()

	// create local chain
	testTriePath2, err := testutil.PathOfTempFile(_triePath2)
	require.NoError(err)
	testDBPath2, err := testutil.PathOfTempFile(_dBPath2)
	require.NoError(err)
	indexDBPath2, err := testutil.PathOfTempFile(_dBPath2)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testTriePath2)
		testutil.CleanupPath(testDBPath2)
		testutil.CleanupPath(indexDBPath2)
	}()
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = indexDBPath2
	require.NoError(copyDB(testTriePath, testTriePath2))
	require.NoError(copyDB(testDBPath, testDBPath2))
	require.NoError(copyDB(indexDBPath, indexDBPath2))
	registry := protocol.NewRegistry()
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	db1, err := db.CreateKVStoreWithCache(cfg.DB, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sf2, err := factory.NewStateDB(factoryCfg, db1, factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap2, err := actpool.NewActPool(cfg.Genesis, sf2, cfg.ActPool)
	require.NoError(err)
	dbcfg := cfg.DB
	dbcfg.DbPath = cfg.Chain.ChainDBPath
	store, err := filedao.NewFileDAO(dbcfg, block.NewDeserializer(cfg.Chain.EVMNetworkID))
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf2}, dbcfg.MaxCacheSize)
	chain := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf2, ap2),
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

	// transfer 1
	// C --> A
	s, _ = accountutil.AccountState(ctx, sf, identityset.Address(30))
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(30), s.PendingNonce(), big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(context.Background(), tsf1))
	act1 := tsf1.Proto()
	err = testutil.WaitUntil(10*time.Millisecond, 2*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(ctx, act1); err != nil {
			return false, err
		}
		acts := ap.PendingActionMap()
		return lenPendingActionMap(acts) == 1, nil
	})
	require.NoError(err)

	blk1, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(chain.CommitBlock(blk1))

	// transfer 2
	// F --> D
	s, _ = accountutil.AccountState(ctx, sf, identityset.Address(33))
	tsf2, err := action.SignedTransfer(identityset.Address(31).String(), identityset.PrivateKey(33), s.PendingNonce(), big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(context.Background(), tsf2))
	blk2, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(chain.CommitBlock(blk2))
	// broadcast to P2P
	act2 := tsf2.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(ctx, act2); err != nil {
			return false, err
		}
		acts := ap.PendingActionMap()
		return lenPendingActionMap(acts) == 2, nil
	})
	require.NoError(err)

	// transfer 3
	// B --> B
	s, _ = accountutil.AccountState(ctx, sf, identityset.Address(29))
	tsf3, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), s.PendingNonce(), big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(context.Background(), tsf3))
	blk3, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(chain.CommitBlock(blk3))
	// broadcast to P2P
	act3 := tsf3.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(ctx, act3); err != nil {
			return false, err
		}
		acts := ap.PendingActionMap()
		return lenPendingActionMap(acts) == 3, nil
	})
	require.NoError(err)

	// transfer 4
	// test --> E
	s, _ = accountutil.AccountState(ctx, sf, identityset.Address(27))
	tsf4, err := action.SignedTransfer(identityset.Address(32).String(), identityset.PrivateKey(27), s.PendingNonce(), big.NewInt(1), []byte{}, 100000, big.NewInt(0))
	require.NoError(err)

	require.NoError(ap2.Add(context.Background(), tsf4))
	blk4, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(chain.CommitBlock(blk4))
	// broadcast to P2P
	act4 := tsf4.Proto()
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		if err := p.BroadcastOutbound(ctx, act4); err != nil {
			return false, err
		}
		acts := ap.PendingActionMap()
		return lenPendingActionMap(acts) == 4, nil
	})
	require.NoError(err)
	// wait 4 blocks being picked and committed
	require.NoError(p.BroadcastOutbound(ctx, blk2.ConvertToBlockPb()))
	require.NoError(p.BroadcastOutbound(ctx, blk4.ConvertToBlockPb()))
	require.NoError(p.BroadcastOutbound(ctx, blk1.ConvertToBlockPb()))
	require.NoError(p.BroadcastOutbound(ctx, blk3.ConvertToBlockPb()))
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		height := bc.TipHeight()
		return int(height) == 9, nil
	})
	require.NoError(err)
	require.True(9 == bc.TipHeight())

	// check balance
	change.SetBytes(nil)
	for _, v := range []struct {
		addrIndex int
		balance   string
	}{
		{28, "24"},
		{29, "34"},
		{30, "46"},
		{31, "70"},
		{32, "101"},
		{33, "5242882"},
	} {
		s, err = accountutil.AccountState(ctx, sf, identityset.Address(v.addrIndex))
		require.NoError(err)
		require.Equal(v.balance, s.Balance.String())
		change.Add(change, s.Balance)
	}
	s, err = accountutil.AccountState(ctx, sf, identityset.Address(27))
	require.NoError(err)
	change.Add(change, s.Balance)
	change.Sub(change, i27State.Balance)
	require.Equal(unit.ConvertIotxToRau(90000000), change)
}

func TestLocalSync(t *testing.T) {
	require := require.New(t)

	cfg, err := newTestConfig()
	require.NoError(err)
	initDBPaths(require, &cfg)
	defer func() {
		clearDBPaths(&cfg)
	}()

	// bootnode
	ctx := context.Background()
	bootnodePort := testutil.RandomPort()
	bootnode := p2p.NewAgent(p2p.Config{
		Host:              "127.0.0.1",
		Port:              bootnodePort,
		ReconnectInterval: 150 * time.Second},
		cfg.Chain.ID,
		hash.ZeroHash256,
		func(proto.Message) (bool, error) {
			return false, nil
		},
		func(_ context.Context, _ uint32, _ string, msg proto.Message) {},
		func(_ context.Context, _ uint32, _ peer.AddrInfo, _ proto.Message) {})
	require.NoError(bootnode.Start(ctx))
	addrs, err := bootnode.Self()
	require.NoError(err)
	cfg.Network.BootstrapNodes = []string{validNetworkAddr(addrs)}

	// Create server
	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))

	sChain := svr.ChainService(cfg.Chain.ID)
	bc := sChain.Blockchain()
	sf := sChain.StateFactory()
	ap := sChain.ActionPool()
	dao := sChain.BlockDAO()
	require.NotNil(bc)
	require.NotNil(sf)
	require.NotNil(ap)
	require.NotNil(dao)
	require.NotNil(svr.P2PAgent())
	require.NoError(addTestingTsfBlocks(bc, ap))

	var blkHash [5]hash.Hash256
	for i := uint64(1); i <= 5; i++ {
		blk, err := dao.GetBlockByHeight(i)
		require.NoError(err)
		blkHash[i-1] = blk.HashBlock()
	}

	testDBPath2, err := testutil.PathOfTempFile(_dBPath2)
	require.NoError(err)
	testTriePath2, err := testutil.PathOfTempFile(_triePath2)
	require.NoError(err)
	indexDBPath2, err := testutil.PathOfTempFile(_dBPath2)
	require.NoError(err)
	blobIndexPath, err := testutil.PathOfTempFile(_blobPath)
	require.NoError(err)
	contractIndexDBPath2, err := testutil.PathOfTempFile(_dBPath2)
	require.NoError(err)

	cfg, err = newTestConfig()
	require.NoError(err)
	initDBPaths(require, &cfg)
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.BlobStoreDBPath = ""
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = indexDBPath2
	cfg.Chain.BlobStoreDBPath = blobIndexPath
	cfg.Chain.ContractStakingIndexDBPath = contractIndexDBPath2
	defer func() {
		testutil.CleanupPath(testTriePath2)
		testutil.CleanupPath(testDBPath2)
		testutil.CleanupPath(indexDBPath2)
		testutil.CleanupPath(blobIndexPath)
		testutil.CleanupPath(contractIndexDBPath2)
	}()

	// Create client
	cfg.Network.BootstrapNodes = []string{validNetworkAddr(addrs)}
	cfg.BlockSync.Interval = 1 * time.Second
	cli, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(cli.Start(ctx))
	nChain := cli.ChainService(cfg.Chain.ID)
	require.NotNil(nChain.Blockchain())
	require.NotNil(cli.P2PAgent())
	require.Zero(nChain.Blockchain().TipHeight())
	defer func() {
		require.NoError(cli.Stop(ctx))
		require.NoError(svr.Stop(ctx))
	}()

	err = testutil.WaitUntil(time.Millisecond*100, time.Second*60, func() (bool, error) {
		peers, err := svr.P2PAgent().ConnectedPeers()
		return len(peers) >= 1, err
	})
	require.NoError(err)
	nDao := nChain.BlockDAO()
	check := testutil.CheckCondition(func() (bool, error) {
		for i := uint64(1); i <= 5; i++ {
			blk, err := nDao.GetBlockByHeight(i)
			if err != nil {
				return false, nil
			}
			if blk.HashBlock() != blkHash[i-1] {
				return false, nil
			}
		}
		return true, nil
	})
	require.NoError(testutil.WaitUntil(time.Millisecond*100, time.Second*60, check))
	require.EqualValues(5, nChain.Blockchain().TipHeight())
}

func TestStartExistingBlockchain(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	testDBPath, err := testutil.PathOfTempFile(_dBPath)
	require.NoError(err)
	testTriePath, err := testutil.PathOfTempFile(_triePath)
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile(_dBPath)
	require.NoError(err)
	testContractStakeIndexPath, err := testutil.PathOfTempFile(_dBPath)
	require.NoError(err)
	testBlobIndexPath, err := testutil.PathOfTempFile(_blobPath)
	require.NoError(err)
	// Disable block reward to make bookkeeping easier
	cfg := config.Default
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.BlobStoreDBPath = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.BlobStoreDBPath = testBlobIndexPath
	cfg.Chain.ContractStakingIndexDBPath = testContractStakeIndexPath
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.ActPool.Store = nil
	cfg.Genesis = genesis.TestDefault()
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()

	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))
	cs := svr.ChainService(cfg.Chain.ID)
	bc := cs.Blockchain()
	ap := cs.ActionPool()
	require.NotNil(bc)
	require.NotNil(cs.StateFactory())
	require.NotNil(ap)
	require.NotNil(cs.BlockDAO())

	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
		testutil.CleanupPath(testBlobIndexPath)
		testutil.CleanupPath(testContractStakeIndexPath)
	}()

	require.NoError(addTestingTsfBlocks(bc, ap))
	require.Equal(uint64(5), bc.TipHeight())

	require.NoError(svr.Stop(ctx))
	// Delete state db and recover to tip
	testutil.CleanupPath(testTriePath)
	testutil.CleanupPath(testContractStakeIndexPath)

	require.NoError(cs.Blockchain().Start(ctx))
	height, _ := cs.StateFactory().Height()
	require.Equal(bc.TipHeight(), height)
	require.Equal(uint64(5), height)
	require.NoError(cs.Blockchain().Stop(ctx))

	// Recover to height 3 from empty state DB
	cfg.DB.DbPath = cfg.Chain.ChainDBPath
	deser := block.NewDeserializer(cfg.Chain.EVMNetworkID)
	dao, err := filedao.NewFileDAO(cfg.DB, deser)
	require.NoError(err)
	require.NoError(dao.Start(protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, cfg.Genesis),
		protocol.BlockchainCtx{
			ChainID: cfg.Chain.ID,
		})))
	for {
		height, err := dao.Height()
		require.NoError(err)
		if height <= 3 {
			break
		}
		require.NoError(dao.DeleteTipBlock())
	}
	require.NoError(dao.Stop(ctx))

	// Build states from height 1 to 3
	testutil.CleanupPath(testTriePath)
	testutil.CleanupPath(testContractStakeIndexPath)
	svr, err = itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))
	cs = svr.ChainService(cfg.Chain.ID)
	height, _ = cs.StateFactory().Height()
	require.Equal(cs.Blockchain().TipHeight(), height)
	require.Equal(uint64(3), height)

	// Recover to height 2 from an existing state DB with Height 3
	require.NoError(svr.Stop(ctx))
	cfg.DB.DbPath = cfg.Chain.ChainDBPath
	dao, err = filedao.NewFileDAO(cfg.DB, deser)
	require.NoError(err)
	require.NoError(dao.Start(protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, cfg.Genesis),
		protocol.BlockchainCtx{
			ChainID: cfg.Chain.ID,
		})))
	for {
		height, err := dao.Height()
		require.NoError(err)
		if height <= 2 {
			break
		}
		require.NoError(dao.DeleteTipBlock())
	}
	require.NoError(dao.Stop(ctx))
	testutil.CleanupPath(testTriePath)
	testutil.CleanupPath(testContractStakeIndexPath)
	svr, err = itx.NewServer(cfg)
	require.NoError(err)
	// Build states from height 1 to 2
	require.NoError(svr.Start(ctx))
	cs = svr.ChainService(cfg.Chain.ID)
	height, _ = cs.StateFactory().Height()
	require.Equal(cs.Blockchain().TipHeight(), height)
	require.Equal(uint64(2), height)
	require.NoError(svr.Stop(ctx))
}

func newTestConfig() (config.Config, error) {
	cfg := config.Default
	cfg = deepcopy.Copy(cfg).(config.Config)
	cfg.Genesis = genesis.TestDefault()
	cfg.Chain.TrieDBPath = _triePath
	cfg.Chain.ChainDBPath = _dBPath
	cfg.Chain.BlobStoreDBPath = _blobPath
	cfg.Chain.MintTimeout = 0
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = testutil.RandomPort()
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Genesis.MidwayBlockHeight = 1
	sk, err := crypto.GenerateKey()

	if err != nil {
		return config.Config{}, err
	}
	cfg.Chain.ProducerPrivKey = sk.HexString()
	return cfg, nil
}

func validNetworkAddr(addrs []multiaddr.Multiaddr) (ret string) {
	for _, addr := range addrs {
		if !strings.Contains(addr.String(), _disabledIP) {
			return addr.String()
		}
	}
	return
}
