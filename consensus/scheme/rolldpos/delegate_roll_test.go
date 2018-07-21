// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
)

func TestStartNextEpochCB(t *testing.T) {
	t.Parallel()

	self := "io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh"
	ctrl := gomock.NewController(t)
	pool := mock_delegate.NewMockPool(ctrl)
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	cfg := &config.RollDPoS{}
	defer ctrl.Finish()

	flag, err := NeverStartNewEpoch(self, 1, pool, bc, cfg)
	require.Nil(t, err)
	require.False(t, flag)

	flag, err = PseudoStarNewEpoch(self, 1, pool, bc, cfg)
	require.Nil(t, err)
	require.True(t, flag)

	// Among the rolling delegates
	pool.EXPECT().RollDelegates(gomock.Any()).Return(
		[]string{
			"io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh",
			"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
			"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
			"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
		},
		nil,
	).Times(1)
	flag, err = PseudoStartRollingEpoch(self, 1, pool, bc, cfg)
	require.Nil(t, err)
	require.True(t, flag)

	// Not among the rolling delegates
	pool.EXPECT().RollDelegates(gomock.Any()).Return(
		[]string{
			"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
			"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
			"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
		},
		nil,
	).Times(1)
	flag, err = PseudoStartRollingEpoch(self, 1, pool, bc, cfg)
	require.Nil(t, err)
	require.False(t, flag)
}
