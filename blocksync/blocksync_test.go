// Copyright (c) 2019 IoTeX Foundation
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
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockdao"
	"github.com/iotexproject/iotex-core/test/mock/mock_blocksync"
	"github.com/iotexproject/iotex-core/test/mock/mock_consensus"
	"github.com/iotexproject/iotex-core/testutil"
)

func newBlockSyncer(cfg config.BlockSync, chain blockchain.Blockchain, dao blockdao.BlockDAO, cs consensus.Consensus) (*blockSyncer, error) {
	bs, err := NewBlockSyncer(cfg, chain.TipHeight, func(h uint64) (*block.Block, error) {
		return dao.GetBlockByHeight(h)
	}, func(blk *block.Block) error {
		if err := cs.ValidateBlockFooter(blk); err != nil {
			return err
		}
		if err := chain.ValidateBlock(blk); err != nil {
			return err
		}
		if err := chain.CommitBlock(blk); err != nil {
			return err
		}
		cs.Calibrate(blk.Height())
		return nil
	}, func(context.Context, uint64, uint64, int) {
	})
	if err != nil {
		return nil, err
	}
	return bs.(*blockSyncer), nil
}

func TestNewBlockSyncer(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctrl := gomock.NewController(t)

	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	// TipHeight return ERROR
	mBc.EXPECT().TipHeight().AnyTimes().Return(uint64(0))
	mBc.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
	blk := block.NewBlockDeprecated(
		uint32(123),
		uint64(0),
		hash.Hash256{},
		testutil.TimestampNow(),
		identityset.PrivateKey(27).PublicKey(),
		nil,
	)
	dao := mock_blockdao.NewMockBlockDAO(ctrl)
	dao.EXPECT().GetBlockByHeight(gomock.Any()).AnyTimes().Return(blk, nil)

	cfg, err := newTestConfig()
	require.NoError(err)

	cs := mock_consensus.NewMockConsensus(ctrl)

	bs, err := newBlockSyncer(cfg.BlockSync, mBc, dao, cs)
	assert.Nil(err)
	assert.NotNil(bs)
}

func TestBlockSyncerStart(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)

	ctx := context.Background()
	mBs := mock_blocksync.NewMockBlockSync(ctrl)
	mBs.EXPECT().Start(gomock.Any()).Times(1)
	assert.Nil(mBs.Start(ctx))
}

func TestBlockSyncerStop(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)

	ctx := context.Background()
	mBs := mock_blocksync.NewMockBlockSync(ctrl)
	mBs.EXPECT().Stop(gomock.Any()).Times(1)
	assert.Nil(mBs.Stop(ctx))
}

func TestBlockSyncerProcessSyncRequest(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctrl := gomock.NewController(t)

	mBc := mock_blockchain.NewMockBlockchain(ctrl)
	mBc.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
	blk := block.NewBlockDeprecated(
		uint32(123),
		uint64(0),
		hash.Hash256{},
		testutil.TimestampNow(),
		identityset.PrivateKey(27).PublicKey(),
		nil,
	)
	dao := mock_blockdao.NewMockBlockDAO(ctrl)
	dao.EXPECT().GetBlockByHeight(gomock.Any()).AnyTimes().Return(blk, nil)
	mBc.EXPECT().TipHeight().AnyTimes().Return(uint64(0))
	cfg, err := newTestConfig()
	require.NoError(err)
	cs := mock_consensus.NewMockConsensus(ctrl)

	bs, err := newBlockSyncer(cfg.BlockSync, mBc, dao, cs)
	assert.NoError(err)
	assert.NoError(bs.ProcessSyncRequest(context.Background(), 1, 1, func(context.Context, *block.Block) error {
		return nil
	}))
}

func TestBlockSyncerProcessSyncRequestError(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	cfg, err := newTestConfig()
	require.NoError(err)

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	dao := mock_blockdao.NewMockBlockDAO(ctrl)
	dao.EXPECT().GetBlockByHeight(uint64(1)).Return(nil, errors.New("some error")).Times(1)
	chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
	chain.EXPECT().TipHeight().Return(uint64(10)).Times(1)
	cs := mock_consensus.NewMockConsensus(ctrl)

	bs, err := newBlockSyncer(cfg.BlockSync, chain, dao, cs)
	require.NoError(err)

	require.Error(bs.ProcessSyncRequest(context.Background(), 1, 5, func(context.Context, *block.Block) error {
		return nil
	}))
}

func TestBlockSyncerProcessBlockTipHeight(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.NoError(err)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool, actpool.EnableExperimentalActions())
	require.NotNil(ap)
	require.NoError(err)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	chain := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(sf, ap)),
	)
	require.NoError(chain.Start(ctx))
	require.NotNil(chain)
	cs := mock_consensus.NewMockConsensus(ctrl)
	cs.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(1)
	cs.EXPECT().Calibrate(uint64(1)).Times(1)

	bs, err := newBlockSyncer(cfg.BlockSync, chain, dao, cs)
	require.NoError(err)

	defer func() {
		require.NoError(chain.Stop(ctx))
	}()

	h := chain.TipHeight()
	blk, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk)
	require.NoError(err)
	ctx, err = chain.Context(ctx)
	require.NoError(err)

	peer := "peer1"

	require.NoError(bs.ProcessBlock(ctx, peer, blk))
	h2 := chain.TipHeight()
	assert.Equal(t, h+1, h2)

	// commit top
	require.NoError(bs.ProcessBlock(ctx, peer, blk))
	h3 := chain.TipHeight()
	assert.Equal(t, h+1, h3)

	// commit same block again
	require.NoError(bs.ProcessBlock(ctx, peer, blk))
	h4 := chain.TipHeight()
	assert.Equal(t, h3, h4)
}

func TestBlockSyncerProcessBlockOutOfOrder(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.NoError(err)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap1, err := actpool.NewActPool(sf, cfg.ActPool, actpool.EnableExperimentalActions())
	require.NotNil(ap1)
	require.NoError(err)
	ap1.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	chain1 := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap1),
		blockchain.BlockValidatorOption(block.NewValidator(sf, ap1)),
	)
	require.NotNil(chain1)
	require.NoError(chain1.Start(ctx))
	cs1 := mock_consensus.NewMockConsensus(ctrl)
	cs1.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)
	cs1.EXPECT().Calibrate(gomock.Any()).Times(3)

	bs1, err := newBlockSyncer(cfg.BlockSync, chain1, dao, cs1)
	require.NoError(err)
	registry2 := protocol.NewRegistry()
	require.NoError(acc.Register(registry2))
	require.NoError(rp.Register(registry2))
	sf2, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry2))
	require.NoError(err)
	ap2, err := actpool.NewActPool(sf2, cfg.ActPool, actpool.EnableExperimentalActions())
	require.NotNil(ap2)
	require.NoError(err)
	ap2.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf2, accountutil.AccountState))
	dao2 := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf2})
	chain2 := blockchain.NewBlockchain(
		cfg,
		dao2,
		factory.NewMinter(sf2, ap2),
		blockchain.BlockValidatorOption(block.NewValidator(sf2, ap2)),
	)
	require.NotNil(chain2)
	require.NoError(chain2.Start(ctx))
	cs2 := mock_consensus.NewMockConsensus(ctrl)
	cs2.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)
	cs2.EXPECT().Calibrate(gomock.Any()).Times(3)
	bs2, err := newBlockSyncer(cfg.BlockSync, chain2, dao2, cs2)
	require.NoError(err)

	defer func() {
		require.NoError(chain1.Stop(ctx))
		require.NoError(chain2.Stop(ctx))
	}()

	// commit top
	ctx, err = chain1.Context(ctx)
	require.NoError(err)

	peer := "peer1"

	blk1, err := chain1.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk1)
	require.NoError(err)
	require.NoError(bs1.ProcessBlock(ctx, peer, blk1))
	blk2, err := chain1.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk2)
	require.NoError(err)
	require.NoError(bs1.ProcessBlock(ctx, peer, blk2))
	blk3, err := chain1.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk3)
	require.NoError(err)
	require.NoError(bs1.ProcessBlock(ctx, peer, blk3))
	h1 := chain1.TipHeight()
	assert.Equal(t, uint64(3), h1)

	require.NoError(bs2.ProcessBlock(ctx, peer, blk3))
	require.NoError(bs2.ProcessBlock(ctx, peer, blk2))
	require.NoError(bs2.ProcessBlock(ctx, peer, blk2))
	require.NoError(bs2.ProcessBlock(ctx, peer, blk1))
	h2 := chain2.TipHeight()
	assert.Equal(t, h1, h2)
}

func TestBlockSyncerProcessBlock(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.NoError(err)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rolldposProtocol := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	require.NoError(rolldposProtocol.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap1, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NotNil(ap1)
	require.NoError(err)
	ap1.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	chain1 := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap1),
		blockchain.BlockValidatorOption(block.NewValidator(sf, ap1)),
	)
	require.NoError(chain1.Start(ctx))
	require.NotNil(chain1)
	cs1 := mock_consensus.NewMockConsensus(ctrl)
	cs1.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)
	cs1.EXPECT().Calibrate(gomock.Any()).Times(3)
	bs1, err := newBlockSyncer(cfg.BlockSync, chain1, dao, cs1)
	require.NoError(err)
	registry2 := protocol.NewRegistry()
	require.NoError(acc.Register(registry2))
	require.NoError(rolldposProtocol.Register(registry2))
	sf2, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry2))
	require.NoError(err)
	ap2, err := actpool.NewActPool(sf2, cfg.ActPool, actpool.EnableExperimentalActions())
	require.NotNil(ap2)
	require.NoError(err)
	ap2.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf2, accountutil.AccountState))
	dao2 := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf2})
	chain2 := blockchain.NewBlockchain(
		cfg,
		dao2,
		factory.NewMinter(sf2, ap2),
		blockchain.BlockValidatorOption(block.NewValidator(sf2, ap2)),
	)
	require.NoError(chain2.Start(ctx))
	require.NotNil(chain2)
	cs2 := mock_consensus.NewMockConsensus(ctrl)
	cs2.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)
	cs2.EXPECT().Calibrate(gomock.Any()).Times(3)
	bs2, err := newBlockSyncer(cfg.BlockSync, chain2, dao2, cs2)
	require.NoError(err)

	defer func() {
		require.NoError(chain1.Stop(ctx))
		require.NoError(chain2.Stop(ctx))
	}()

	ctx, err = chain1.Context(ctx)
	require.NoError(err)

	peer := "peer1"

	// commit top
	blk1, err := chain1.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk1)
	require.NoError(err)
	require.NoError(bs1.ProcessBlock(ctx, peer, blk1))
	blk2, err := chain1.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk2)
	require.NoError(err)
	require.NoError(bs1.ProcessBlock(ctx, peer, blk2))
	blk3, err := chain1.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk3)
	require.NoError(err)
	require.NoError(bs1.ProcessBlock(ctx, peer, blk3))
	h1 := chain1.TipHeight()
	assert.Equal(t, uint64(3), h1)

	require.NoError(bs2.ProcessBlock(ctx, peer, blk2))
	require.NoError(bs2.ProcessBlock(ctx, peer, blk3))
	require.NoError(bs2.ProcessBlock(ctx, peer, blk1))
	h2 := chain2.TipHeight()
	assert.Equal(t, h1, h2)
}

func TestBlockSyncerSync(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.NoError(err)
	registry := protocol.NewRegistry()
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool, actpool.EnableExperimentalActions())
	require.NotNil(ap)
	require.NoError(err)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	chain := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(sf, ap)),
	)
	require.NoError(chain.Start(ctx))
	require.NotNil(chain)
	cs := mock_consensus.NewMockConsensus(ctrl)
	cs.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(2)
	cs.EXPECT().Calibrate(gomock.Any()).Times(2)

	bs, err := newBlockSyncer(cfg.BlockSync, chain, dao, cs)
	require.NoError(err)
	require.NotNil(bs)
	require.NoError(bs.Start(ctx))
	time.Sleep(time.Millisecond << 7)

	defer func() {
		require.NoError(bs.Stop(ctx))
		require.NoError(chain.Stop(ctx))
	}()

	ctx, err = chain.Context(ctx)
	require.NoError(err)

	peer := "peer1"

	blk, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk)
	require.NoError(err)
	require.NoError(bs.ProcessBlock(ctx, peer, blk))

	blk, err = chain.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk)
	require.NoError(err)
	require.NoError(bs.ProcessBlock(ctx, peer, blk))
	time.Sleep(time.Millisecond << 7)
}

func newTestConfig() (config.Config, error) {
	testTriePath, err := testutil.PathOfTempFile("trie")
	if err != nil {
		return config.Config{}, err
	}
	testDBPath, err := testutil.PathOfTempFile("db")
	if err != nil {
		return config.Config{}, err
	}
	defer func() {
		testutil.CleanupPathV2(testTriePath)
		testutil.CleanupPathV2(testDBPath)
	}()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.BlockSync.Interval = 100 * time.Millisecond
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Host = "127.0.0.1"
	cfg.Network.Port = 10000
	cfg.Network.BootstrapNodes = []string{"127.0.0.1:10000", "127.0.0.1:4689"}
	cfg.Genesis.EnableGravityChainVoting = false
	return cfg, nil
}

func TestBlockSyncerPeerBlockList(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	cfg, err := newTestConfig()
	require.NoError(err)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool, actpool.EnableExperimentalActions())
	require.NotNil(ap)
	require.NoError(err)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	chain := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(sf, ap)),
	)
	require.NotNil(chain)
	require.NoError(chain.Start(ctx))
	cs := mock_consensus.NewMockConsensus(ctrl)
	cs.EXPECT().ValidateBlockFooter(gomock.Any()).Return(nil).Times(3)
	cs.EXPECT().Calibrate(gomock.Any()).Times(2)

	bs, err := newBlockSyncer(cfg.BlockSync, chain, dao, cs)
	require.NoError(err)

	defer func() {
		require.NoError(chain.Stop(ctx))
	}()

	ctx, err = chain.Context(ctx)
	require.NoError(err)

	h := chain.TipHeight()
	blk1, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk1)
	require.NoError(err)

	blk2 := block.NewBlockDeprecated(
		uint32(123),
		blk1.Height(),
		hash.Hash256{},
		testutil.TimestampNow(),
		identityset.PrivateKey(27).PublicKey(),
		nil,
	)

	require.NoError(bs.ProcessBlock(ctx, "peer1", blk2))
	require.NoError(bs.ProcessBlock(ctx, "peer2", blk1))

	h2 := chain.TipHeight()
	assert.Equal(t, h+1, h2)

	blk3, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk3)
	require.NoError(err)

	require.NoError(bs.ProcessBlock(ctx, "peer1", blk3))

	h3 := chain.TipHeight()
	assert.Equal(t, h2, h3)

	blk4, err := chain.MintNewBlock(testutil.TimestampNow())
	require.NotNil(blk4)
	require.NoError(err)

	require.NoError(bs.ProcessBlock(ctx, "peer2", blk4))

	h4 := chain.TipHeight()
	assert.Equal(t, h2+1, h4)
}

func TestDummyBlockSync(t *testing.T) {
	require := require.New(t)
	bs := NewDummyBlockSyncer()
	require.NoError(bs.Start(nil))
	require.NoError(bs.Stop(nil))
	require.NoError(bs.ProcessBlock(nil, "", nil))
	require.NoError(bs.ProcessSyncRequest(nil, 0, 0, nil))
	require.Equal(bs.TargetHeight(), uint64(0))
	require.Equal(bs.SyncStatus(), "")
}
