// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
	"net"
	"testing"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core-internal/config"
	"github.com/iotexproject/iotex-core-internal/network/node"
	"github.com/iotexproject/iotex-core-internal/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core-internal/test/mock/mock_delegate"
)

func TestRollDPoSCtx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := []net.Addr{
		node.NewTCPNode("127.0.0.1:40000"),
		node.NewTCPNode("127.0.0.1:40001"),
		node.NewTCPNode("127.0.0.1:40002"),
		node.NewTCPNode("127.0.0.1:40003"),
	}
	ctx := makeTestRollDPoSCtx(
		ctrl,
		config.RollDPoS{
			NumSubEpochs: 2,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().TipHeight().Return(uint64(8), nil).Times(1)
		},
		func(pool *mock_delegate.MockPool) {
			pool.EXPECT().NumDelegatesPerEpoch().Return(uint(4), nil).Times(1)
			pool.EXPECT().RollDelegates(gomock.Any()).Return(candidates, nil).Times(1)
		},
	)

	epoch, height, err := ctx.calcEpochNumAndHeight()
	require.Nil(t, err)
	assert.Equal(t, uint64(2), epoch)
	assert.Equal(t, uint64(9), height)

	delegates, err := ctx.getRollingDelegates(height)
	require.Nil(t, err)
	assert.Equal(t, candidates, delegates)
}

func makeTestRollDPoSCtx(
	ctrl *gomock.Controller,
	cfg config.RollDPoS,
	mockChain func(*mock_blockchain.MockBlockchain),
	mochPool func(*mock_delegate.MockPool),
) *rollDPoSCtx {
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mockChain(chain)
	pool := mock_delegate.NewMockPool(ctrl)
	mochPool(pool)
	return &rollDPoSCtx{
		cfg:   cfg,
		id:    node.NewTCPNode("127.0.0.1:40000"),
		chain: chain,
		pool:  pool,
		clock: clock.NewMock(),
	}
}
