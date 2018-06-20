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
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
)

func TestRuleDKGGenerateCondition(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	addr := common.NewTCPNode("127.0.0.1:40001")
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().TipHeight().Return(uint64(16), nil).Times(1)
	pool := mock_delegate.NewMockPool(ctrl)
	pool.EXPECT().NumDelegatesPerEpoch().Return(uint(4), nil).Times(1)
	delegates := []net.Addr{
		common.NewTCPNode("127.0.0.1:40001"),
		common.NewTCPNode("127.0.0.1:40002"),
		common.NewTCPNode("127.0.0.1:40003"),
		common.NewTCPNode("127.0.0.1:40004"),
	}
	pool.EXPECT().RollDelegates(gomock.Any()).Return(delegates, nil).Times(1)

	h := ruleDKGGenerate{
		RollDPoS: &RollDPoS{
			self: addr,
			cfg: config.RollDPoS{
				ProposerInterval: time.Millisecond,
				NumSubEpochs:     2,
			},
			bc:        bc,
			eventChan: make(chan *fsm.Event, 1),
			pool:      pool,
		},
	}
	h.RollDPoS.prnd = &proposerRotation{RollDPoS: h.RollDPoS}

	require.True(
		t,
		h.Condition(&fsm.Event{
			State: stateRoundStart,
		}),
	)
	require.NotNil(t, h.epochCtx)
	require.Equal(t, uint64(3), h.epochCtx.num)
	require.Equal(t, uint64(17), h.epochCtx.height)
	require.Equal(t, delegates, h.epochCtx.delegates)

}
