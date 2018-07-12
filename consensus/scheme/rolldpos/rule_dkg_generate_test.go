// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
)

func TestRuleDKGGenerateCondition(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	addr := "io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh"
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().TipHeight().Return(uint64(16), nil).Times(1)
	pool := mock_delegate.NewMockPool(ctrl)
	pool.EXPECT().NumDelegatesPerEpoch().Return(uint(4), nil).Times(4)
	delegates := []string{
		"io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh",
		"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
		"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
		"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
	}
	pool.EXPECT().RollDelegates(gomock.Any()).Return(delegates, nil).Times(2)

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

	// The epoch should be correctly set even the node misses some early blocks for the epoch
	bc.EXPECT().TipHeight().Return(uint64(18), nil).Times(1)

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
