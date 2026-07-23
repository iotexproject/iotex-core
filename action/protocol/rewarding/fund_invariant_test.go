// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// noUnproductives is passed to testProtocol so slashUqd finds no candidates
// to slash. The invariant tests rely on this because the fixture patches
// staking.SlashCandidateByID to a no-op — the return value is not "no slash
// happened," it's "the slash pretended to succeed but transferred no value."
// A non-empty uqd map would leave GrantEpochReward crediting a phantom
// slashAmount into unclaimedBalance while totalBalance stays put, which
// mimics a real fund state divergence and would drown out real invariant
// violations we are trying to catch.
var noUnproductives = map[string]uint64{}

// TestFundInvariant_HoldsAfterDeposit is the base case: right after a
// Deposit the fund's total and unclaimed balances are equal, and both
// perAddress and pool are empty. The invariant helper must accept this
// state as valid.
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

// TestFundInvariant_HoldsAfterGrantEpochReward_PreFork walks the pre-fork
// legacy epoch grant path: splitDelegateEpochReward returns (amount, 0) for
// every delegate, foundation bonus fires, no cursor is written. Every
// grant is offset by a matching updateAvailableBalance debit, so the
// invariant must hold at the end.
func TestFundInvariant_HoldsAfterGrantEpochReward_PreFork(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		// testProtocol leaves the fork gate closed by default — no cursor
		// path, legacy per-delegate grants only.
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		r.True(protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution)

		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)),
			"pre-fork Phase A must preserve total = unclaimed + Σ(perAddress) + Σ(pool)")
	}, noUnproductives, false, 0)
}

// TestFundInvariant_HoldsAfterGrantEpochReward_PostForkDeferredCursor walks the
// post-fork Phase A path in the absence of any block-side voter accrual:
// no poll snapshot is present, so the full epoch share enters pending pools
// and a deferred cursor is created. The invariant must still hold.
func TestFundInvariant_HoldsAfterGrantEpochReward_PostForkDeferredCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(got, "post-fork voter shares must remain pending without a snapshot")

		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)))
	}, noUnproductives, false, 0)
}

// TestFundInvariant_DetectsViolation is a sanity check on the helper
// itself: an out-of-band mutation to the fund's total balance without
// a matching change to the per-address / pool ledger must be caught,
// and the error message must include the delta.
//
// The full-drain-span invariant assertion lives in the e2e stress test
// (iip59_stress_test.go), which runs a real staking factory so
// distributeVoterOnly's grantToAccount side effects actually fire and
// offset the pool decrement. The unit scaffold's mock view does not
// support ConstructBaseView, so unit tests are limited to Phase A.
func TestFundInvariant_DetectsViolation(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)
		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)))

		// Break the invariant by inflating totalBalance without touching
		// unclaimed / perAddress / pool. The helper must catch this.
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
