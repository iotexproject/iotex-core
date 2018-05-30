// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	bc "github.com/iotexproject/iotex-core/blockchain"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
)

func TestProposerRotation(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange 2 consensus nodes
	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.1:10000"),
		cm.NewTCPNode("192.168.0.1:10001"),
	}
	m := func(mcks mocks) {
		mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
		mcks.dNet.EXPECT().Broadcast(gomock.Any()).AnyTimes()
		mcks.bc.EXPECT().TipHeight().AnyTimes()
		genesis := bc.NewGenesisBlock(bc.Gen)
		mcks.bc.EXPECT().MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any()).Return(genesis, nil).AnyTimes()
	}
	cs := createTestRollDPoS(ctrl, delegates[0], delegates, m, true)
	cs.Start()
	defer cs.Stop()

	time.Sleep(2 * time.Second)

	assert.NotNil(t, cs.roundCtx)
	assert.Equal(t, true, cs.roundCtx.isPr)
	assert.Equal(t, delegates, cs.roundCtx.delegates)
}

func TestFixedProposer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.1:10000"),
		cm.NewTCPNode("192.168.0.1:10001"),
	}
	pool := mock_delegate.NewMockPool(ctrl)
	pool.EXPECT().AllDelegates().Return(delegates, nil).Times(2)

	isPr, err := FixedProposer(delegates[0], pool, nil, 0)
	assert.Nil(t, err)
	assert.True(t, isPr)

	isPr, err = FixedProposer(delegates[1], pool, nil, 0)
	assert.Nil(t, err)
	assert.False(t, isPr)

	pool.EXPECT().AllDelegates().Return(nil, delegate.ErrZeroDelegate).Times(1)

	isPr, err = FixedProposer(delegates[1], pool, nil, 0)
	assert.Equal(t, delegate.ErrZeroDelegate, err)
	assert.False(t, isPr)
}
