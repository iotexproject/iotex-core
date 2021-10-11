// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
)

func TestAdminPb(t *testing.T) {
	r := require.New(t)

	// actual data of admin.v1 on mainnet
	b, err := hex.DecodeString("0a133830303030303030303030303030303030303012173138373530303030303030303030303030303030303030186422143830303030303030303030303030303030303030282430b8443855")
	r.NoError(err)
	a := admin{}
	r.NoError(a.Deserialize(b))

	g := genesis.Default
	r.Equal(a.blockReward.String(), g.DardanellesBlockRewardStr)
	r.Equal(a.epochReward.String(), g.AleutianEpochRewardStr)
	r.Equal(a.numDelegatesForEpochReward, g.NumDelegatesForEpochReward)
	r.Equal(a.foundationBonus.String(), g.FoundationBonusStr)
	r.Equal(a.numDelegatesForFoundationBonus, g.NumDelegatesForFoundationBonus)
	r.Equal(a.foundationBonusLastEpoch, g.FoundationBonusLastEpoch)
	r.EqualValues(85, a.productivityThreshold)
	r.False(a.hasFoundationBonusExtension())

	// add foundation bonus extension
	a.foundationBonusExtension = config.Default.Genesis.Rewarding.FoundationBonusExtension
	b1, err := a.Serialize()
	r.NoError(err)
	r.Equal(b, b1[:len(b)])
	a1 := admin{}
	r.NoError(a1.Deserialize(b1))
	r.True(a1.hasFoundationBonusExtension())
	r.Equal(a, a1)
}

func TestProtocol_SetEpochReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		amount, err := p.EpochReward(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(100), amount)

		require.NoError(t, p.SetReward(ctx, sm, big.NewInt(200), false))

		amount, err = p.EpochReward(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(200), amount)

	}, false)
}
