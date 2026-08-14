// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
)

func TestProtocol_Fund(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		actionCtx, ok := protocol.GetActionCtx(ctx)
		require.True(t, ok)

		// Deposit 5 token
		rlog, err := p.Deposit(ctx, sm, big.NewInt(5), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		require.NoError(t, err)
		require.Equal(t, 1, len(rlog))
		require.Equal(t, big.NewInt(5).String(), rlog[0].Amount.String())
		require.Equal(t, actionCtx.Caller.String(), rlog[0].Sender)
		require.Equal(t, address.RewardingPoolAddr, rlog[0].Recipient)

		totalBalance, _, err := p.TotalBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), totalBalance)
		availableBalance, _, err := p.AvailableBalance(ctx, sm)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), availableBalance)
		acc, err := accountutil.LoadAccount(sm, actionCtx.Caller)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(995), acc.Balance)

		// Deposit another 6 token will fail because
		_, err = p.Deposit(ctx, sm, big.NewInt(996), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		require.Error(t, err)
	}, nil, false, 0)

}

func TestDepositNegativeGasFee(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		_, err := DepositGas(ctx, sm, big.NewInt(-1))
		require.Error(t, err)
	}, nil, false, 0)
}

func TestFundSerializeDeserialize(t *testing.T) {
	r := require.New(t)
	f := fund{
		totalBalance:     big.NewInt(1000),
		unclaimedBalance: big.NewInt(400),
	}
	data, err := f.Serialize()
	r.NoError(err)

	var f2 fund
	r.NoError(f2.Deserialize(data))
	r.Equal(0, f2.totalBalance.Cmp(f.totalBalance))
	r.Equal(0, f2.unclaimedBalance.Cmp(f.unclaimedBalance))
}

func TestFundDeserializeError(t *testing.T) {
	r := require.New(t)
	var f fund
	r.Error(f.Deserialize([]byte{0xff, 0xff, 0xff, 0xff}))
}

func TestFundEncodeDecode(t *testing.T) {
	r := require.New(t)
	f := fund{
		totalBalance:     big.NewInt(12345),
		unclaimedBalance: big.NewInt(678),
	}
	v, err := f.Encode()
	r.NoError(err)
	r.NotEmpty(v.PrimaryData)
	r.NotEmpty(v.SecondaryData)

	var f2 fund
	r.NoError(f2.Decode(v))
	r.Equal(0, f2.totalBalance.Cmp(f.totalBalance))
	r.Equal(0, f2.unclaimedBalance.Cmp(f.unclaimedBalance))
}

func TestFundDecodeEmptyDefaultsToZero(t *testing.T) {
	r := require.New(t)
	f := fund{
		totalBalance:     big.NewInt(0),
		unclaimedBalance: big.NewInt(0),
	}
	v, err := f.Encode()
	r.NoError(err)

	var f2 fund
	r.NoError(f2.Decode(v))
	r.Equal(0, f2.totalBalance.Sign())
	r.Equal(0, f2.unclaimedBalance.Sign())
}

func TestIsZero(t *testing.T) {
	r := require.New(t)
	r.True(isZero(nil))
	r.True(isZero(big.NewInt(0)))
	r.False(isZero(big.NewInt(1)))
	r.False(isZero(big.NewInt(-1)))
}

func TestDepositZeroAmount(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		logs, err := p.Deposit(ctx, sm, big.NewInt(0), 0)
		require.NoError(t, err)
		require.Nil(t, logs)
	}, nil, false, 0)
}

func TestDepositGasAtGenesisBypassed(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		gctx := protocol.MustGetBlockCtx(ctx)
		gctx.BlockHeight = 0
		ctx0 := protocol.WithBlockCtx(ctx, gctx)
		logs, err := DepositGas(ctx0, sm, big.NewInt(100))
		require.NoError(t, err)
		require.Nil(t, logs)
	}, nil, false, 0)
}

// noUnproductives keeps the invariant fixtures independent of slashing.
var noUnproductives = map[string]uint64{}

func TestFundInvariant_HoldsAfterDeposit(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)
		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)))
	}, noUnproductives, false, 0)
}

func TestSettleCompoundOutflowEmitsTransfer(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		log, err := p.settleCompoundOutflow(big.NewInt(123))
		r.NoError(err)
		r.Equal(iotextypes.TransactionLogType_DEPOSIT_TO_BUCKET, log.Type)
		r.Equal(address.RewardingPoolAddr, log.Sender)
		r.Equal(address.StakingBucketPoolAddr, log.Recipient)
		r.Equal("123", log.Amount.String())

		total, _, err := p.TotalBalance(ctx, sm)
		r.NoError(err)
		r.Equal("1000", total.String())
	}, noUnproductives, false, 0)
}

func TestFundInvariant_HoldsAfterGrantEpochReward_PreFork(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		r.True(protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution)

		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)
		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()
		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)
		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)))
	}, noUnproductives, false, 0)
}

func TestFundInvariant_HoldsAfterGrantEpochReward_PostForkDeferredCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)
		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()
		openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)
		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)
		got, err := p.readVoterRewardDistributionState(ctx, sm)
		r.NoError(err)
		r.Nil(got, "missing profile data defaults to direct owner payout")
		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)))
	}, noUnproductives, false, 0)
}

func TestFundInvariant_DetectsViolation(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)
		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)))

		f := fund{}
		_, err = p.state(ctx, sm, _fundKey, &f)
		r.NoError(err)
		f.totalBalance = new(big.Int).Add(f.totalBalance, big.NewInt(42))
		r.NoError(p.putState(ctx, sm, _fundKey, &f))

		err = p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t))
		r.Error(err)
		r.Contains(err.Error(), "rewarding fund invariant violated")
		r.Contains(err.Error(), "delta=42")
	}, noUnproductives, false, 0)
}
