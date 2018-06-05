// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"

	bc "github.com/iotexproject/iotex-core/blockchain"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
	"time"
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
		genesis := bc.NewGenesisBlock(bc.Gen)
		mcks.bc.EXPECT().MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any()).Return(genesis, nil).AnyTimes()
		mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).AnyTimes()
	}
	cs := createTestRollDPoS(ctrl, delegates[0], delegates, m, true, FixedProposer, nil)
	cs.Start()
	defer cs.Stop()
	cs.handleEvent(&fsm.Event{State: stateDKGGenerate})

	waitFor(
		t,
		func() bool { return cs.roundCtx != nil && cs.roundCtx.isPr },
		2*time.Second,
		"proposer is not elected")
	assert.NotNil(t, cs.roundCtx)
	assert.Equal(t, true, cs.roundCtx.isPr)
}

func TestFixedProposer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.1:10000"),
		cm.NewTCPNode("192.168.0.1:10001"),
	}
	pool := mock_delegate.NewMockPool(ctrl)
	pool.EXPECT().AllDelegates().Return(delegates, nil).Times(1)

	pr, err := FixedProposer(pool, nil, 0, 0)
	assert.Nil(t, err)
	assert.Equal(t, delegates[0].String(), pr.String())
	assert.NotEqual(t, delegates[1].String(), pr.String())

	pool.EXPECT().AllDelegates().Return(nil, delegate.ErrZeroDelegate).Times(1)

	pr, err = FixedProposer(pool, nil, 0, 0)
	assert.Nil(t, pr)
	assert.Equal(t, delegate.ErrZeroDelegate, err)
}

func TestPseudoRotatedProposer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.1:10000"),
		cm.NewTCPNode("192.168.0.1:10001"),
		cm.NewTCPNode("192.168.0.1:10002"),
		cm.NewTCPNode("192.168.0.1:10003"),
	}
	pool := mock_delegate.NewMockPool(ctrl)
	pool.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()

	for i := 0; i < 4; i++ {
		pr, err := PseudoRotatedProposer(pool, nil, 0, 10000+uint64(i))
		assert.Nil(t, err)
		assert.Equal(t, delegates[i].String(), pr.String())
	}
}
