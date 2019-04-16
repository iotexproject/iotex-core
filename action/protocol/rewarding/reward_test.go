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

	"github.com/gogo/protobuf/proto"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding/rewardingpb"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestProtocol_GrantBlockReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)

		// Grant block reward will fail because of no available balance
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		_, err = p.GrantBlockReward(ctx, ws)
		require.Error(t, err)

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Deposit(ctx, ws, big.NewInt(200)))
		require.NoError(t, stateDB.Commit(ws))

		// Grant block reward
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		rewardLog, err := p.GrantBlockReward(ctx, ws)
		require.NoError(t, err)
		require.Equal(t, p.addr.String(), rewardLog.Address)
		var rl rewardingpb.RewardLog
		require.NoError(t, proto.Unmarshal(rewardLog.Data, &rl))
		require.Equal(t, rewardingpb.RewardLog_BLOCK_REWARD, rl.Type)
		require.Equal(t, "10", rl.Amount)
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		availableBalance, err := p.AvailableBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(190), availableBalance)
		// Operator shouldn't get reward
		unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, raCtx.Producer)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		// Beneficiary should get reward
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, identityset.Address(0))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), unclaimedBalance)

		// Grant the same block reward again will fail
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		_, err = p.GrantBlockReward(ctx, ws)
		require.Error(t, err)
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
		rewardLogs, err := p.GrantEpochReward(ctx, ws)
		require.NoError(t, err)
		require.Equal(t, 8, len(rewardLogs))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		availableBalance, err := p.AvailableBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(90+5), availableBalance)
		// Operator shouldn't get reward
		unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["producer"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		// Beneficiary should get reward
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, identityset.Address(0))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(40+5), unclaimedBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["alfa"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(30+5), unclaimedBalance)
		// The 3-th candidate can't get the reward because it doesn't meet the productivity requirement
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["bravo"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["charlie"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10+5), unclaimedBalance)
		// The 5-th candidate can't get the reward because of being out of the range
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["delta"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		// The 6-th candidate can't get the foundation bonus because of being out of the range
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["echo"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)

		// Assert logs
		expectedResults := []struct {
			t      rewardingpb.RewardLog_RewardType
			addr   string
			amount string
		}{
			{
				rewardingpb.RewardLog_EPOCH_REWARD,
				identityset.Address(0).String(),
				"40",
			},
			{
				rewardingpb.RewardLog_EPOCH_REWARD,
				testaddress.Addrinfo["alfa"].String(),
				"30",
			},
			{
				rewardingpb.RewardLog_EPOCH_REWARD,
				testaddress.Addrinfo["charlie"].String(),
				"10",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				identityset.Address(0).String(),
				"5",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				testaddress.Addrinfo["alfa"].String(),
				"5",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				testaddress.Addrinfo["bravo"].String(),
				"5",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				testaddress.Addrinfo["charlie"].String(),
				"5",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				testaddress.Addrinfo["delta"].String(),
				"5",
			},
		}
		for i := 0; i < 8; i++ {
			require.Equal(t, p.addr.String(), rewardLogs[i].Address)
			var rl rewardingpb.RewardLog
			require.NoError(t, proto.Unmarshal(rewardLogs[i].Data, &rl))
			assert.Equal(t, expectedResults[i].t, rl.Type)
			assert.Equal(t, expectedResults[i].addr, rl.Addr)
			assert.Equal(t, expectedResults[i].amount, rl.Amount)
		}

		// Grant the same epoch reward again will fail
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		_, err = p.GrantEpochReward(ctx, ws)
		require.Error(t, err)

		// Grant the epoch reward on a block that is not the last one in an epoch will fail
		raCtx.BlockHeight++
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		_, err = p.GrantEpochReward(protocol.WithRunActionsCtx(context.Background(), raCtx), ws)
		require.Error(t, err)
	}, false)

	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Deposit(ctx, ws, big.NewInt(200)))
		require.NoError(t, stateDB.Commit(ws))

		// Grant epoch reward
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		_, err = p.GrantEpochReward(ctx, ws)
		require.NoError(t, err)
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		// The 5-th candidate can't get the reward because exempting from the epoch reward
		unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["delta"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		// The 6-th candidate can get the foundation bonus because it's still within the rage after excluding 5-th one
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, testaddress.Addrinfo["echo"])
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(4+5), unclaimedBalance)
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
		rewardLog, err := p.GrantBlockReward(ctx, ws)
		require.NoError(t, err)
		require.Equal(t, p.addr.String(), rewardLog.Address)
		var rl rewardingpb.RewardLog
		require.NoError(t, proto.Unmarshal(rewardLog.Data, &rl))
		require.Equal(t, rewardingpb.RewardLog_BLOCK_REWARD, rl.Type)
		require.Equal(t, "10", rl.Amount)
		require.NoError(t, stateDB.Commit(ws))

		// Claim 5 token
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)
		claimRaCtx := raCtx
		claimRaCtx.Caller = identityset.Address(0)
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
		unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, claimRaCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		primAcc, err := accountutil.LoadAccount(ws, hash.BytesToHash160(claimRaCtx.Caller.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), primAcc.Balance)

		// Claim negative amount of token will fail
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.Claim(claimCtx, ws, big.NewInt(-5)))

		// Claim 0 amount won't fail, but also will not get the token
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.Claim(claimCtx, ws, big.NewInt(0)))
		require.NoError(t, stateDB.Commit(ws))

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		totalBalance, err = p.TotalBalance(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(15), totalBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, claimRaCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		primAcc, err = accountutil.LoadAccount(ws, hash.BytesToHash160(claimRaCtx.Caller.Bytes()))
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
		unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, claimRaCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		primAcc, err = accountutil.LoadAccount(ws, hash.BytesToHash160(claimRaCtx.Caller.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), primAcc.Balance)

		// Claim the 3-rd 5 token will fail be cause no balance for the address
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.Claim(claimCtx, ws, big.NewInt(5)))

		// Operator should have nothing to claim
		claimRaCtx.Caller = raCtx.Producer
		claimCtx = protocol.WithRunActionsCtx(context.Background(), claimRaCtx)
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.Claim(claimCtx, ws, big.NewInt(1)))
	}, false)
}

func TestProtocol_NoRewardAddr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer func() {
		require.NoError(t, stateDB.Stop(context.Background()))
	}()

	chain := mock_chainmanager.NewMockChainManager(ctrl)
	chain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
		{
			Address:       identityset.Address(0).String(),
			Votes:         unit.ConvertIotxToRau(1000000),
			RewardAddress: "",
		},
		{
			Address:       identityset.Address(1).String(),
			Votes:         unit.ConvertIotxToRau(1000000),
			RewardAddress: identityset.Address(1).String(),
		},
	}, nil).AnyTimes()
	chain.EXPECT().ProductivityByEpoch(gomock.Any()).Return(
		uint64(19),
		map[string]uint64{
			identityset.Address(0).String(): 9,
			identityset.Address(1).String(): 10,
		},
		nil,
	).AnyTimes()
	p := NewProtocol(chain, rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	))

	// Initialize the protocol
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 0,
		},
	)
	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(
		t,
		p.Initialize(
			ctx,
			ws,
			big.NewInt(0),
			big.NewInt(10),
			big.NewInt(100),
			10,
			nil,
			big.NewInt(5),
			5,
			365,
			50,
		))
	require.NoError(t, stateDB.Commit(ws))

	// Create a test account with 1000 token
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, identityset.Address(0).String(), big.NewInt(1000))
	require.NoError(t, err)
	require.NoError(t, stateDB.Commit(ws))

	ctx = protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    identityset.Address(0),
			Caller:      identityset.Address(0),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
		},
	)
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, p.Deposit(ctx, ws, big.NewInt(200)))
	require.NoError(t, stateDB.Commit(ws))

	// Grant block reward
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	rewardLog, err := p.GrantBlockReward(ctx, ws)
	require.NoError(t, err)
	require.Nil(t, rewardLog)
	require.NoError(t, stateDB.Commit(ws))

	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	availableBalance, err := p.AvailableBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(200), availableBalance)
	unclaimedBalance, err := p.UnclaimedBalance(ctx, ws, identityset.Address(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), unclaimedBalance)

	// Grant epoch reward
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	rewardLogs, err := p.GrantEpochReward(ctx, ws)
	require.NoError(t, err)
	require.Equal(t, 2, len(rewardLogs))
	require.NoError(t, stateDB.Commit(ws))

	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	availableBalance, err = p.AvailableBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(145), availableBalance)
	unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, identityset.Address(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), unclaimedBalance)
	// It doesn't affect others to get reward
	unclaimedBalance, err = p.UnclaimedBalance(ctx, ws, identityset.Address(1))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(55), unclaimedBalance)
	require.Equal(t, p.addr.String(), rewardLogs[0].Address)
	var rl rewardingpb.RewardLog
	require.NoError(t, proto.Unmarshal(rewardLogs[0].Data, &rl))
	assert.Equal(t, rewardingpb.RewardLog_EPOCH_REWARD, rl.Type)
	assert.Equal(t, identityset.Address(1).String(), rl.Addr)
	assert.Equal(t, "50", rl.Amount)
	require.Equal(t, p.addr.String(), rewardLogs[1].Address)
	require.NoError(t, proto.Unmarshal(rewardLogs[1].Data, &rl))
	assert.Equal(t, rewardingpb.RewardLog_FOUNDATION_BONUS, rl.Type)
	assert.Equal(t, identityset.Address(1).String(), rl.Addr)
	assert.Equal(t, "5", rl.Amount)
}
