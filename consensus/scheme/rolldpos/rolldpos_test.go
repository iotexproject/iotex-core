// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_network"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestRollDPoSCtx(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 4)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	clock := clock.NewMock()
	var prevHash hash.Hash32B
	blk := blockchain.NewBlock(
		1,
		8,
		prevHash,
		clock,
		make([]*action.Transfer, 0),
		make([]*action.Vote, 0),
		make([]*action.Execution, 0),
	)
	ctx := makeTestRollDPoSCtx(
		testAddrs[0],
		ctrl,
		config.RollDPoS{
			NumSubEpochs: 2,
			NumDelegates: 4,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().TipHeight().Return(uint64(8), nil).Times(3)
			blockchain.EXPECT().GetBlockByHeight(uint64(8)).Return(blk, nil).Times(1)
			blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
				{Address: candidates[0]},
				{Address: candidates[1]},
				{Address: candidates[2]},
				{Address: candidates[3]},
			}, nil).Times(1)
		},
		func(_ *mock_actpool.MockActPool) {},
		func(_ *mock_network.MockOverlay) {},
		clock,
	)

	epoch, height, err := ctx.calcEpochNumAndHeight()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), epoch)
	assert.Equal(t, uint64(9), height)

	delegates, err := ctx.rollingDelegates(epoch)
	require.NoError(t, err)
	crypto.SortCandidates(candidates, epoch)
	assert.Equal(t, candidates, delegates)

	ctx.epoch = epochCtx{
		num:          epoch,
		height:       height,
		numSubEpochs: 1,
		delegates:    delegates,
	}
	proposer, height, err := ctx.rotatedProposer()
	require.NoError(t, err)
	assert.Equal(t, candidates[1], proposer)
	assert.Equal(t, uint64(9), height)

	clock.Add(time.Second)
	duration, err := ctx.calcDurationSinceLastBlock()
	require.NoError(t, err)
	assert.Equal(t, time.Second, duration)

	yes, no := ctx.calcQuorum(map[string]bool{
		candidates[0]: true,
		candidates[1]: true,
		candidates[2]: true,
	})
	assert.True(t, yes)
	assert.False(t, no)

	yes, no = ctx.calcQuorum(map[string]bool{
		candidates[0]: false,
		candidates[1]: false,
		candidates[2]: false,
	})
	assert.False(t, yes)
	assert.True(t, no)

	yes, no = ctx.calcQuorum(map[string]bool{
		candidates[0]: true,
		candidates[1]: true,
		candidates[2]: false,
		candidates[3]: false,
	})
	assert.False(t, yes)
	assert.False(t, no)
}

func TestIsEpochFinished(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 4)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	t.Run("not-finished", func(t *testing.T) {
		ctx := makeTestRollDPoSCtx(
			testAddrs[0],
			ctrl,
			config.RollDPoS{
				NumSubEpochs: 2,
			},
			func(blockchain *mock_blockchain.MockBlockchain) {
				blockchain.EXPECT().TipHeight().Return(uint64(8), nil).Times(1)
			},
			func(_ *mock_actpool.MockActPool) {},
			func(_ *mock_network.MockOverlay) {},
			clock.NewMock(),
		)
		finished, err := ctx.isEpochFinished()
		require.NoError(t, err)
		assert.False(t, finished)
	})
	t.Run("finished", func(t *testing.T) {
		ctx := makeTestRollDPoSCtx(
			testAddrs[0],
			ctrl,
			config.RollDPoS{
				NumSubEpochs: 2,
			},
			func(blockchain *mock_blockchain.MockBlockchain) {
				blockchain.EXPECT().TipHeight().Return(uint64(12), nil).Times(1)
			},
			func(_ *mock_actpool.MockActPool) {},
			func(_ *mock_network.MockOverlay) {},
			clock.NewMock(),
		)
		finished, err := ctx.isEpochFinished()
		require.NoError(t, err)
		assert.False(t, finished)
	})
}

func TestNewRollDPoS(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("normal", func(t *testing.T) {
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(newTestAddr()).
			SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetP2P(mock_network.NewMockOverlay(ctrl)).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})
	t.Run("mock-clock", func(t *testing.T) {
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(newTestAddr()).
			SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetP2P(mock_network.NewMockOverlay(ctrl)).
			SetClock(clock.NewMock()).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
		_, ok := r.ctx.clock.(*clock.Mock)
		assert.True(t, ok)
	})
	t.Run("missing-dep", func(t *testing.T) {
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(newTestAddr()).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetP2P(mock_network.NewMockOverlay(ctrl)).
			Build()
		assert.Error(t, err)
		assert.Nil(t, r)
	})
}

func TestRollDPoS_Metrics(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 5)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	blockchain := mock_blockchain.NewMockBlockchain(ctrl)
	blockchain.EXPECT().TipHeight().Return(uint64(8), nil).Times(2)
	blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
		{Address: candidates[0]},
		{Address: candidates[1]},
		{Address: candidates[2]},
		{Address: candidates[3]},
		{Address: candidates[4]},
	}, nil).AnyTimes()

	r, err := NewRollDPoSBuilder().
		SetConfig(config.RollDPoS{NumDelegates: 4}).
		SetAddr(newTestAddr()).
		SetBlockchain(blockchain).
		SetActPool(mock_actpool.NewMockActPool(ctrl)).
		SetP2P(mock_network.NewMockOverlay(ctrl)).
		Build()
	require.NoError(t, err)
	require.NotNil(t, r)

	m, err := r.Metrics()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), m.LatestEpoch)
	crypto.SortCandidates(candidates, m.LatestEpoch)
	assert.Equal(t, candidates[:4], m.LatestDelegates)
	assert.Equal(t, candidates[1], m.LatestBlockProducer)
	assert.Equal(t, candidates, m.Candidates)
}

func TestRollDPoS_convertToConsensusEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r, err := NewRollDPoSBuilder().
		SetConfig(config.RollDPoS{}).
		SetAddr(newTestAddr()).
		SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
		SetActPool(mock_actpool.NewMockActPool(ctrl)).
		SetP2P(mock_network.NewMockOverlay(ctrl)).
		Build()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	// Test propose msg
	addr := newTestAddr()
	transfer, err := action.NewTransfer(1, big.NewInt(100), "src", "dst")
	require.NoError(t, err)
	selfPubKey := testaddress.Addrinfo["producer"].PublicKey
	address, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, selfPubKey)
	require.NoError(t, err)
	vote, err := action.NewVote(2, address.RawAddress, address.RawAddress)
	require.NoError(t, err)
	var prevHash hash.Hash32B
	blk := blockchain.NewBlock(
		1,
		1,
		prevHash,
		clock.New(),
		[]*action.Transfer{transfer}, []*action.Vote{vote},
		nil,
	)
	msg := iproto.ViewChangeMsg{
		Vctype:     iproto.ViewChangeMsg_PROPOSE,
		Block:      blk.ConvertToBlockPb(),
		SenderAddr: addr.RawAddress,
	}
	evt, err := r.convertToConsensusEvt(&msg)
	assert.NoError(t, err)
	assert.NotNil(t, evt)
	pbEvt, ok := evt.(*proposeBlkEvt)
	assert.True(t, ok)
	assert.NotNil(t, pbEvt.block)

	// Test prevote msg
	blkHash := blk.HashBlock()
	msg = iproto.ViewChangeMsg{
		Vctype:     iproto.ViewChangeMsg_PREVOTE,
		BlockHash:  blkHash[:],
		SenderAddr: addr.RawAddress,
	}
	evt, err = r.convertToConsensusEvt(&msg)
	assert.NoError(t, err)
	assert.NotNil(t, evt)
	_, ok = evt.(*voteEvt)
	assert.True(t, ok)

	// Test prevote msg
	msg = iproto.ViewChangeMsg{
		Vctype:     iproto.ViewChangeMsg_VOTE,
		BlockHash:  blkHash[:],
		SenderAddr: addr.RawAddress,
	}
	evt, err = r.convertToConsensusEvt(&msg)
	assert.NoError(t, err)
	assert.NotNil(t, evt)
	_, ok = evt.(*voteEvt)
	assert.True(t, ok)

	// Test invalid msg
	msg = iproto.ViewChangeMsg{
		Vctype: 100,
	}
	evt, err = r.convertToConsensusEvt(&msg)
	assert.Error(t, err)
	assert.Nil(t, evt)
}

func makeTestRollDPoSCtx(
	addr *iotxaddress.Address,
	ctrl *gomock.Controller,
	cfg config.RollDPoS,
	mockChain func(*mock_blockchain.MockBlockchain),
	mockActPool func(*mock_actpool.MockActPool),
	mockP2P func(overlay *mock_network.MockOverlay),
	clock clock.Clock,
) *rollDPoSCtx {
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mockChain(chain)
	actPool := mock_actpool.NewMockActPool(ctrl)
	mockActPool(actPool)
	p2p := mock_network.NewMockOverlay(ctrl)
	mockP2P(p2p)
	return &rollDPoSCtx{
		cfg:     cfg,
		addr:    addr,
		chain:   chain,
		actPool: actPool,
		p2p:     p2p,
		clock:   clock,
	}
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
	if _, ok := msg.(*iproto.ViewChangeMsg); !ok {
		return nil
	}
	for _, r := range o.peers {
		if err := r.Handle(msg); err != nil {
			return errors.Wrap(err, "error when handling the proto msg directly")
		}
	}
	return nil
}

func (o *directOverlay) Tell(net.Addr, proto.Message) error { return nil }

func (o *directOverlay) Self() net.Addr { return o.addr }

func (o *directOverlay) GetPeers() []net.Addr {
	addrs := make([]net.Addr, 0, len(o.peers))
	for addr := range o.peers {
		addrs = append(addrs, addr)
	}
	return addrs
}

func TestRollDPoSConsensus(t *testing.T) {
	t.Parallel()

	newConsensusComponents := func(numNodes int) ([]*RollDPoS, []*directOverlay, []blockchain.Blockchain) {
		cfg := config.Default
		cfg.Consensus.RollDPoS.Delay = 300 * time.Millisecond
		cfg.Consensus.RollDPoS.ProposerInterval = time.Second
		cfg.Consensus.RollDPoS.AcceptProposeTTL = 100 * time.Millisecond
		cfg.Consensus.RollDPoS.AcceptPrevoteTTL = 100 * time.Millisecond
		cfg.Consensus.RollDPoS.AcceptVoteTTL = 100 * time.Millisecond
		cfg.Consensus.RollDPoS.NumDelegates = uint(numNodes)

		chainAddrs := make([]*iotxaddress.Address, 0, numNodes)
		networkAddrs := make([]net.Addr, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			chainAddrs = append(chainAddrs, newTestAddr())
			networkAddrs = append(networkAddrs, node.NewTCPNode(fmt.Sprintf("127.0.0.%d:4689", i+1)))
		}

		chainRawAddrs := make([]string, 0, numNodes)
		addressMap := make(map[string]*iotxaddress.Address, 0)
		for _, addr := range chainAddrs {
			chainRawAddrs = append(chainRawAddrs, addr.RawAddress)
			addressMap[addr.RawAddress] = addr
		}
		crypto.SortCandidates(chainRawAddrs, 1)
		for i, rawAddress := range chainRawAddrs {
			chainAddrs[i] = addressMap[rawAddress]
		}

		candidatesByHeightFunc := func(_ uint64) ([]*state.Candidate, error) {
			candidates := make([]*state.Candidate, 0, numNodes)
			for _, addr := range chainAddrs {
				candidates = append(candidates, &state.Candidate{Address: addr.RawAddress})
			}
			return candidates, nil
		}

		chains := make([]blockchain.Blockchain, 0, numNodes)
		p2ps := make([]*directOverlay, 0, numNodes)
		cs := make([]*RollDPoS, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			chain := blockchain.NewBlockchain(&cfg, blockchain.InMemDaoOption(), blockchain.InMemStateFactoryOption())
			chains = append(chains, chain)

			actPool, err := actpool.NewActPool(chain, cfg.ActPool)
			require.NoError(t, err)

			p2p := &directOverlay{
				addr:  networkAddrs[i],
				peers: make(map[net.Addr]*RollDPoS),
			}
			p2ps = append(p2ps, p2p)

			consensus, err := NewRollDPoSBuilder().
				SetAddr(chainAddrs[i]).
				SetConfig(cfg.Consensus.RollDPoS).
				SetBlockchain(chain).
				SetActPool(actPool).
				SetP2P(p2p).
				SetCandidatesByHeightFunc(candidatesByHeightFunc).
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
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(4)

		for i := 0; i < 4; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 4; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			for _, chain := range chains {
				if blk, err := chain.GetBlockByHeight(1); blk == nil || err != nil {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("10-epoch", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skip the 10-epoch test in short mode.")
		}
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(4)

		for i := 0; i < 4; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 4; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			for _, chain := range chains {
				if blk, err := chain.GetBlockByHeight(10); blk == nil || err != nil {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	checkChains := func(chains []blockchain.Blockchain, height uint64) {
		assert.NoError(t, testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
			for _, chain := range chains {
				blk, err := chain.GetBlockByHeight(height)
				if blk == nil || err != nil {
					return false, nil
				}
				if !blk.IsDummyBlock() {
					return true, errors.New("not a dummy block")
				}
			}
			return true, nil
		}))
	}

	t.Run("proposer-network-partition-dummy-block", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(4)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 4; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 4; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		checkChains(chains, 1)
	})

	t.Run("non-proposer-network-partition-dummy-block", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(4)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 0 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[0].addr)
			}
		}

		for i := 0; i < 4; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 4; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		checkChains(chains, 1)
	})

	t.Run("network-partition-time-rotation", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(4)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 4; i++ {
			cs[i].ctx.cfg.TimeBasedRotation = true
			cs[i].ctx.cfg.EnableDummyBlock = false
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 4; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		checkChains(chains, 4)
	})

	t.Run("proposer-network-partition-blocking", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(4)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 4; i++ {
			cs[i].ctx.cfg.EnableDummyBlock = false
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 4; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		time.Sleep(2 * time.Second)
		for _, chain := range chains {
			blk, err := chain.GetBlockByHeight(1)
			assert.Nil(t, blk)
			assert.Error(t, err)
		}
	})

	t.Run("non-proposer-network-partition-blocking", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(4)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 0 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[0].addr)
			}
		}

		for i := 0; i < 4; i++ {
			cs[i].ctx.cfg.EnableDummyBlock = false
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 4; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		time.Sleep(2 * time.Second)
		for i, chain := range chains {
			blk, err := chain.GetBlockByHeight(1)
			if i == 0 {
				assert.Nil(t, blk)
				assert.Error(t, err)
			} else {
				assert.NotNil(t, blk)
				assert.NoError(t, err)
			}
		}
	})
}
