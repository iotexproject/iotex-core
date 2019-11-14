// Copyright (c) 2019 IoTeX Foundation
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
	"github.com/iotexproject/iotex-core/config"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

func TestProtocol_GrantBlockReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)

		// Grant block reward will fail because of no available balance
		_, err := p.GrantBlockReward(ctx, sm)
		require.Error(t, err)

		require.NoError(t, p.Deposit(ctx, sm, big.NewInt(200)))

		// Grant block reward
		rewardLog, err := p.GrantBlockReward(ctx, sm)
		require.NoError(t, err)
		require.Equal(t, p.addr.String(), rewardLog.Address)
		var rl rewardingpb.RewardLog
		require.NoError(t, proto.Unmarshal(rewardLog.Data, &rl))
		require.Equal(t, rewardingpb.RewardLog_BLOCK_REWARD, rl.Type)
		require.Equal(t, "10", rl.Amount)

		availableBalance, err := p.AvailableBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(190), availableBalance)
		// Operator shouldn't get reward
		unclaimedBalance, err := p.UnclaimedBalance(ctx, sm, raCtx.Producer)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		// Beneficiary should get reward
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(0))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), unclaimedBalance)

		// Grant the same block reward again will fail
		_, err = p.GrantBlockReward(ctx, sm)
		require.Error(t, err)
	}, false)
}

func TestProtocol_GrantEpochReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)

		require.NoError(t, p.Deposit(ctx, sm, big.NewInt(200)))

		// Grant epoch reward
		rewardLogs, err := p.GrantEpochReward(ctx, sm)
		require.NoError(t, err)
		require.Equal(t, 8, len(rewardLogs))

		availableBalance, err := p.AvailableBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(90+5), availableBalance)
		// Operator shouldn't get reward
		unclaimedBalance, err := p.UnclaimedBalance(ctx, sm, identityset.Address(27))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		// Beneficiary should get reward
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(0))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(40+5), unclaimedBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(28))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(30+5), unclaimedBalance)
		// The 3-th candidate can't get the reward because it doesn't meet the productivity requirement
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(29))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(30))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10+5), unclaimedBalance)
		// The 5-th candidate can't get the reward because of being out of the range
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(31))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		// The 6-th candidate can't get the foundation bonus because of being out of the range
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(32))
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
				identityset.Address(28).String(),
				"30",
			},
			{
				rewardingpb.RewardLog_EPOCH_REWARD,
				identityset.Address(30).String(),
				"10",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				identityset.Address(0).String(),
				"5",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				identityset.Address(28).String(),
				"5",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				identityset.Address(29).String(),
				"5",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				identityset.Address(30).String(),
				"5",
			},
			{
				rewardingpb.RewardLog_FOUNDATION_BONUS,
				identityset.Address(31).String(),
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
		_, err = p.GrantEpochReward(ctx, sm)
		require.Error(t, err)

		// Grant the epoch reward on a block that is not the last one in an epoch will fail
		raCtx.BlockHeight++
		_, err = p.GrantEpochReward(protocol.WithRunActionsCtx(context.Background(), raCtx), sm)
		require.Error(t, err)
	}, false)

	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		require.NoError(t, p.Deposit(ctx, sm, big.NewInt(200)))

		// Grant epoch reward
		_, err := p.GrantEpochReward(ctx, sm)
		require.NoError(t, err)

		// The 5-th candidate can't get the reward because exempting from the epoch reward
		unclaimedBalance, err := p.UnclaimedBalance(ctx, sm, identityset.Address(31))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		// The 6-th candidate can get the foundation bonus because it's still within the rage after excluding 5-th one
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(32))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(4+5), unclaimedBalance)
	}, true)
}

func TestProtocol_ClaimReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		// Deposit 20 token into the rewarding fund
		require.NoError(t, p.Deposit(ctx, sm, big.NewInt(20)))

		// Grant block reward
		rewardLog, err := p.GrantBlockReward(ctx, sm)
		require.NoError(t, err)
		require.Equal(t, p.addr.String(), rewardLog.Address)
		var rl rewardingpb.RewardLog
		require.NoError(t, proto.Unmarshal(rewardLog.Data, &rl))
		require.Equal(t, rewardingpb.RewardLog_BLOCK_REWARD, rl.Type)
		require.Equal(t, "10", rl.Amount)

		// Claim 5 token
		raCtx, ok := protocol.GetRunActionsCtx(ctx)
		require.True(t, ok)
		claimRaCtx := raCtx
		claimRaCtx.Caller = identityset.Address(0)
		claimCtx := protocol.WithRunActionsCtx(context.Background(), claimRaCtx)

		require.NoError(t, p.Claim(claimCtx, sm, big.NewInt(5)))

		totalBalance, err := p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(15), totalBalance)
		unclaimedBalance, err := p.UnclaimedBalance(ctx, sm, claimRaCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		primAcc, err := accountutil.LoadAccount(sm, hash.BytesToHash160(claimRaCtx.Caller.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), primAcc.Balance)

		// Claim negative amount of token will fail
		require.Error(t, p.Claim(claimCtx, sm, big.NewInt(-5)))

		// Claim 0 amount won't fail, but also will not get the token
		require.NoError(t, p.Claim(claimCtx, sm, big.NewInt(0)))

		totalBalance, err = p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(15), totalBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, claimRaCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		primAcc, err = accountutil.LoadAccount(sm, hash.BytesToHash160(claimRaCtx.Caller.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), primAcc.Balance)

		// Claim another 5 token
		require.NoError(t, p.Claim(claimCtx, sm, big.NewInt(5)))

		totalBalance, err = p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), totalBalance)
		unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, claimRaCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		primAcc, err = accountutil.LoadAccount(sm, hash.BytesToHash160(claimRaCtx.Caller.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), primAcc.Balance)

		// Claim the 3-rd 5 token will fail be cause no balance for the address
		require.Error(t, p.Claim(claimCtx, sm, big.NewInt(5)))

		// Operator should have nothing to claim
		claimRaCtx.Caller = raCtx.Producer
		claimCtx = protocol.WithRunActionsCtx(context.Background(), claimRaCtx)
		require.Error(t, p.Claim(claimCtx, sm, big.NewInt(1)))
	}, false)
}

func TestProtocol_NoRewardAddr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := db.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			val, err := cb.Get("state", addrHash[:])
			if err != nil {
				return state.ErrStateNotExist
			}
			return state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			ss, err := state.Serialize(account)
			if err != nil {
				return err
			}
			cb.Put("state", addrHash[:], ss, "failed to put state")
			return nil
		}).AnyTimes()

	p := NewProtocol(func(uint64) (uint64, map[string]uint64, error) {
		return uint64(19),
			map[string]uint64{
				identityset.Address(0).String(): 9,
				identityset.Address(1).String(): 10,
			},
			nil
	}, rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	))

	ge := config.Default.Genesis
	ge.Rewarding.InitBalanceStr = "0"
	ge.Rewarding.BlockRewardStr = "10"
	ge.Rewarding.EpochRewardStr = "100"
	ge.Rewarding.NumDelegatesForEpochReward = 10
	ge.Rewarding.ExemptAddrStrsFromEpochReward = []string{}
	ge.Rewarding.FoundationBonusStr = "5"
	ge.Rewarding.NumDelegatesForFoundationBonus = 5
	ge.Rewarding.FoundationBonusLastEpoch = 365
	ge.Rewarding.ProductivityThreshold = 50

	// Initialize the protocol
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 0,
			Genesis:     ge,
		},
	)
	require.NoError(t, p.CreateGenesisStates(ctx, sm))

	// Create a test account with 1000 token
	_, err := accountutil.LoadOrCreateAccount(sm, identityset.Address(0).String(), big.NewInt(1000))
	require.NoError(t, err)

	ctx = protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    identityset.Address(0),
			Caller:      identityset.Address(0),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
			Candidates: []*state.Candidate{
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
			},
		},
	)
	require.NoError(t, p.Deposit(ctx, sm, big.NewInt(200)))

	// Grant block reward
	rewardLog, err := p.GrantBlockReward(ctx, sm)
	require.NoError(t, err)
	require.Nil(t, rewardLog)

	availableBalance, err := p.AvailableBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(200), availableBalance)
	unclaimedBalance, err := p.UnclaimedBalance(ctx, sm, identityset.Address(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), unclaimedBalance)

	// Grant epoch reward
	rewardLogs, err := p.GrantEpochReward(ctx, sm)
	require.NoError(t, err)
	require.Equal(t, 2, len(rewardLogs))

	availableBalance, err = p.AvailableBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(145), availableBalance)
	unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), unclaimedBalance)
	// It doesn't affect others to get reward
	unclaimedBalance, err = p.UnclaimedBalance(ctx, sm, identityset.Address(1))
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
