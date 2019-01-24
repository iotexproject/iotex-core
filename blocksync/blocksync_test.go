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

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/actpool"
	bc "github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_blocksync"
	"github.com/iotexproject/iotex-core/test/mock/mock_consensus"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

var opts = []Option{
	WithUnicastOutBound(func(_ context.Context, _ peerstore.PeerInfo, msg proto.Message) error {
		return nil
	}),
	WithNeighbors(func(_ context.Context) ([]peerstore.PeerInfo, error) { return nil, nil }),
}

func TestSyncTaskInterval(t *testing.T) {
	assert := assert.New(t)

	interval := time.Duration(0)

	cfgLightWeight := config.Config{
		NodeType: config.LightweightType,
	}
	lightWeight := syncTaskInterval(cfgLightWeight)
	assert.Equal(interval, lightWeight)

	cfgDelegate := config.Config{
		NodeType: config.DelegateType,
		BlockSync: config.BlockSync{
			Interval: interval,
		},
	}
	delegate := syncTaskInterval(cfgDelegate)
	assert.Equal(interval, delegate)

	cfgFullNode := config.Config{
		NodeType: config.FullNodeType,
		BlockSync: config.BlockSync{
			Interval: interval,
		},
	}
	interval <<= 2
	fullNode := syncTaskInterval(cfgFullNode)
	assert.Equal(interval, fullNode)
}

func TestNewBlockSyncer(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	// TipHeight return ERROR
	mBc.EXPECT().TipHeight().AnyTimes().Return(uint64(0))
	mBc.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
	blk := block.NewBlockDeprecated(
		uint32(123),
		uint64(0),
		hash.Hash32B{},
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	mBc.EXPECT().GetBlockByHeight(gomock.Any()).AnyTimes().Return(blk, nil)

	cfg, err := newTestConfig()
	require.Nil(err)
	ap, err := actpool.NewActPool(mBc, cfg.ActPool)
	assert.NoError(err)

	cs := mock_consensus.NewMockConsensus(ctrl)

	// Lightweight
	cfgLightWeight := config.Config{
		NodeType: config.LightweightType,
	}

	bsLightWeight, err := NewBlockSyncer(cfgLightWeight, mBc, ap, cs, opts...)
	assert.NoError(err)
	assert.NotNil(bsLightWeight)

	// Delegate
	cfgDelegate := config.Config{
		NodeType: config.DelegateType,
	}
	cfgDelegate.Network.BootstrapNodes = []string{"123"}

	_, err = NewBlockSyncer(cfgDelegate, mBc, ap, cs)
	assert.Nil(err)

	// FullNode
	cfgFullNode := config.Config{
		NodeType: config.FullNodeType,
	}
	cfgFullNode.Network.BootstrapNodes = []string{"123"}

	bs, err := NewBlockSyncer(cfgFullNode, mBc, ap, cs, opts...)
	assert.Nil(err)
	assert.NotNil(bs)
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
	mBc.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
	blk := block.NewBlockDeprecated(
		uint32(123),
		uint64(0),
		hash.Hash32B{},
		testutil.TimestampNow(),
		ta.Keyinfo["producer"].PubKey,
		nil,
	)
	mBc.EXPECT().GetBlockByHeight(gomock.Any()).AnyTimes().Return(blk, nil)
	mBc.EXPECT().TipHeight().AnyTimes().Return(uint64(0))
	cfg, err := newTestConfig()
	require.Nil(err)
	ap, err := actpool.NewActPool(mBc, cfg.ActPool)
	assert.NoError(err)
	cs := mock_consensus.NewMockConsensus(ctrl)

	cfgFullNode := config.Config{
		NodeType: config.FullNodeType,
	}
	cfgFullNode.Network.BootstrapNodes = []string{"123"}

	bs, err := NewBlockSyncer(cfgFullNode, mBc, ap, cs, opts...)
	assert.Nil(err)

	pbBs := &iproto.BlockSync{
		Start: 1,
		End:   1,
	}

	bs.(*blockSyncer).ackSyncReq = false
	assert.Nil(bs.ProcessSyncRequest(context.Background(), peerstore.PeerInfo{}, pbBs))
	bs.(*blockSyncer).ackSyncReq = true
	assert.Nil(bs.ProcessSyncRequest(context.Background(), peerstore.PeerInfo{}, pbBs))
}

func TestBlockSyncerProcessSyncRequestError(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

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
	cs := mock_consensus.NewMockConsensus(ctrl)

	bs, err := NewBlockSyncer(cfg, chain, ap, cs, opts...)
	require.Nil(err)

	defer func() {
		require.Nil(chain.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
		ctrl.Finish()
	}()

	pbBs := &iproto.BlockSync{
		Start: 1,
		End:   5,
	}

	bs.(*blockSyncer).ackSyncReq = true
	require.Error(bs.ProcessSyncRequest(context.Background(), peerstore.PeerInfo{}, pbBs))
}

func TestBlockSyncerProcessBlockTipHeight(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)

	chain := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain))
	chain.Validator().AddActionValidators(account.NewProtocol())
	require.NoError(chain.Start(ctx))
	require.NotNil(chain)
	ap, err := actpool.NewActPool(chain, cfg.ActPool)
	require.NotNil(ap)
	require.NoError(err)
	cs := mock_consensus.NewMockConsensus(ctrl)
	cs.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(1)

	bs, err := NewBlockSyncer(cfg, chain, ap, cs, opts...)
	require.Nil(err)

	defer func() {
		require.Nil(chain.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
		ctrl.Finish()
	}()

	h := chain.TipHeight()
	blk, err := chain.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk)
	require.NoError(err)
	bs.(*blockSyncer).ackBlockCommit = false
	require.Nil(bs.ProcessBlock(ctx, blk))
	h2 := chain.TipHeight()
	assert.Equal(t, h, h2)

	// commit top
	bs.(*blockSyncer).ackBlockCommit = true
	require.Nil(bs.ProcessBlock(ctx, blk))
	h3 := chain.TipHeight()
	assert.Equal(t, h+1, h3)

	// commit same block again
	require.Nil(bs.ProcessBlock(ctx, blk))
	h4 := chain.TipHeight()
	assert.Equal(t, h3, h4)
}

func TestBlockSyncerProcessBlockOutOfOrder(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)

	chain1 := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	chain1.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain1))
	chain1.Validator().AddActionValidators(account.NewProtocol())
	require.NoError(chain1.Start(ctx))
	require.NotNil(chain1)
	ap1, err := actpool.NewActPool(chain1, cfg.ActPool)
	require.NotNil(ap1)
	require.NoError(err)
	cs1 := mock_consensus.NewMockConsensus(ctrl)
	cs1.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)

	bs1, err := NewBlockSyncer(cfg, chain1, ap1, cs1, opts...)
	require.Nil(err)
	chain2 := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	chain2.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain2))
	chain2.Validator().AddActionValidators(account.NewProtocol())
	require.NoError(chain2.Start(ctx))
	require.NotNil(chain2)
	ap2, err := actpool.NewActPool(chain2, cfg.ActPool)
	require.NotNil(ap2)
	require.Nil(err)
	cs2 := mock_consensus.NewMockConsensus(ctrl)
	cs2.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)
	bs2, err := NewBlockSyncer(cfg, chain2, ap2, cs2, opts...)
	require.Nil(err)

	defer func() {
		require.Nil(chain1.Stop(ctx))
		require.Nil(chain2.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
		ctrl.Finish()
	}()

	// commit top
	blk1, err := chain1.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk1)
	require.Nil(err)
	require.Nil(bs1.ProcessBlock(ctx, blk1))
	blk2, err := chain1.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk2)
	require.Nil(err)
	require.Nil(bs1.ProcessBlock(ctx, blk2))
	blk3, err := chain1.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk3)
	require.Nil(err)
	require.Nil(bs1.ProcessBlock(ctx, blk3))
	h1 := chain1.TipHeight()
	assert.Equal(t, uint64(3), h1)

	require.Nil(bs2.ProcessBlock(ctx, blk3))
	require.Nil(bs2.ProcessBlock(ctx, blk2))
	require.Nil(bs2.ProcessBlock(ctx, blk2))
	require.Nil(bs2.ProcessBlock(ctx, blk1))
	h2 := chain2.TipHeight()
	assert.Equal(t, h1, h2)
}

func TestBlockSyncerProcessBlockSync(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.Nil(err)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)

	chain1 := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	chain1.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain1))
	chain1.Validator().AddActionValidators(account.NewProtocol())
	require.NoError(chain1.Start(ctx))
	require.NotNil(chain1)
	ap1, err := actpool.NewActPool(chain1, cfg.ActPool)
	require.NotNil(ap1)
	require.Nil(err)
	cs1 := mock_consensus.NewMockConsensus(ctrl)
	cs1.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)
	bs1, err := NewBlockSyncer(cfg, chain1, ap1, cs1, opts...)
	require.Nil(err)
	chain2 := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	chain2.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain2))
	chain2.Validator().AddActionValidators(account.NewProtocol())
	require.NoError(chain2.Start(ctx))
	require.NotNil(chain2)
	ap2, err := actpool.NewActPool(chain2, cfg.ActPool)
	require.NotNil(ap2)
	require.Nil(err)
	cs2 := mock_consensus.NewMockConsensus(ctrl)
	cs2.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)
	bs2, err := NewBlockSyncer(cfg, chain2, ap2, cs2, opts...)
	require.Nil(err)

	defer func() {
		require.Nil(chain1.Stop(ctx))
		require.Nil(chain2.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
		ctrl.Finish()
	}()

	// commit top
	blk1, err := chain1.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk1)
	require.NoError(err)
	require.Nil(bs1.ProcessBlock(ctx, blk1))
	blk2, err := chain1.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk2)
	require.NoError(err)
	require.Nil(bs1.ProcessBlock(ctx, blk2))
	blk3, err := chain1.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk3)
	require.NoError(err)
	require.Nil(bs1.ProcessBlock(ctx, blk3))
	h1 := chain1.TipHeight()
	assert.Equal(t, uint64(3), h1)

	require.Nil(bs2.ProcessBlockSync(ctx, blk3))
	require.Nil(bs2.ProcessBlockSync(ctx, blk2))
	require.Nil(bs2.ProcessBlockSync(ctx, blk1))
	h2 := chain2.TipHeight()
	assert.Equal(t, h1, h2)
}

func TestBlockSyncerSync(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

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
	cs := mock_consensus.NewMockConsensus(ctrl)
	cs.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(2)

	bs, err := NewBlockSyncer(cfg, chain, ap, cs, opts...)
	require.NotNil(bs)
	require.NoError(err)
	require.Nil(bs.Start(ctx))
	time.Sleep(time.Millisecond << 7)

	defer func() {
		require.Nil(bs.Stop(ctx))
		require.Nil(chain.Stop(ctx))
		testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
		testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
		ctrl.Finish()
	}()

	blk, err := chain.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk)
	require.NoError(err)
	require.Nil(bs.ProcessBlock(ctx, blk))

	blk, err = chain.MintNewBlock(nil, ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey, ta.Addrinfo["producer"].Bech32())
	require.NotNil(blk)
	require.NoError(err)
	require.Nil(bs.ProcessBlock(ctx, blk))
	time.Sleep(time.Millisecond << 7)
}

func newTestConfig() (config.Config, error) {
	cfg := config.Default
	cfg.Chain.TrieDBPath = "trie.test"
	cfg.Chain.ChainDBPath = "db.test"
	cfg.BlockSync.Interval = 100 * time.Millisecond
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Host = "127.0.0.1"
	cfg.Network.Port = 10000
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:10000", "127.0.0.1:4689"}
	return cfg, nil
}

func TestBlockSyncerChaser(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.NoError(err)

	chain := bc.NewBlockchain(cfg, bc.InMemStateFactoryOption(), bc.InMemDaoOption())
	require.NoError(chain.Start(ctx))
	ap, err := actpool.NewActPool(chain, cfg.ActPool)
	require.NoError(err)
	cs := mock_consensus.NewMockConsensus(ctrl)
	bs, err := NewBlockSyncer(cfg, chain, ap, cs, opts...)
	require.NoError(err)
	require.NoError(bs.Start(ctx))

	defer func() {
		require.NoError(chain.Stop(ctx))
		require.NoError(bs.Stop(ctx))
		ctrl.Finish()
	}()

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
		return bs.TargetHeight() == 1, nil
	}))
}
