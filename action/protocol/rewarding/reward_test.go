// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func TestProtocol_GrantBlockReward(t *testing.T) {
	req := require.New(t)
	for _, tv := range []struct {
		blockReward *big.Int
		deposit     *big.Int
		isWakeBlock bool
	}{
		{big.NewInt(10), big.NewInt(200), false},
		{big.NewInt(48), big.NewInt(300), true},
	} {
		testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
			if tv.isWakeBlock {
				g := genesis.MustExtractGenesisContext(ctx)
				g.WakeBlockRewardStr = tv.blockReward.String()
				blkCtx := protocol.MustGetBlockCtx(ctx)
				blkCtx.BlockHeight = g.WakeBlockHeight
				ctx = genesis.WithGenesisContext(protocol.WithBlockCtx(ctx, blkCtx), g)
				req.NoError(p.CreatePreStates(ctx, sm))
			}
			// verify block reward
			br, err := p.BlockReward(ctx, sm)
			req.NoError(err)
			req.Equal(tv.blockReward, br)
			// Grant block reward will fail because of no available balance
			_, err = p.GrantBlockReward(ctx, sm)
			req.Error(err)

			_, err = p.Deposit(ctx, sm, tv.deposit, iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
			req.NoError(err)

			// Grant block reward
			rewardLog, err := p.GrantBlockReward(ctx, sm)
			req.NoError(err)
			req.Equal(p.addr.String(), rewardLog.Address)
			var rl rewardingpb.RewardLog
			req.NoError(proto.Unmarshal(rewardLog.Data, &rl))
			req.Equal(rewardingpb.RewardLog_BLOCK_REWARD, rl.Type)
			req.Equal(tv.blockReward.String(), rl.Amount)

			availableBalance, _, err := p.AvailableBalance(ctx, sm)
			req.NoError(err)
			req.Equal(tv.deposit.Sub(tv.deposit, tv.blockReward), availableBalance)
			// Operator shouldn't get reward
			blkCtx := protocol.MustGetBlockCtx(ctx)
			unclaimedBalance, _, err := p.UnclaimedBalance(ctx, sm, blkCtx.Producer)
			req.NoError(err)
			req.Equal(big.NewInt(0), unclaimedBalance)
			// Beneficiary should get reward
			unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(0))
			req.NoError(err)
			req.Equal(tv.blockReward, unclaimedBalance)

			// Grant the same block reward again will fail
			_, err = p.GrantBlockReward(ctx, sm)
			req.Error(err)

			// Grant with priority fee after VanuatuBlockHeight
			blkCtx.AccumulatedTips = *big.NewInt(5)
			blkCtx.BlockHeight = genesis.TestDefault().VanuatuBlockHeight
			ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, blkCtx))
			tLog, err := DepositGas(ctx, sm, nil, protocol.PriorityFeeOption(&blkCtx.AccumulatedTips))
			req.NoError(err)
			req.Equal(tLog[0].Type, iotextypes.TransactionLogType_PRIORITY_FEE)
			req.Equal(&blkCtx.AccumulatedTips, tLog[0].Amount)
			rewardLog, err = p.GrantBlockReward(ctx, sm)
			req.NoError(err)
			rls, err := UnmarshalRewardLog(rewardLog.Data)
			req.NoError(err)
			req.Len(rls.Logs, 2)
			req.Equal(rewardingpb.RewardLog_BLOCK_REWARD, rls.Logs[0].Type)
			req.Equal(tv.blockReward.String(), rls.Logs[0].Amount)
			req.Equal(rewardingpb.RewardLog_PRIORITY_BONUS, rls.Logs[1].Type)
			req.Equal(blkCtx.AccumulatedTips.String(), rls.Logs[1].Amount)

			// check available and receiver balance
			availableBalance, _, err = p.AvailableBalance(ctx, sm)
			req.NoError(err)
			req.Equal(tv.deposit.Sub(tv.deposit, tv.blockReward), availableBalance)
			unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(0))
			req.NoError(err)
			tv.blockReward.Lsh(tv.blockReward, 1)
			req.Equal(tv.blockReward.Add(tv.blockReward, &blkCtx.AccumulatedTips), unclaimedBalance)
		}, false)
	}
}

func TestProtocol_GrantEpochReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		blkCtx, ok := protocol.GetBlockCtx(ctx)
		require.True(t, ok)

		_, err := p.Deposit(ctx, sm, big.NewInt(200), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		require.NoError(t, err)

		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		// Grant epoch reward
		rewardLogs, err := p.GrantEpochReward(ctx, sm)
		require.NoError(t, err)
		require.Equal(t, 8, len(rewardLogs))

		availableBalance, _, err := p.AvailableBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(90+5), availableBalance)
		// Operator shouldn't get reward
		unclaimedBalance, _, err := p.UnclaimedBalance(ctx, sm, identityset.Address(27))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		// Beneficiary should get reward
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(0))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(40+5), unclaimedBalance)
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(28))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(30+5), unclaimedBalance)
		// The 3-th candidate can't get the reward because it doesn't meet the productivity requirement
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(29))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(30))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10+5), unclaimedBalance)
		// The 5-th candidate can't get the reward because of being out of the range
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(31))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		// The 6-th candidate can't get the foundation bonus because of being out of the range
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(32))
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
		blkCtx.BlockHeight++
		ctx = protocol.WithBlockCtx(ctx, blkCtx)
		_, err = p.GrantEpochReward(ctx, sm)
		require.Error(t, err)
	}, false)

	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		_, err := p.Deposit(ctx, sm, big.NewInt(200), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		require.NoError(t, err)

		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		// Grant epoch reward
		_, err = p.GrantEpochReward(ctx, sm)
		require.NoError(t, err)

		// The 5-th candidate can't get the reward because exempting from the epoch reward
		unclaimedBalance, _, err := p.UnclaimedBalance(ctx, sm, identityset.Address(31))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		// The 6-th candidate can get the foundation bonus because it's still within the range after excluding 5-th one
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(32))
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(4+5), unclaimedBalance)
	}, true)
}

func TestProtocol_ClaimReward(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		// Deposit 20 token into the rewarding fund
		_, err := p.Deposit(ctx, sm, big.NewInt(20), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		require.NoError(t, err)

		// Grant block reward
		rewardLog, err := p.GrantBlockReward(ctx, sm)
		require.NoError(t, err)
		require.Equal(t, p.addr.String(), rewardLog.Address)
		var rl rewardingpb.RewardLog
		require.NoError(t, proto.Unmarshal(rewardLog.Data, &rl))
		require.Equal(t, rewardingpb.RewardLog_BLOCK_REWARD, rl.Type)
		require.Equal(t, "10", rl.Amount)

		// Claim 5 token
		actionCtx, ok := protocol.GetActionCtx(ctx)
		require.True(t, ok)
		claimActionCtx := actionCtx
		claimActionCtx.Caller = identityset.Address(0)
		claimCtx := protocol.WithActionCtx(ctx, claimActionCtx)

		// Record the init balance of account
		primAcc, err := accountutil.LoadAccount(sm, claimActionCtx.Caller)
		require.NoError(t, err)
		initBalance := primAcc.Balance

		_, err = p.Claim(claimCtx, sm, big.NewInt(5), claimActionCtx.Caller)
		require.NoError(t, err)

		totalBalance, _, err := p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(15), totalBalance)
		unclaimedBalance, _, err := p.UnclaimedBalance(ctx, sm, claimActionCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		primAcc, err = accountutil.LoadAccount(sm, claimActionCtx.Caller)
		require.NoError(t, err)
		initBalance = new(big.Int).Add(initBalance, big.NewInt(5))
		assert.Equal(t, initBalance, primAcc.Balance)

		// Claim negative amount of token will fail
		_, err = p.Claim(claimCtx, sm, big.NewInt(-5), claimActionCtx.Caller)
		require.Error(t, err)

		// Claim 0 amount won't fail, but also will not get the token
		_, err = p.Claim(claimCtx, sm, big.NewInt(0), claimActionCtx.Caller)
		require.NoError(t, err)

		totalBalance, _, err = p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(15), totalBalance)
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, claimActionCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), unclaimedBalance)
		primAcc, err = accountutil.LoadAccount(sm, claimActionCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, initBalance, primAcc.Balance)

		// Claim another 5 token
		rlog, err := p.Claim(claimCtx, sm, big.NewInt(5), claimActionCtx.Caller)
		require.NoError(t, err)
		require.NoError(t, err)
		require.NotNil(t, rlog)
		require.Equal(t, big.NewInt(5).String(), rlog.Amount.String())
		require.Equal(t, address.RewardingPoolAddr, rlog.Sender)
		require.Equal(t, claimActionCtx.Caller.String(), rlog.Recipient)

		totalBalance, _, err = p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(10), totalBalance)
		unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, claimActionCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), unclaimedBalance)
		primAcc, err = accountutil.LoadAccount(sm, claimActionCtx.Caller)
		require.NoError(t, err)
		initBalance = new(big.Int).Add(initBalance, big.NewInt(5))
		assert.Equal(t, initBalance, primAcc.Balance)

		// Claim the 3-rd 5 token will fail be cause no balance for the address
		_, err = p.Claim(claimCtx, sm, big.NewInt(5), claimActionCtx.Caller)
		require.Error(t, err)

		// Operator should have nothing to claim
		blkCtx, ok := protocol.GetBlockCtx(ctx)
		require.True(t, ok)
		claimActionCtx.Caller = blkCtx.Producer
		claimCtx = protocol.WithActionCtx(ctx, claimActionCtx)
		_, err = p.Claim(claimCtx, sm, big.NewInt(1), claimActionCtx.Caller)
		require.Error(t, err)
	}, false)
}

func TestProtocol_NoRewardAddr(t *testing.T) {
	ctrl := gomock.NewController(t)

	registry := protocol.NewRegistry()
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := batch.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			val, err := cb.Get("state", cfg.Key)
			if err != nil {
				return 0, state.ErrStateNotExist
			}
			return 0, state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ss, err := state.Serialize(account)
			if err != nil {
				return 0, err
			}
			cb.Put("state", cfg.Key, ss, "failed to put state")
			return 0, nil
		}).AnyTimes()
	sm.EXPECT().Height().Return(uint64(1), nil).AnyTimes()

	ge := genesis.TestDefault()
	ge.Rewarding.InitBalanceStr = "0"
	ge.Rewarding.BlockRewardStr = "10"
	ge.Rewarding.EpochRewardStr = "100"
	ge.Rewarding.NumDelegatesForEpochReward = 10
	ge.Rewarding.ExemptAddrStrsFromEpochReward = []string{}
	ge.Rewarding.FoundationBonusStr = "5"
	ge.Rewarding.NumDelegatesForFoundationBonus = 5
	ge.Rewarding.FoundationBonusLastEpoch = 365
	ge.Rewarding.ProductivityThreshold = 50
	ge.Rewarding.FoundationBonusP2StartEpoch = 365
	ge.Rewarding.FoundationBonusP2EndEpoch = 365

	// Create a test account with 1000 token
	ge.InitBalanceMap[identityset.Address(0).String()] = "1000"

	p := NewProtocol(ge.Rewarding)
	rp := rolldpos.NewProtocol(
		ge.NumCandidateDelegates,
		ge.NumDelegates,
		ge.NumSubEpochs,
	)
	abps := []*state.Candidate{
		{
			Address:       identityset.Address(0).String(),
			Votes:         unit.ConvertIotxToRau(1000000),
			RewardAddress: identityset.Address(0).String(),
		},
		{
			Address:       identityset.Address(1).String(),
			Votes:         unit.ConvertIotxToRau(1000000),
			RewardAddress: identityset.Address(1).String(),
		},
	}
	g := genesis.TestDefault()
	committee := mock_committee.NewMockCommittee(ctrl)
	slasher, err := poll.NewSlasher(
		func(uint64, uint64) (map[string]uint64, error) {
			return map[string]uint64{
				identityset.Address(0).String(): 9,
				identityset.Address(1).String(): 10,
			}, nil
		},
		func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
			return abps, 0, nil
		},
		nil,
		nil,
		nil,
		2,
		2,
		g.ProductivityThreshold,
		g.ProbationEpochPeriod,
		g.UnproductiveDelegateMaxCacheSize,
		g.ProbationIntensityRate)
	require.NoError(t, err)
	pp, err := poll.NewGovernanceChainCommitteeProtocol(
		nil,
		committee,
		uint64(123456),
		func(uint64) (time.Time, error) { return time.Now(), nil },
		blockchain.DefaultConfig.PollInitialCandidatesInterval,
		slasher,
	)
	require.NoError(t, err)
	require.NoError(t, rp.Register(registry))
	require.NoError(t, pp.Register(registry))
	require.NoError(t, p.Register(registry))

	// Initialize the protocol
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(
			protocol.WithRegistry(context.Background(), registry),
			ge,
		),
		protocol.BlockCtx{
			BlockHeight: 0,
		},
	)
	ctx = protocol.WithFeatureCtx(ctx)
	ap := account.NewProtocol(DepositGas)
	require.NoError(t, ap.CreateGenesisStates(ctx, sm))
	require.NoError(t, p.CreateGenesisStates(ctx, sm))
	ctx = protocol.WithBlockchainCtx(
		protocol.WithRegistry(ctx, registry),
		protocol.BlockchainCtx{
			Tip: protocol.TipInfo{
				Height: 1,
			},
		},
	)

	require.NoError(t, p.CreateGenesisStates(ctx, sm))

	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			Producer:    identityset.Address(0),
			BlockHeight: g.NumDelegates * g.NumSubEpochs,
		},
	)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller: identityset.Address(0),
		},
	)
	_, err = p.Deposit(ctx, sm, big.NewInt(200), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
	require.NoError(t, err)

	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	// Grant block reward
	_, err = p.GrantBlockReward(ctx, sm)
	require.NoError(t, err)

	availableBalance, _, err := p.AvailableBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(190), availableBalance)
	unclaimedBalance, _, err := p.UnclaimedBalance(ctx, sm, identityset.Address(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(10), unclaimedBalance)

	// Grant epoch reward
	ctx = protocol.WithFeatureCtx(ctx)
	rewardLogs, err := p.GrantEpochReward(ctx, sm)
	require.NoError(t, err)
	require.Equal(t, 4, len(rewardLogs))

	availableBalance, _, err = p.AvailableBalance(ctx, sm)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(80), availableBalance)
	unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(65), unclaimedBalance)
	// It doesn't affect others to get reward
	unclaimedBalance, _, err = p.UnclaimedBalance(ctx, sm, identityset.Address(1))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(55), unclaimedBalance)
	require.Equal(t, p.addr.String(), rewardLogs[0].Address)
	var rl rewardingpb.RewardLog
	require.NoError(t, proto.Unmarshal(rewardLogs[0].Data, &rl))
	assert.Equal(t, rewardingpb.RewardLog_EPOCH_REWARD, rl.Type)
	assert.Equal(t, identityset.Address(0).String(), rl.Addr)
	assert.Equal(t, "50", rl.Amount)
	require.Equal(t, p.addr.String(), rewardLogs[1].Address)
	require.NoError(t, proto.Unmarshal(rewardLogs[1].Data, &rl))
	assert.Equal(t, rewardingpb.RewardLog_EPOCH_REWARD, rl.Type)
	assert.Equal(t, identityset.Address(1).String(), rl.Addr)
	assert.Equal(t, "50", rl.Amount)
}

func TestRewardLogCompatibility(t *testing.T) {
	r := require.New(t)
	rl := &rewardingpb.RewardLog{
		Type:   rewardingpb.RewardLog_BLOCK_REWARD,
		Addr:   "io1",
		Amount: "100",
	}
	data, err := proto.Marshal(rl)
	r.NoError(err)
	rls := &rewardingpb.RewardLogs{
		Logs: []*rewardingpb.RewardLog{rl},
	}
	datas, err := proto.Marshal(rls)
	r.NoError(err)
	t.Logf("rls = %+v", rls)

	rls2, err := UnmarshalRewardLog(data)
	r.NoError(err)
	t.Logf("decoded from rl = %+v", rls2)
	datao, err := proto.Marshal(rls2)
	r.NoError(err)
	r.Equal(datas, datao)

	rls2, err = UnmarshalRewardLog(datas)
	r.NoError(err)
	t.Logf("decoded from rls = %+v", rls2)
	datao, err = proto.Marshal(rls2)
	r.NoError(err)
	r.Equal(datas, datao)
}

func TestProtocol_CalculateReward(t *testing.T) {
	req := require.New(t)
	var (
		dardanellesBlockReward = unit.ConvertIotxToRau(8)
		wakeBlockReward, _     = big.NewInt(0).SetString("4800000000000000000", 10)
	)
	for _, tv := range []struct {
		accumuTips                    *big.Int
		isWakeBlock                   bool
		blockReward, totalReward, tip *big.Int
	}{
		{unit.ConvertIotxToRau(3), false, dardanellesBlockReward, unit.ConvertIotxToRau(11), unit.ConvertIotxToRau(3)},
		{unit.ConvertIotxToRau(12), false, dardanellesBlockReward, unit.ConvertIotxToRau(20), unit.ConvertIotxToRau(12)},
		{unit.ConvertIotxToRau(3), true, (&big.Int{}).Sub(wakeBlockReward, unit.ConvertIotxToRau(3)), wakeBlockReward, unit.ConvertIotxToRau(3)},
		{unit.ConvertIotxToRau(6), true, (&big.Int{}).SetInt64(0), unit.ConvertIotxToRau(6), unit.ConvertIotxToRau(6)},
	} {
		testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
			// update block reward
			g := genesis.MustExtractGenesisContext(ctx)
			blkCtx := protocol.MustGetBlockCtx(ctx)
			blkCtx.AccumulatedTips.Set(tv.accumuTips)
			if tv.isWakeBlock {
				g.WakeBlockRewardStr = wakeBlockReward.String()
				blkCtx.BlockHeight = g.WakeBlockHeight
				ctx = protocol.WithFeatureCtx(genesis.WithGenesisContext(protocol.WithBlockCtx(ctx, blkCtx), g))
				req.NoError(p.CreatePreStates(ctx, sm))
			} else {
				g.DardanellesBlockRewardStr = dardanellesBlockReward.String()
				blkCtx.BlockHeight = g.DardanellesBlockHeight
				ctx = genesis.WithGenesisContext(protocol.WithBlockCtx(ctx, blkCtx), g)
				req.NoError(p.CreatePreStates(ctx, sm))
				blkCtx.BlockHeight = g.VanuatuBlockHeight
				ctx = protocol.WithFeatureCtx(genesis.WithGenesisContext(protocol.WithBlockCtx(ctx, blkCtx), g))
			}
			// verify block reward, total reward, and tip
			total, br, tip, err := p.calculateTotalRewardAndTip(ctx, sm)
			req.NoError(err)
			req.Zero(tv.blockReward.Cmp(br))
			req.Zero(tv.totalReward.Cmp(total))
			req.Zero(tv.tip.Cmp(tip))
			req.Zero(total.Cmp(br.Add(br, tip)))
		}, false)
	}
}
