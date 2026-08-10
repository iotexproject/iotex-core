// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestVoterBudgetPerBlock confirms the fork gate short-circuits the
// voter-count cap. Pre-fork, VoterBudgetPerBlock is force-zeroed so
// distributeVoterOnly never runs a window; the payout stays unbounded
// (matches genesis-parity). Post-fork the configured value is passed
// through.
func TestVoterBudgetPerBlock(t *testing.T) {
	r := require.New(t)

	forkOff, _, pForkOff, _, _ := newVoterRewardCtx(t, false)
	pForkOff.cfg.VoterBudgetPerBlock = 2000
	r.Equal(uint32(0), pForkOff.voterBudgetPerBlock(forkOff),
		"pre-fork: voter budget must be 0 regardless of VoterBudgetPerBlock")

	forkOn, _, pForkOn, _, _ := newVoterRewardCtx(t, true)
	pForkOn.cfg.VoterBudgetPerBlock = 2000
	r.Equal(uint32(2000), pForkOn.voterBudgetPerBlock(forkOn),
		"post-fork with budget=2000: voter budget must be 2000")

	pForkOn.cfg.VoterBudgetPerBlock = 0
	r.Equal(uint32(0), pForkOn.voterBudgetPerBlock(forkOn),
		"post-fork with budget=0: voter budget must be 0 (unbounded voters per delegate)")
}

// TestGrantEpochReward_DefaultsToOwnerWhenProfileUnregistered confirms that a
// migrated delegate that opted in but never registered a DelegateProfile
// receives the full reward directly.
//
// The fixture writes the snapshot FreezePollSnapshot produces for exactly that
// candidate (poll_snapshot.go): opted in, Registered false, and both commission
// rates defaulted to the full 10000 bps because the profile view returned
// nothing to override them with. That default is the design — no registered
// rate means no voter split — so the delegate stays on IIP-59 rails and
// creditRewardDirect pays the owner, which is what the transaction log below
// counts.
//
// Writing the snapshot is load-bearing. Omitting it does not reach this design
// at all: it reaches the no-snapshot-for-the-era branch of
// resolveDelegateRewardRouting, which is a different rule (not on rails this
// era, pay legacy) covered by voter_snapshot_missing_test.go. The two used to
// coincide, so this test passed without a snapshot while asserting something
// it did not set up.
func TestGrantEpochReward_DefaultsToOwnerWhenProfileUnregistered(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		for i := 27; i <= 31; i++ {
			r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, identityset.Address(i), &staking.CandidatePollSnapshot{
				OnchainRewardEnabled:       true,
				Registered:                 false,
				BlockCommissionBasisPoints: _basisPointsDenom,
				EpochCommissionBasisPoints: _basisPointsDenom,
				TotalWeight:                new(big.Int),
				FreezeHeight:               iip59FixtureFreezeHeight,
			}))
		}
		openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		transactionLogs, _, err := p.GrantEpochReward(ctx, sm)
		r.NoError(err)
		r.NotEmpty(transactionLogs)
		directPayouts := 0
		for _, transactionLog := range transactionLogs {
			if transactionLog.Type == iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND {
				directPayouts++
			}
		}
		r.Positive(directPayouts)

		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(got)

		r.Error(p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1),
			"sentinel must be written by GrantEpochReward even when the cursor is empty")
	}, nil, false, 0)
}

// TestGrantEpochReward_NonEraAccruesVoterShareWithoutCursor verifies the
// distinction between per-epoch accounting and per-era distribution. An
// delegate's voter share must enter the pending pool every epoch,
// while cursor materialization remains restricted to era boundaries.
func TestGrantEpochReward_NonEraAccruesVoterShareWithoutCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		g := genesis.MustExtractGenesisContext(ctx)
		g.Rewarding.EpochsPerRewardEra = 2
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		candAddr := identityset.Address(27)
		candID := candAddr.Bytes()
		writeSnapshot(t, sm, candAddr, true, 2500, []voterEntry{
			{addr: identityset.Address(1), weight: big.NewInt(1)},
		})

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		// Candidate 27 receives 40% of the 100 rau epoch reward. With a
		// 25% commission, 10 goes to the delegate and 30 to the voter pool.
		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		pool, err := p.readPendingBlockRewardPool(ctx, sm, candID)
		r.NoError(err)
		r.Equal(int64(30), pool.Int64(), "non-era epoch voter share must accrue in the pool")
		cursor, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(cursor, "non-era epoch must not materialize a drain cursor")
		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)))

		// Epoch 2 is the era boundary. Its 30 rau voter share joins the
		// prior epoch's 30 rau, and the cursor freezes the accumulated 60.
		rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
		blk := protocol.MustGetBlockCtx(ctx)
		blk.BlockHeight = rp.GetEpochLastBlockHeight(2)
		ctx = protocol.WithBlockCtx(ctx, blk)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		pool, err = p.readPendingBlockRewardPool(ctx, sm, candID)
		r.NoError(err)
		r.Equal(int64(60), pool.Int64())
		cursor, err = p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(cursor, "era boundary must materialize a drain cursor")
		var frozen *epochDrainDelegateWork
		for _, work := range cursor.Delegates {
			if string(work.CandidateIdentifier) == string(candID) {
				frozen = &work
				break
			}
		}
		r.NotNil(frozen, "candidate 27 must be included in the era cursor")
		r.Equal(int64(60), frozen.VoterAmountFrozen.Int64())
		r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, allProtocolAddrs(t)))
	}, noUnproductives, false, 0)
}

// TestGrantEpochReward_LiveCursorAtPhaseA_DegradesGracefully confirms the
// IIP-59 §10.2 degrade path: when a previous era's cursor survives into
// the next era's Phase A entry, GrantEpochReward does NOT halt block
// production. Instead it emits an EPOCH_DRAIN_OVERRUN receipt log
// describing the residue, deletes the stale cursor, and continues the
// Phase A grant. The stale pool balances themselves stay in place —
// Phase A's own materialisation later in the same call picks them up
// as work items for the fresh era.
func TestGrantEpochReward_LiveCursorAtPhaseA_DegradesGracefully(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		// enableIIP59 sets ToBeEnabledBlockHeight=1 AND forces
		// EpochsPerRewardEra=1 so the block-height fixture (epoch 1's
		// last block) counts as an era boundary; the overrun handler
		// only fires on era boundaries.
		ctx = enableIIP59(t, ctx)

		live := &epochDrainCursor{
			TargetEra: 1,
			Delegates: []epochDrainDelegateWork{
				{CandidateIdentifier: identityset.Address(27).Bytes(), VoterAmountFrozen: big.NewInt(1)},
			},
		}
		r.NoError(p.writeEpochDrainCursor(ctx, sm, live))

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)
		openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)

		_, rewardLogs, err := p.GrantEpochReward(ctx, sm)
		r.NoError(err)
		// The stale cursor is deleted. With no profile snapshot, the new epoch
		// defaults to full owner payout and creates no replacement cursor.
		after, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(after)
		// First log must be the EPOCH_DRAIN_OVERRUN handoff naming the
		// stale era + remaining-delegates count. Actual residue value is
		// tested end-to-end in the PR6 test suite; here we assert only
		// the log type + addr encoding shape.
		r.NotEmpty(rewardLogs)
		overrun := &rewardingpb.RewardLog{}
		r.NoError(proto.Unmarshal(rewardLogs[0].Data, overrun))
		r.Equal(rewardingpb.RewardLog_EPOCH_DRAIN_OVERRUN, overrun.Type)
		// "<target_era>:<delegates_still_holding_a_pool>". The stale cursor's
		// delegate has no pool balance seeded, so the count is 0.
		r.Equal("1:0", overrun.Addr)
	}, nil, false, 0)
}

// TestGrantEpochReward_FeatureOffIgnoresCursor confirms the fork-off
// invariant: with NoVoterRewardDistribution=true, GrantEpochReward
// runs the legacy single-block loop and NEVER reads/writes/deletes
// the cursor. A cursor persisted before the fork opened must survive
// a legacy epoch grant untouched.
func TestGrantEpochReward_FeatureOffIgnoresCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		// testProtocol leaves ToBeEnabledBlockHeight at the default
		// (math.MaxUint64), so fork is off. Sanity-check.
		r.True(protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution,
			"testProtocol default must have fork off")

		// Seed a plausible-looking cursor for the CURRENT epoch so that,
		// if the fork gate were mistakenly bypassed, GrantEpochReward
		// would take the continuation branch and skip Phase A. A survivor
		// cursor after grant proves cursorEnabled=false held.
		injected := &epochDrainCursor{
			TargetEra: 1,
			Delegates: []epochDrainDelegateWork{
				{CandidateIdentifier: identityset.Address(27).Bytes(), VoterAmountFrozen: big.NewInt(42)},
			},
		}
		r.NoError(p.writeEpochDrainCursor(ctx, sm, injected))

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		// The cursor must still be present, unmodified. If the fork-off
		// path had read it, we'd have taken the continuation branch and
		// either skipped Phase A's assertNoRewardYet (invalid) or finalized
		// the cursor (equally invalid).
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(got, "cursor must survive a legacy epoch grant")
		r.Equal(injected.TargetEra, got.TargetEra)
		r.Equal(injected.ShardsDone, got.ShardsDone)
		r.Len(got.Delegates, 1)
		r.Equal(injected.Delegates[0].CandidateIdentifier, got.Delegates[0].CandidateIdentifier)
		r.Equal(int64(42), got.Delegates[0].VoterAmountFrozen.Int64())
	}, nil, false, 0)
}

// TestGrantEpochReward_PoolAccrualBuildsCursor confirms that block-time
// voter accruals — pool balance credited by GrantBlockReward — get folded
// into the epoch-boundary cursor even
// when the per-delegate epoch split has no fresh voter share (fallback
// branch of splitDelegateEpochReward). This is what preserves late-
// arriving voter accruals across the era boundary: the cursor freezes
// pool + epochShare, and Phase B's decrement removes exactly that
// frozen amount from the pool.
func TestGrantEpochReward_PoolAccrualBuildsCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Seed a pool balance for candidate 27 (the first candidate the
		// default poll list uses). No frozen snapshot exists, so the
		// epoch-side split returns (amount, 0) — the cursor entry comes
		// purely from the pool accrual.
		candID := identityset.Address(27).Bytes()
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(1_234)))
		r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, identityset.Address(27), &staking.CandidatePollSnapshot{
			OnchainRewardEnabled:       true,
			BlockCommissionBasisPoints: _basisPointsDenom,
			EpochCommissionBasisPoints: _basisPointsDenom,
			Registered:                 true,
			TotalWeight:                big.NewInt(1),
			FreezeHeight:               iip59FixtureFreezeHeight,
		}))
		openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(got, "pool accrual must build a cursor entry")
		r.Equal(uint64(1), got.TargetEra)
		r.Equal(uint16(0), got.ShardsDone)

		var found bool
		for _, work := range got.Delegates {
			if string(work.CandidateIdentifier) == string(candID) {
				found = true
				r.Equal(int64(1_234), work.VoterAmountFrozen.Int64(),
					"cursor must freeze the pool accrual as-is when epoch split yields no voter share")
			}
		}
		r.True(found, "candidate 27 must appear in the cursor")

		// Sentinel is written by GrantEpochReward regardless of cursor state.
		r.Error(p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1),
			"sentinel must be written in the same call that builds the cursor")
	}, nil, false, 0)
}
