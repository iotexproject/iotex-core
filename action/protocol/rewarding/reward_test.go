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

	"github.com/iotexproject/iotex-core/test/testaddress"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state/factory"
)

func TestProtocol_GrantBlockReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)

		// Grant block reward will fail because of no available balance
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.GrantBlockReward(ctx, ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Deposit(ctx, ws, big.NewInt(200)))
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
	}, false)
}

func TestProtocol_GrantEpochReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)

		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Deposit(ctx, ws, big.NewInt(200)))
		require.NoError(t, stateDB.Commit(ws))

		// Grant epoch reward
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.GrantEpochReward(ctx, ws))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		availableBalance, err := p.AvailableBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(100), availableBalance)
		unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["producer"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(45), unclaimedBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["alfa"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(35), unclaimedBalance)
		// The 3-th candidate can't get the reward because it doesn't meet the productivity requirement
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["bravo"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["charlie"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(15), unclaimedBalance)
		// The 5-th candidate can't get the reward because being out of the range
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["delta"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)

		// Grant the same epoch reward again will fail
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.GrantEpochReward(ctx, ws))

		// Grant the epoch reward on a block that is not the last one in an epoch will fail
		raCtx.BlockHeight++
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.GrantEpochReward(protocol.WithRunActionsCtx(context.Background(), raCtx), ws))
	}, false)

	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Deposit(ctx, ws, big.NewInt(200)))
		require.NoError(t, stateDB.Commit(ws))

		// Grant epoch reward
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.GrantEpochReward(ctx, ws))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		// The 5-th candidate can't get the reward because excempting from the epoch reward
		unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["delta"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
	}, true)
}

func TestProtocol_ClaimReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		// Deposit 20 token into the rewarding fund
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Deposit(ctx, ws, big.NewInt(20)))
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
		primAcc, err := accountutil.LoadAccount(ws, hash.BytesToHash160(raCtx.Producer.Bytes()))
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
		primAcc, err = accountutil.LoadAccount(ws, hash.BytesToHash160(raCtx.Producer.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), primAcc.Balance)

		// Claim the 3-rd 5 token will fail be cause no balance for the address
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.Claim(claimCtx, ws, big.NewInt(5)))
	}, false)
}
