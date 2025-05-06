// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	cp "github.com/iotexproject/iotex-core/v2/crypto"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/p2p/node"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_factory"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

type addrKeyPair struct {
	priKey      crypto.PrivateKey
	encodedAddr string
}

func TestNewRollDPoS(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	g := genesis.TestDefault()
	builderCfg := BuilderConfig{
		Chain:              blockchain.DefaultConfig,
		Consensus:          DefaultConfig,
		DardanellesUpgrade: consensusfsm.DefaultDardanellesUpgradeConfig,
		DB:                 db.DefaultConfig,
		Genesis:            g,
		SystemActive:       true,
	}
	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
	)
	delegatesByEpoch := func(uint64, []byte) ([]string, error) { return nil, nil }
	t.Run("normal", func(t *testing.T) {
		sk := identityset.PrivateKey(0)
		chain := mock_blockchain.NewMockBlockchain(ctrl)
		chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
		r, err := NewRollDPoSBuilder().
			SetConfig(builderCfg).
			SetPriKey(sk).
			SetChainManager(NewChainManager(chain, mock_factory.NewMockFactory(ctrl), &dummyBlockBuildFactory{})).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			SetDelegatesByEpochFunc(delegatesByEpoch).
			SetProposersByEpochFunc(delegatesByEpoch).
			RegisterProtocol(rp).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})
	t.Run("mock-clock", func(t *testing.T) {
		sk := identityset.PrivateKey(0)
		chain := mock_blockchain.NewMockBlockchain(ctrl)
		chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
		r, err := NewRollDPoSBuilder().
			SetConfig(builderCfg).
			SetPriKey(sk).
			SetChainManager(NewChainManager(chain, mock_factory.NewMockFactory(ctrl), &dummyBlockBuildFactory{})).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			SetClock(clock.NewMock()).
			SetDelegatesByEpochFunc(delegatesByEpoch).
			SetProposersByEpochFunc(delegatesByEpoch).
			RegisterProtocol(rp).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
		_, ok := r.ctx.Clock().(*clock.Mock)
		assert.True(t, ok)
	})

	t.Run("root chain API", func(t *testing.T) {
		sk := identityset.PrivateKey(0)
		chain := mock_blockchain.NewMockBlockchain(ctrl)
		chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
		r, err := NewRollDPoSBuilder().
			SetConfig(builderCfg).
			SetPriKey(sk).
			SetChainManager(NewChainManager(chain, mock_factory.NewMockFactory(ctrl), &dummyBlockBuildFactory{})).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			SetClock(clock.NewMock()).
			SetDelegatesByEpochFunc(delegatesByEpoch).
			SetProposersByEpochFunc(delegatesByEpoch).
			RegisterProtocol(rp).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})
	t.Run("missing-dep", func(t *testing.T) {
		sk := identityset.PrivateKey(0)
		r, err := NewRollDPoSBuilder().
			SetConfig(builderCfg).
			SetPriKey(sk).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			SetDelegatesByEpochFunc(delegatesByEpoch).
			SetProposersByEpochFunc(delegatesByEpoch).
			RegisterProtocol(rp).
			Build()
		assert.Error(t, err)
		assert.Nil(t, r)
	})
}

func makeBlock(t *testing.T, accountIndex, numOfEndosements int, makeInvalidEndorse bool, height int) *block.Block {
	unixTime := 1500000000
	blkTime := int64(-1)
	if height != 9 {
		height = 9
		blkTime = int64(-7723372030)
	}
	timeT := time.Unix(blkTime, 0)
	rap := block.RunnableActionsBuilder{}
	ra := rap.Build()
	blk, err := block.NewBuilder(ra).
		SetHeight(uint64(height)).
		SetTimestamp(timeT).
		SetVersion(1).
		SetReceiptRoot(hash.Hash256b([]byte("hello, world!"))).
		SetDeltaStateDigest(hash.Hash256b([]byte("world, hello!"))).
		SetPrevBlockHash(hash.Hash256b([]byte("hello, block!"))).
		SignAndBuild(identityset.PrivateKey(accountIndex))
	require.NoError(t, err)
	footerForBlk := &block.Footer{}
	typesFooter := iotextypes.BlockFooter{}

	for i := 0; i < numOfEndosements; i++ {
		timeTime := time.Unix(int64(unixTime), 0)
		hs := blk.HashBlock()
		var consensusVote *ConsensusVote
		if makeInvalidEndorse {
			consensusVote = NewConsensusVote(hs[:], LOCK)
		} else {
			consensusVote = NewConsensusVote(hs[:], COMMIT)
		}
		en, err := endorsement.Endorse(consensusVote, timeTime, identityset.PrivateKey(i))
		require.NoError(t, err)
		require.Equal(t, 1, len(en))
		typesFooter.Endorsements = append(typesFooter.Endorsements, en[0].Proto())
	}
	ts := timestamppb.New(time.Unix(int64(unixTime), 0))
	typesFooter.Timestamp = ts
	require.NotNil(t, typesFooter.Timestamp)
	err = footerForBlk.ConvertFromBlockFooterPb(&typesFooter)
	require.NoError(t, err)
	blk.Footer = *footerForBlk
	return &blk
}

func TestValidateBlockFooter(t *testing.T) {
	ctrl := gomock.NewController(t)
	candidates := make([]string, 5)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = identityset.Address(i).String()
	}
	clock := clock.NewMock()
	blockHeight := uint64(8)
	footer := &block.Footer{}
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().BlockFooterByHeight(blockHeight).Return(footer, nil).AnyTimes()
	bc.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
	bc.EXPECT().TipHeight().Return(blockHeight).AnyTimes()
	bc.EXPECT().BlockHeaderByHeight(blockHeight).Return(&block.Header{}, nil).AnyTimes()
	bc.EXPECT().TipHash().Return(hash.ZeroHash256).AnyTimes()
	sk1 := identityset.PrivateKey(1)
	g := genesis.TestDefault()
	g.NumDelegates = 4
	g.NumSubEpochs = 1
	g.BlockInterval = 10 * time.Second
	g.Timestamp = int64(1500000000)
	builderCfg := BuilderConfig{
		Chain:              blockchain.DefaultConfig,
		Consensus:          DefaultConfig,
		DardanellesUpgrade: consensusfsm.DefaultDardanellesUpgradeConfig,
		DB:                 db.DefaultConfig,
		Genesis:            g,
		SystemActive:       true,
		WakeUpgrade:        consensusfsm.DefaultWakeUpgradeConfig,
	}
	builderCfg.Consensus.ConsensusDBPath = ""
	bc.EXPECT().Genesis().Return(g).AnyTimes()
	sf := mock_factory.NewMockFactory(ctrl)
	sf.EXPECT().StateReaderAt(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
	)
	delegatesByEpoch := func(uint64, []byte) ([]string, error) {
		return []string{
			candidates[0],
			candidates[1],
			candidates[2],
			candidates[3],
		}, nil
	}
	r, err := NewRollDPoSBuilder().
		SetConfig(builderCfg).
		SetPriKey(sk1).
		SetChainManager(NewChainManager(bc, sf, &dummyBlockBuildFactory{})).
		SetBroadcast(func(_ proto.Message) error {
			return nil
		}).
		SetDelegatesByEpochFunc(delegatesByEpoch).
		SetProposersByEpochFunc(delegatesByEpoch).
		SetClock(clock).
		RegisterProtocol(rp).
		Build()
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(context.Background()))

	// all right
	blk := makeBlock(t, 1, 4, false, 9)
	err = r.ValidateBlockFooter(blk)
	require.NoError(t, err)

	// Proposer is wrong
	blk = makeBlock(t, 4, 4, false, 9)
	err = r.ValidateBlockFooter(blk)
	require.Error(t, err)

	// Not enough endorsements
	blk = makeBlock(t, 1, 2, false, 9)
	err = r.ValidateBlockFooter(blk)
	require.Error(t, err)

	// round information is wrong
	blk = makeBlock(t, 1, 4, false, 0)
	err = r.ValidateBlockFooter(blk)
	require.Error(t, err)

	// Some endorsement is invalid
	blk = makeBlock(t, 1, 4, true, 9)
	err = r.ValidateBlockFooter(blk)
	require.Error(t, err)
}

func TestRollDPoS_Metrics(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	candidates := make([]string, 5)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = identityset.Address(i).String()
		t.Logf("candidate %d: %s", i, candidates[i])
	}

	clock := clock.NewMock()
	blockHeight := uint64(8)
	footer := &block.Footer{}
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().TipHeight().Return(blockHeight).AnyTimes()
	bc.EXPECT().TipHash().Return(hash.ZeroHash256).AnyTimes()
	bc.EXPECT().BlockFooterByHeight(blockHeight).Return(footer, nil).AnyTimes()
	bc.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
	bc.EXPECT().BlockHeaderByHeight(blockHeight).Return(&block.Header{}, nil).AnyTimes()

	sk1 := identityset.PrivateKey(1)
	cfg := DefaultConfig
	cfg.ConsensusDBPath = "consensus.db"
	g := genesis.TestDefault()
	g.NumDelegates = 4
	g.NumSubEpochs = 1
	g.BlockInterval = 10 * time.Second
	g.Timestamp = int64(1500000000)
	builderCfg := BuilderConfig{
		Chain:              blockchain.DefaultConfig,
		Consensus:          cfg,
		DardanellesUpgrade: consensusfsm.DefaultDardanellesUpgradeConfig,
		DB:                 db.DefaultConfig,
		Genesis:            g,
		SystemActive:       true,
	}
	bc.EXPECT().Genesis().Return(g).Times(2)
	sf := mock_factory.NewMockFactory(ctrl)
	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
	)
	delegatesByEpoch := func(uint64, []byte) ([]string, error) {
		return []string{
			candidates[0],
			candidates[1],
			candidates[2],
			candidates[3],
		}, nil
	}
	r, err := NewRollDPoSBuilder().
		SetConfig(builderCfg).
		SetPriKey(sk1).
		SetChainManager(NewChainManager(bc, sf, &dummyBlockBuildFactory{})).
		SetBroadcast(func(_ proto.Message) error {
			return nil
		}).
		SetClock(clock).
		SetDelegatesByEpochFunc(delegatesByEpoch).
		SetProposersByEpochFunc(delegatesByEpoch).
		RegisterProtocol(rp).
		Build()
	require.NoError(t, err)
	require.NotNil(t, r)
	ctx := r.ctx.(*rollDPoSCtx)
	clock.Add(ctx.BlockInterval(blockHeight))
	require.NoError(t, ctx.Start(context.Background()))
	ctx.round, err = ctx.roundCalc.UpdateRound(ctx.round, blockHeight+1, ctx.BlockInterval(blockHeight+1), clock.Now(), 2*time.Second)
	require.NoError(t, err)

	m, err := r.Metrics()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), m.LatestEpoch)
	assert.Equal(t, candidates[:4], m.LatestDelegates)
	assert.Equal(t, candidates[1], m.LatestBlockProducer)
}

// E2E RollDPoS tests bellow

type directOverlay struct {
	addr  net.Addr
	peers map[net.Addr]*RollDPoS
}

func (o *directOverlay) Start(_ context.Context) error { return nil }

func (o *directOverlay) Stop(_ context.Context) error { return nil }

func (o *directOverlay) Broadcast(msg proto.Message) error {
	// Only broadcast consensus message
	if cMsg, ok := msg.(*iotextypes.ConsensusMessage); ok {
		for _, r := range o.peers {
			if err := r.HandleConsensusMsg(cMsg); err != nil {
				return errors.Wrap(err, "error when handling consensus message directly")
			}
		}
	}
	return nil
}

func (o *directOverlay) Tell(uint32, net.Addr, proto.Message) error { return nil }

func (o *directOverlay) Self() net.Addr { return o.addr }

func (o *directOverlay) GetPeers() []net.Addr {
	addrs := make([]net.Addr, 0, len(o.peers))
	for addr := range o.peers {
		addrs = append(addrs, addr)
	}
	return addrs
}

func TestRollDPoSConsensus(t *testing.T) {
	newConsensusComponents := func(numNodes int) ([]*RollDPoS, []*directOverlay, []blockchain.Blockchain) {
		cfg := DefaultConfig
		cfg.ConsensusDBPath = ""
		cfg.Delay = 300 * time.Millisecond
		cfg.FSM.AcceptBlockTTL = 800 * time.Millisecond
		cfg.FSM.AcceptProposalEndorsementTTL = 400 * time.Millisecond
		cfg.FSM.AcceptLockEndorsementTTL = 400 * time.Millisecond
		cfg.FSM.CommitTTL = 400 * time.Millisecond
		cfg.FSM.UnmatchedEventTTL = time.Second
		cfg.FSM.UnmatchedEventInterval = 10 * time.Millisecond
		cfg.ToleratedOvertime = 200 * time.Millisecond
		g := genesis.TestDefault()
		g.BlockInterval = 2 * time.Second
		g.Blockchain.NumDelegates = uint64(numNodes)
		g.Blockchain.NumSubEpochs = 1
		g.EnableGravityChainVoting = false
		builderCfg := BuilderConfig{
			Chain:              blockchain.DefaultConfig,
			Consensus:          cfg,
			DardanellesUpgrade: consensusfsm.DefaultDardanellesUpgradeConfig,
			DB:                 db.DefaultConfig,
			Genesis:            g,
			SystemActive:       true,
		}
		chainAddrs := make([]*addrKeyPair, 0, numNodes)
		networkAddrs := make([]net.Addr, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			sk := identityset.PrivateKey(i)
			addr := addrKeyPair{
				encodedAddr: identityset.Address(i).String(),
				priKey:      sk,
			}
			chainAddrs = append(chainAddrs, &addr)
			networkAddrs = append(networkAddrs, node.NewTCPNode(fmt.Sprintf("127.0.0.%d:4689", i+1)))
		}

		chainRawAddrs := make([]string, 0, numNodes)
		addressMap := make(map[string]*addrKeyPair)
		for _, addr := range chainAddrs {
			chainRawAddrs = append(chainRawAddrs, addr.encodedAddr)
			addressMap[addr.encodedAddr] = addr
		}
		cp.SortCandidates(chainRawAddrs, 1, cp.CryptoSeed)
		for i, rawAddress := range chainRawAddrs {
			chainAddrs[i] = addressMap[rawAddress]
		}

		delegatesByEpochFunc := func(_ uint64, _ []byte) ([]string, error) {
			candidates := make([]string, 0, numNodes)
			for _, addr := range chainAddrs {
				candidates = append(candidates, addr.encodedAddr)
			}
			return candidates, nil
		}

		chains := make([]blockchain.Blockchain, 0, numNodes)
		p2ps := make([]*directOverlay, 0, numNodes)
		cs := make([]*RollDPoS, 0, numNodes)
		bc := blockchain.DefaultConfig
		factoryCfg := factory.GenerateConfig(bc, g)
		for i := 0; i < numNodes; i++ {
			ctx := context.Background()
			bc.ProducerPrivKey = hex.EncodeToString(chainAddrs[i].priKey.Bytes())
			registry := protocol.NewRegistry()
			sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
			require.NoError(t, err)
			require.NoError(t, sf.Start(genesis.WithGenesisContext(
				protocol.WithRegistry(ctx, registry),
				g,
			)))
			actPool, err := actpool.NewActPool(g, sf, actpool.DefaultConfig)
			require.NoError(t, err)
			require.NoError(t, err)
			acc := account.NewProtocol(rewarding.DepositGas)
			require.NoError(t, acc.Register(registry))
			rp := rolldpos.NewProtocol(g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs)
			require.NoError(t, rp.Register(registry))
			store, err := filedao.NewFileDAOInMemForTest()
			require.NoError(t, err)
			dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, db.DefaultConfig.MaxCacheSize)
			chain := blockchain.NewBlockchain(
				bc,
				g,
				dao,
				factory.NewMinter(sf, actPool),
				blockchain.BlockValidatorOption(block.NewValidator(
					sf,
					protocol.NewGenericValidator(sf, accountutil.AccountState),
				)),
			)
			chains = append(chains, chain)

			p2p := &directOverlay{
				addr:  networkAddrs[i],
				peers: make(map[net.Addr]*RollDPoS),
			}
			p2ps = append(p2ps, p2p)
			minter := factory.NewMinter(sf, actPool)
			consensus, err := NewRollDPoSBuilder().
				SetPriKey(chainAddrs[i].priKey).
				SetConfig(builderCfg).
				SetChainManager(NewChainManager(chain, sf, minter)).
				SetBroadcast(p2p.Broadcast).
				SetDelegatesByEpochFunc(delegatesByEpochFunc).
				SetProposersByEpochFunc(delegatesByEpochFunc).
				RegisterProtocol(rp).
				Build()
			require.NoError(t, err)

			cs = append(cs, consensus)
		}
		for i := 0; i < numNodes; i++ {
			for j := 0; j < numNodes; j++ {
				if i != j {
					p2ps[i].peers[p2ps[j].addr] = cs[j]
				}
			}
		}
		return cs, p2ps, chains
	}

	t.Run("1-block", func(t *testing.T) {
		// TODO: fix and enable the test
		t.Skip()

		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(200*time.Millisecond, 10*time.Second, func() (bool, error) {
			for _, chain := range chains {
				if chain.TipHeight() < 1 {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("1-epoch", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skip the 1-epoch test in short mode.")
		}
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(200*time.Millisecond, 100*time.Second, func() (bool, error) {
			for _, chain := range chains {
				if chain.TipHeight() < 48 {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("network-partition-time-rotation", func(t *testing.T) {
		// TODO: fix and enable the test
		t.Skip()

		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				cs[idx].ctx.(*rollDPoSCtx).roundCalc.timeBasedRotation = true
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		assert.NoError(t, testutil.WaitUntil(200*time.Millisecond, 60*time.Second, func() (bool, error) {
			for i, chain := range chains {
				if i == 1 {
					continue
				}
				if chain.TipHeight() < 4 {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("proposer-network-partition-blocking", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		time.Sleep(5 * time.Second)
		for _, chain := range chains {
			header, err := chain.BlockHeaderByHeight(1)
			assert.Nil(t, header)
			assert.Error(t, err)
		}
	})

	t.Run("non-proposer-network-partition-blocking", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 0 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[0].addr)
			}
		}

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(200*time.Millisecond, 60*time.Second, func() (bool, error) {
			for i, chain := range chains {
				if i == 0 {
					continue
				}
				if chain.TipHeight() < 2 {
					return false, nil
				}
			}
			return true, nil
		}))
		for i, chain := range chains {
			header, err := chain.BlockHeaderByHeight(1)
			if i == 0 {
				assert.Nil(t, header)
				assert.Error(t, err)
			} else {
				assert.NotNil(t, header)
				assert.NoError(t, err)
			}
		}
	})
}

type dummyBlockBuildFactory struct{}

func (d *dummyBlockBuildFactory) Mint(ctx context.Context, pk crypto.PrivateKey) (*block.Block, error) {
	return &block.Block{}, nil
}

func (d *dummyBlockBuildFactory) ReceiveBlock(*block.Block) error {
	return nil
}
func (d *dummyBlockBuildFactory) Init(hash.Hash256) {}
func (d *dummyBlockBuildFactory) AddProposal(*block.Block) error {
	return nil
}
func (d *dummyBlockBuildFactory) Block(hash.Hash256) *block.Block {
	return &block.Block{}
}
