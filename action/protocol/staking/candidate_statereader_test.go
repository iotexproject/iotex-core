// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
	"github.com/stretchr/testify/require"
)

func Test_CandidateStateReader(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	h, err := sm.Height()
	require.NoError(err)
	csr, err := ConstructBaseView(sm)
	require.Equal(err, protocol.ErrNoName)
	view, _, err := CreateBaseView(sm, false)
	require.NoError(err)
	csr = &candSR{
		StateReader: sm,
		height:      h,
		view:        view,
	}

	require.Equal(csr.Height(), h)
	require.Equal(csr.SR(), sm)
	require.Equal(len(csr.AllCandidates()), 0)
	require.Equal(csr.TotalStakedAmount(), big.NewInt(0))
	require.Equal(csr.ActiveBucketsCount(), uint64(0))
}
