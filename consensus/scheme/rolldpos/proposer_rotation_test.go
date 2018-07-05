// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/network/node"
)

func TestProposerRotation(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	// arrange 2 consensus nodes
	delegates := []net.Addr{
		node.NewTCPNode("192.168.0.1:10000"),
		node.NewTCPNode("192.168.0.1:10001"),
	}
	m := func(mcks mocks) {
		mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
		mcks.dNet.EXPECT().Broadcast(gomock.Any()).AnyTimes()
		genesis := blockchain.NewGenesisBlock(nil)
		mcks.bc.EXPECT().
			MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(genesis, nil).
			AnyTimes()
		mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).AnyTimes()
	}
	cs := createTestRollDPoS(
		ctrl, delegates[0], delegates, m, FixedProposer, 10*time.Millisecond, NeverStartNewEpoch, nil)
	cs.Start(ctx)
	defer cs.Stop(ctx)
	cs.enqueueEvent(&fsm.Event{State: stateDKGGenerate})

	waitFor(
		t,
		func() bool { return cs.roundCtx != nil && cs.roundCtx.isPr },
		2*time.Second,
		"proposer is not elected")
	require.NotNil(t, cs.roundCtx)
	require.Equal(t, true, cs.roundCtx.isPr)
}

func TestFixedProposer(t *testing.T) {
	delegates := []net.Addr{
		node.NewTCPNode("192.168.0.1:10000"),
		node.NewTCPNode("192.168.0.1:10001"),
	}

	pr, err := FixedProposer(delegates, nil, 0, 0)
	require.Nil(t, err)
	require.Equal(t, delegates[0].String(), pr.String())
	require.NotEqual(t, delegates[1].String(), pr.String())

	pr, err = FixedProposer(make([]net.Addr, 0), nil, 0, 0)
	require.Nil(t, pr)
	require.Equal(t, delegate.ErrZeroDelegate, err)
}

func TestPseudoRotatedProposer(t *testing.T) {
	delegates := []net.Addr{
		node.NewTCPNode("192.168.0.1:10000"),
		node.NewTCPNode("192.168.0.1:10001"),
		node.NewTCPNode("192.168.0.1:10002"),
		node.NewTCPNode("192.168.0.1:10003"),
	}

	for i := 0; i < 4; i++ {
		pr, err := PseudoRotatedProposer(delegates, nil, 0, 10000+uint64(i))
		require.Nil(t, err)
		require.Equal(t, delegates[i].String(), pr.String())
	}
}
