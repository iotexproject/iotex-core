// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
	"github.com/iotexproject/iotex-core/test/mock/mock_state"
)

func TestStartNextEpochCB(t *testing.T) {
	t.Parallel()

	self := common.NewTCPNode("127.0.0.1:40000")
	ctrl := gomock.NewController(t)
	sf := mock_state.NewMockFactory(ctrl)
	pool := mock_delegate.NewMockPool(ctrl)
	defer ctrl.Finish()

	flag, err := NeverStartNewEpoch(self, 1, sf, pool)
	require.Nil(t, err)
	require.False(t, flag)

	flag, err = PseudoStarNewEpoch(self, 1, sf, pool)
	require.Nil(t, err)
	require.True(t, flag)

	// Among the rolling delegates
	pool.EXPECT().RollDelegates(gomock.Any()).Return(
		[]net.Addr{
			common.NewTCPNode("127.0.0.1:40000"),
			common.NewTCPNode("127.0.0.1:40001"),
			common.NewTCPNode("127.0.0.1:40002"),
			common.NewTCPNode("127.0.0.1:40003"),
		},
		nil,
	).Times(1)
	flag, err = PseudoStartRollingEpoch(self, 1, sf, pool)
	require.Nil(t, err)
	require.True(t, flag)

	// Not among the rolling delegates
	pool.EXPECT().RollDelegates(gomock.Any()).Return(
		[]net.Addr{
			common.NewTCPNode("127.0.0.1:40001"),
			common.NewTCPNode("127.0.0.1:40002"),
			common.NewTCPNode("127.0.0.1:40003"),
		},
		nil,
	).Times(1)
	flag, err = PseudoStartRollingEpoch(self, 1, sf, pool)
	require.Nil(t, err)
	require.False(t, flag)
}
