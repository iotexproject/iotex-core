// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrandReward(t *testing.T) {
	require := require.New(t)
	tests := []struct {
		rewardType int
		height     uint64
	}{
		{BlockReward, 100},
		{EpochReward, 200},
	}
	for _, test := range tests {
		g := &GrantReward{
			rewardType: test.rewardType,
			height:     test.height,
		}
		require.Equal(test.rewardType, g.RewardType())
		require.Equal(test.height, g.Height())
		require.NoError(g.SanityCheck())
		intrinsicGas, err := g.IntrinsicGas()
		require.NoError(err)
		require.Zero(intrinsicGas)
		elp := (&EnvelopeBuilder{}).SetGasPrice(_defaultGasPrice).
			SetAction(g).Build()
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal(big.NewInt(0), cost)

		g2 := &GrantReward{}
		require.NoError(g2.LoadProto(g.Proto()))
		require.Equal(g, g2)
	}
}
