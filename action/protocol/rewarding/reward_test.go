// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state/factory"
)

func TestProtocol_GrantReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)

		// Grant block reward will fail because of no available balance
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.GrantBlockReward(ctx, ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Donate(ctx, ws, big.NewInt(200)))
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

func TestProtocol_ClaimReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		// Donate 20 token into the rewarding fund
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Donate(ctx, ws, big.NewInt(20)))
		require.NoError(t, stateDB.Commit(ws))

		// Grant block reward
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.GrantBlockReward(ctx, ws))
		require.NoError(t, stateDB.Commit(ws))

		// Claim 5 token
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)
		claimRaCtx := raCtx
		claimRaCtx.Caller = raCtx.Producer
		claimCtx := protocol.WithRunActionsCtx(context.Background(), claimRaCtx)

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Claim(claimCtx, ws, big.NewInt(5)))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		totalBalance, err := p.TotalBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(15), totalBalance)
		unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, raCtx.Producer)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		primAcc, err := account.LoadAccount(ws, byteutil.BytesTo20B(raCtx.Producer.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), primAcc.Balance)

		// Claim another 5 token
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Claim(claimCtx, ws, big.NewInt(5)))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		totalBalance, err = p.TotalBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), totalBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, raCtx.Producer)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		primAcc, err = account.LoadAccount(ws, byteutil.BytesTo20B(raCtx.Producer.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), primAcc.Balance)

		// Claim the 3-rd 5 token will fail be cause no balance for the address
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.Claim(claimCtx, ws, big.NewInt(5)))
	})
}
