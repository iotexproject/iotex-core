// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
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
	for i := 0; i < 4; i++ {
		candidates[i] = testAddrs[i].RawAddress
	}
	var prevHash hash.Hash32B
	blk := blockchain.NewBlock(1, 8, prevHash, make([]*action.Transfer, 0), make([]*action.Vote, 0))
	ctx := makeTestRollDPoSCtx(
		testAddrs[0],
		ctrl,
		config.RollDPoS{
			NumSubEpochs: 2,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().TipHeight().Return(uint64(8), nil).Times(3)
			blockchain.EXPECT().GetBlockByHeight(uint64(8)).Return(blk, nil).Times(1)
		},
		func(pool *mock_delegate.MockPool) {
			pool.EXPECT().NumDelegatesPerEpoch().Return(uint(4), nil).Times(1)
			pool.EXPECT().RollDelegates(gomock.Any()).Return(candidates, nil).Times(1)
		},
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
