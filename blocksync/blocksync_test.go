// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/actpool"
	bc "github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_blocksync"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestSyncTaskInterval(t *testing.T) {
	assert := assert.New(t)

	interval := time.Duration(0)

	cfgLightWeight := &config.Config{
		NodeType: config.LightweightType,
	}
	lightWeight := syncTaskInterval(cfgLightWeight)
	assert.Equal(interval, lightWeight)

	cfgDelegate := &config.Config{
		NodeType: config.DelegateType,
		BlockSync: config.BlockSync{
			Interval: interval,
		},
	}
	delegate := syncTaskInterval(cfgDelegate)
	assert.Equal(interval, delegate)

	cfgFullNode := &config.Config{
		NodeType: config.FullNodeType,
		BlockSync: config.BlockSync{
			Interval: interval,
		},
	}
	interval <<= 2
	fullNode := syncTaskInterval(cfgFullNode)
	assert.Equal(interval, fullNode)
}

func generateP2P() network.Overlay {
	c := &config.Network{
		Host: "127.0.0.1",
		Port: 10001,
		MsgLogsCleaningInterval: 2 * time.Second,
		MsgLogRetention:         10 * time.Second,
		HealthCheckInterval:     time.Second,
		SilentInterval:          5 * time.Second,
		PeerMaintainerInterval:  time.Second,
		NumPeersLowerBound:      5,
		NumPeersUpperBound:      5,
		AllowMultiConnsPerHost:  true,
		RateLimitEnabled:        false,
		PingInterval:            time.Second,
		BootstrapNodes:          []string{"127.0.0.1:10001", "127.0.0.1:10002"},
		MaxMsgSize:              1024 * 1024 * 10,
		PeerDiscovery:           true,
	}
	return network.NewOverlay(c)
}

func TestNewBlockSyncer(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	p2p := generateP2P()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	// TipHeight return ERROR
	mBc.EXPECT().TipHeight().AnyTimes().Return(uint64(0))
	mBc.EXPECT().ChainID().AnyTimes().Return(iotxaddress.MainChainID())
	blk := bc.NewBlock(uint32(123), uint64(0), hash.Hash32B{}, clock.New(), nil, nil, nil)
	mBc.EXPECT().GetBlockByHeight(gomock.Any()).AnyTimes().Return(blk, nil)

	cfg, err := newTestConfig()
	require.Nil(err)
	ap, err := actpool.NewActPool(mBc, cfg.ActPool)
	assert.NoError(err)

	// Lightweight
	cfgLightWeight := &config.Config{
		NodeType: config.LightweightType,
	}

	bsLightWeight, err := NewBlockSyncer(cfgLightWeight, mBc, ap, p2p)
	assert.NoError(err)
	assert.NotNil(bsLightWeight)

	// Delegate
	cfgDelegate := &config.Config{
		NodeType: config.DelegateType,
	}
	cfgDelegate.Network.BootstrapNodes = []string{"123"}

	_, err = NewBlockSyncer(cfgDelegate, mBc, ap, p2p)
	assert.Nil(err)

	// FullNode
	cfgFullNode := &config.Config{
		NodeType: config.FullNodeType,
	}
	cfgFullNode.Network.BootstrapNodes = []string{"123"}

	bs, err := NewBlockSyncer(cfgFullNode, mBc, ap, p2p)
	assert.Nil(err)
	assert.Equal(p2p, bs.P2P())
}

func TestBlockSyncerStart(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mBs := mock_blocksync.NewMockBlockSync(ctrl)
	mBs.EXPECT().Start(gomock.Any()).Times(1)
	assert.Nil(mBs.Start(ctx))
}

func TestBlockSyncerStop(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mBs := mock_blocksync.NewMockBlockSync(ctrl)
	mBs.EXPECT().Stop(gomock.Any()).Times(1)
	assert.Nil(mBs.Stop(ctx))
}

func TestBlockSyncerProcessSyncRequest(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	mBc.EXPECT().ChainID().AnyTimes().Return(iotxaddress.MainChainID())
	blk := bc.NewBlock(uint32(123), uint64(0), hash.Hash32B{}, clock.New(), nil, nil, nil)
	mBc.EXPECT().GetBlockByHeight(gomock.Any()).AnyTimes().Return(blk, nil)
	mBc.EXPECT().TipHeight().AnyTimes().Return(uint64(0))
	cfg, err := newTestConfig()
	require.Nil(err)
	ap, err := actpool.NewActPool(mBc, cfg.ActPool)
	assert.NoError(err)
	p2p := generateP2P()

	cfgFullNode := &config.Config{
		NodeType: config.FullNodeType,
	}
	cfgFullNode.Network.BootstrapNodes = []string{"123"}

	bs, err := NewBlockSyncer(cfgFullNode, mBc, ap, p2p)
	assert.Nil(err)

	pbBs := &pb.BlockSync{
		Start: 1,
		End:   1,
	}

	bs.(*blockSyncer).ackSyncReq = false
	assert.Nil(bs.ProcessSyncRequest("", pbBs))
	bs.(*blockSyncer).ackSyncReq = true
	assert.Nil(bs.ProcessSyncRequest("", pbBs))
}

func TestBlockSyncerProcessSyncRequestError(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)

	chain := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	require.NoError(chain.Start(ctx))
	require.NotNil(chain)
	ap, err := actpool.NewActPool(chain, cfg.ActPool)
	require.NotNil(ap)
	require.NoError(err)
	bs, err := NewBlockSyncer(cfg, chain, ap, network.NewOverlay(&cfg.Network))
	require.Nil(err)

	defer func() {
		require.Nil(chain.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	}()

	pbBs := &pb.BlockSync{
		Start: 1,
		End:   5,
	}

	bs.(*blockSyncer).ackSyncReq = true
	require.Error(bs.ProcessSyncRequest("", pbBs))
}

func TestBlockSyncerProcessBlockTipHeight(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)

	chain := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	require.NoError(chain.Start(ctx))
	require.NotNil(chain)
	ap, err := actpool.NewActPool(chain, cfg.ActPool)
	require.NotNil(ap)
	require.NoError(err)
	bs, err := NewBlockSyncer(cfg, chain, ap, network.NewOverlay(&cfg.Network))
	require.Nil(err)

	defer func() {
		require.Nil(chain.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	}()

	h := chain.TipHeight()
	blk, err := chain.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk)
	require.NoError(err)
	bs.(*blockSyncer).ackBlockCommit = false
	require.Nil(bs.ProcessBlock(blk))
	h2 := chain.TipHeight()
	assert.Equal(t, h, h2)

	// commit top
	bs.(*blockSyncer).ackBlockCommit = true
	require.Nil(bs.ProcessBlock(blk))
	h3 := chain.TipHeight()
	assert.Equal(t, h+1, h3)

	// commit same block again
	require.Nil(bs.ProcessBlock(blk))
	h4 := chain.TipHeight()
	assert.Equal(t, h3, h4)
}

func TestBlockSyncerProcessBlockOutOfOrder(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)

	chain1 := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	require.NoError(chain1.Start(ctx))
	require.NotNil(chain1)
	ap1, err := actpool.NewActPool(chain1, cfg.ActPool)
	require.NotNil(ap1)
	require.NoError(err)
	bs1, err := NewBlockSyncer(cfg, chain1, ap1, network.NewOverlay(&cfg.Network))
	require.Nil(err)
	chain2 := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	require.NoError(chain2.Start(ctx))
	require.NotNil(chain2)
	ap2, err := actpool.NewActPool(chain2, cfg.ActPool)
	require.NotNil(ap2)
	require.Nil(err)
	bs2, err := NewBlockSyncer(cfg, chain2, ap2, network.NewOverlay(&cfg.Network))
	require.Nil(err)

	defer func() {
		require.Nil(chain1.Stop(ctx))
		require.Nil(chain2.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	}()

	// commit top
	blk1, err := chain1.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk1)
	require.Nil(err)
	require.Nil(bs1.ProcessBlock(blk1))
	blk2, err := chain1.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk2)
	require.Nil(err)
	require.Nil(bs1.ProcessBlock(blk2))
	blk3, err := chain1.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk3)
	require.Nil(err)
	require.Nil(bs1.ProcessBlock(blk3))
	h1 := chain1.TipHeight()
	assert.Equal(t, uint64(3), h1)

	require.Nil(bs2.ProcessBlock(blk3))
	require.Nil(bs2.ProcessBlock(blk2))
	require.Nil(bs2.ProcessBlock(blk2))
	require.Nil(bs2.ProcessBlock(blk1))
	h2 := chain2.TipHeight()
	assert.Equal(t, h1, h2)
}

func TestBlockSyncerProcessBlockSync(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)

	chain1 := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	require.NoError(chain1.Start(ctx))
	require.NotNil(chain1)
	ap1, err := actpool.NewActPool(chain1, cfg.ActPool)
	require.NotNil(ap1)
	require.Nil(err)
	bs1, err := NewBlockSyncer(cfg, chain1, ap1, network.NewOverlay(&cfg.Network))
	require.Nil(err)
	chain2 := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	require.NoError(chain2.Start(ctx))
	require.NotNil(chain2)
	ap2, err := actpool.NewActPool(chain2, cfg.ActPool)
	require.NotNil(ap2)
	require.Nil(err)
	bs2, err := NewBlockSyncer(cfg, chain2, ap2, network.NewOverlay(&cfg.Network))
	require.Nil(err)

	defer func() {
		require.Nil(chain1.Stop(ctx))
		require.Nil(chain2.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	}()

	// commit top
	blk1, err := chain1.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk1)
	require.NoError(err)
	require.Nil(bs1.ProcessBlock(blk1))
	blk2, err := chain1.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk2)
	require.NoError(err)
	require.Nil(bs1.ProcessBlock(blk2))
	blk3, err := chain1.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk3)
	require.NoError(err)
	require.Nil(bs1.ProcessBlock(blk3))
	h1 := chain1.TipHeight()
	assert.Equal(t, uint64(3), h1)

	require.Nil(bs2.ProcessBlockSync(blk3))
	require.Nil(bs2.ProcessBlockSync(blk2))
	require.Nil(bs2.ProcessBlockSync(blk1))
	h2 := chain2.TipHeight()
	assert.Equal(t, h1, h2)
}

func TestBlockSyncerSync(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)

	chain := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	require.NoError(chain.Start(ctx))
	require.NotNil(chain)
	ap, err := actpool.NewActPool(chain, cfg.ActPool)
	require.NotNil(ap)
	require.NoError(err)

	bs, err := NewBlockSyncer(cfg, chain, ap, network.NewOverlay(&cfg.Network))
	require.NotNil(bs)
	require.NoError(err)
	require.Nil(bs.Start(ctx))
	time.Sleep(time.Millisecond << 7)

	defer func() {
		require.Nil(bs.Stop(ctx))
		require.Nil(chain.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	}()

	blk, err := chain.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk)
	require.NoError(err)
	require.Nil(bs.ProcessBlock(blk))

	blk, err = chain.MintNewBlock(nil, nil, nil, ta.Addrinfo["producer"], "")
	require.NotNil(blk)
	require.NoError(err)
	require.Nil(bs.ProcessBlock(blk))
	time.Sleep(time.Millisecond << 7)
}

func newTestConfig() (*config.Config, error) {
	cfg := config.Default
	cfg.Chain.TrieDBPath = "trie.test"
	cfg.Chain.ChainDBPath = "db.test"
	cfg.BlockSync.Interval = time.Millisecond << 4
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Host = "127.0.0.1"
	cfg.Network.Port = 10000
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:10000", "127.0.0.1:4689"}
	return &cfg, nil
}
