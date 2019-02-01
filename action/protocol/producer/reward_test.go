// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state/factory"
)

func TestProtocol_GrantReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)

		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Donate(ctx, ws, big.NewInt(200), []byte("hello, world")))
		require.NoError(t, stateDB.Commit(ws))

		// Grant block reward
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.GrantBlockReward(ctx, ws))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		availableBalance, err := p.AvailableBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(190), availableBalance)
		unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, raCtx.Producer)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), unclaimedBalance)

		// Grant the same block reward again will fail
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.GrantBlockReward(ctx, ws))

		// Grant epoch reward
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.GrantEpochReward(ctx, ws))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		availableBalance, err = p.AvailableBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(90), availableBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, raCtx.Producer)
		require.NoError(t, err)
		// TODO: assert unclaimedBalance after splitting epoch reward correctly

		// Grant the same epoch reward again will fail
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.GrantEpochReward(ctx, ws))
	})
}
