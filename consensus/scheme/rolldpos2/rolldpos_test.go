// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
	"math/big"
	"testing"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
	"github.com/iotexproject/iotex-core/test/mock/mock_network"
)

func TestRollDPoSCtx(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 4)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}
	var prevHash hash.Hash32B
	blk := blockchain.NewBlock(1, 8, prevHash, make([]*action.Transfer, 0), make([]*action.Vote, 0))
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
				&state.Candidate{Address: candidates[0]},
				&state.Candidate{Address: candidates[1]},
				&state.Candidate{Address: candidates[2]},
				&state.Candidate{Address: candidates[3]},
			}, true).Times(1)
		},
		func(pool *mock_delegate.MockPool) {},
		func(_ *mock_actpool.MockActPool) {},
		func(_ *mock_network.MockOverlay) {},
	)

	epoch, height, err := ctx.calcEpochNumAndHeight()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), epoch)
	assert.Equal(t, uint64(9), height)

	delegates, err := ctx.rollingDelegates(height)
	require.NoError(t, err)
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

	duration, err := ctx.calcDurationSinceLastBlock()
	require.NoError(t, err)
	assert.True(t, duration > 0)

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
			func(_ *mock_delegate.MockPool) {},
			func(_ *mock_actpool.MockActPool) {},
			func(_ *mock_network.MockOverlay) {},
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
			func(_ *mock_delegate.MockPool) {},
			func(_ *mock_actpool.MockActPool) {},
			func(_ *mock_network.MockOverlay) {},
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
			SetDelegatePool(mock_delegate.NewMockPool(ctrl)).
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
			SetDelegatePool(mock_delegate.NewMockPool(ctrl)).
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
			SetDelegatePool(mock_delegate.NewMockPool(ctrl)).
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
	}, true).AnyTimes()

	pool := mock_delegate.NewMockPool(ctrl)
	pool.EXPECT().AllDelegates().Return(candidates, nil).Times(1)

	r, err := NewRollDPoSBuilder().
		SetConfig(config.RollDPoS{NumDelegates: 4}).
		SetAddr(newTestAddr()).
		SetBlockchain(blockchain).
		SetActPool(mock_actpool.NewMockActPool(ctrl)).
		SetDelegatePool(pool).
		SetP2P(mock_network.NewMockOverlay(ctrl)).
		Build()
	require.NoError(t, err)
	require.NotNil(t, r)

	m, err := r.Metrics()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), m.LatestEpoch)
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
		SetDelegatePool(mock_delegate.NewMockPool(ctrl)).
		SetP2P(mock_network.NewMockOverlay(ctrl)).
		Build()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	// Test propose msg
	addr := newTestAddr()
	transfer := action.NewTransfer(1, big.NewInt(100), "src", "dst")
	vote := action.NewVote(2, []byte("src"), []byte("dst"))
	var prevHash hash.Hash32B
	blk := blockchain.NewBlock(1, 1, prevHash, []*action.Transfer{transfer}, []*action.Vote{vote})
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
	mockPool func(*mock_delegate.MockPool),
	mockActPool func(*mock_actpool.MockActPool),
	mockP2P func(overlay *mock_network.MockOverlay),
) *rollDPoSCtx {
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mockChain(chain)
	pool := mock_delegate.NewMockPool(ctrl)
	mockPool(pool)
	actPool := mock_actpool.NewMockActPool(ctrl)
	mockActPool(actPool)
	p2p := mock_network.NewMockOverlay(ctrl)
	mockP2P(p2p)
	return &rollDPoSCtx{
		cfg:     cfg,
		addr:    addr,
		chain:   chain,
		pool:    pool,
		actPool: actPool,
		p2p:     p2p,
		clock:   clock.NewMock(),
	}
}
