// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// The reward snapshot lives under one per-candidate key with no era qualifier.
// A candidate frozen in era N and skipped at era N+1's freeze therefore still
// reads back, carrying era N's FreezeHeight H'. Handed to the drain unchecked,
// H' becomes the evaluation height for weights whose buckets come from era
// N+1's copy-on-write window (opened at H): the numerator is bucket membership
// at H, the denominator is the candidate's Votes at H'. Nothing errors — the
// era just settles on a mixed basis.
//
// freezeDelegateDrainWork treats such a snapshot as absent, which is the path
// the freezer already produces for a candidate it never froze.

// TestFreezePendingPoolDrainWork_StaleSnapshotDefersPool is the guard.
func TestFreezePendingPoolDrainWork_StaleSnapshotDefersPool(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)

	const currentEraH = uint64(20_000)
	const previousEraH = uint64(10_000)
	openEraWindowForTest(t, ctx, sm, currentEraH)

	r.NoError(staking.TestOnlyPutCandidateRewardSnapshotFor(sm, candAddr, &staking.CandidateRewardSnapshot{
		EpochCommissionBasisPoints: 3_000,
		BlockCommissionBasisPoints: 3_000,
		TotalWeight:                big.NewInt(1_000_000),
		FreezeHeight:               previousEraH,
		SelfStakeBucketIdx:         7,
	}))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candAddr.Bytes(), big.NewInt(5_000)))

	work, err := p.freezePendingPoolDrainWork(ctx, sm, currentEraH)
	r.NoError(err, "a stale snapshot is chain state, so it must degrade rather than halt the block")
	r.Empty(work, "a stale denominator must not enter this era's plan")

	pool, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
	r.NoError(err)
	r.Zero(big.NewInt(5_000).Cmp(pool),
		"a stale snapshot must leave the pool intact for a later era")
}

// TestFreezePendingPoolDrainWork_CurrentEraSnapshotEntersPlan is the regression
// lock on the other side. The guard must key on the era, not on the mere
// presence of a FreezeHeight — an over-broad version would defer every
// delegate, forever, and IIP-59 would pay no voters at all.
func TestFreezePendingPoolDrainWork_CurrentEraSnapshotEntersPlan(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)

	const currentEraH = uint64(20_000)
	openEraWindowForTest(t, ctx, sm, currentEraH)

	r.NoError(staking.TestOnlyPutCandidateRewardSnapshotFor(sm, candAddr, &staking.CandidateRewardSnapshot{
		EpochCommissionBasisPoints: 3_000,
		BlockCommissionBasisPoints: 3_000,
		TotalWeight:                big.NewInt(1_000_000),
		FreezeHeight:               currentEraH,
		SelfStakeBucketIdx:         7,
	}))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candAddr.Bytes(), big.NewInt(5_000)))

	work, err := p.freezePendingPoolDrainWork(ctx, sm, currentEraH)
	r.NoError(err)
	r.Len(work, 1)
	r.Equal(candAddr.Bytes(), work[0].CandidateIdentifier)
	r.Zero(big.NewInt(5_000).Cmp(work[0].VoterAmountFrozen))
	r.Zero(big.NewInt(1_000_000).Cmp(work[0].TotalWeight))
	r.Equal(uint64(7), work[0].SelfStakeBucketIdx)

	pool, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
	r.NoError(err)
	r.Zero(big.NewInt(5_000).Cmp(pool), "plan construction must not consume a payable pool")
}

// TestFreezePendingPoolDrainWork_ExitedCandidateStillEntersPlan proves plan
// construction is driven by pending pools, not the current poll or candidate
// set. A candidate may leave after the era freeze while voter money remains;
// its fresh snapshot is sufficient to settle that frozen era.
func TestFreezePendingPoolDrainWork_ExitedCandidateStillEntersPlan(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	const currentEraH = uint64(20_000)
	exited := identityset.Address(20)
	_, _, err := staking.NewCandidateByAddressReader(sm).CandidateByAddress(exited)
	r.ErrorIs(err, state.ErrStateNotExist,
		"fixture candidate must be absent from current candidate state")

	openEraWindowForTest(t, ctx, sm, currentEraH)
	r.NoError(staking.TestOnlyPutCandidateRewardSnapshotFor(sm, exited, &staking.CandidateRewardSnapshot{
		TotalWeight:        big.NewInt(900),
		FreezeHeight:       currentEraH,
		SelfStakeBucketIdx: staking.NoSelfStakeBucketIndex,
	}))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, exited.Bytes(), big.NewInt(321)))

	work, err := p.freezePendingPoolDrainWork(ctx, sm, currentEraH)
	r.NoError(err)
	r.Len(work, 1)
	r.Equal(exited.Bytes(), work[0].CandidateIdentifier)
	r.Zero(big.NewInt(321).Cmp(work[0].VoterAmountFrozen))
	r.Zero(big.NewInt(900).Cmp(work[0].TotalWeight))

	pool, err := p.readPendingBlockRewardPool(ctx, sm, exited.Bytes())
	r.NoError(err)
	r.Zero(big.NewInt(321).Cmp(pool), "plan construction must not consume the exited candidate's pool")
}

// TestFreezePendingPoolDrainWork_MissingSnapshotDefersPool pins the pre-existing
// "no snapshot -> defer" path the guard degrades into.
func TestFreezePendingPoolDrainWork_MissingSnapshotDefersPool(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)
	openEraWindowForTest(t, ctx, sm, 20_000)
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candAddr.Bytes(), big.NewInt(5_000)))

	work, err := p.freezePendingPoolDrainWork(ctx, sm, 20_000)
	r.NoError(err)
	r.Empty(work)

	pool, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
	r.NoError(err)
	r.Zero(big.NewInt(5_000).Cmp(pool), "a missing snapshot must leave the pool intact")
}

// TestFreezePendingPoolDrainWork_ZeroWeightSnapshotDefersPool ensures a fresh
// snapshot with no payable voter denominator behaves like a missing snapshot.
func TestFreezePendingPoolDrainWork_ZeroWeightSnapshotDefersPool(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)

	const currentEraH = uint64(20_000)
	openEraWindowForTest(t, ctx, sm, currentEraH)
	r.NoError(staking.TestOnlyPutCandidateRewardSnapshotFor(sm, candAddr, &staking.CandidateRewardSnapshot{
		TotalWeight:        new(big.Int),
		FreezeHeight:       currentEraH,
		SelfStakeBucketIdx: 7,
	}))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candAddr.Bytes(), big.NewInt(5_000)))

	work, err := p.freezePendingPoolDrainWork(ctx, sm, currentEraH)
	r.NoError(err)
	r.Empty(work, "a zero-weight snapshot must not enter the plan")

	pool, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
	r.NoError(err)
	r.Zero(big.NewInt(5_000).Cmp(pool), "a zero-weight snapshot must leave the pool intact")
}

// TestDistributeEpochCommissions_StaleSnapshotDefersPool walks the real caller,
// which is what proves the era height actually travels from the copy-on-write
// window into the guard. A unit test on freezeDelegateDrainWork alone would
// still pass if distributeEpochCommissions handed it a zero.
func TestDistributeEpochCommissions_StaleSnapshotDefersPool(t *testing.T) {
	for _, tc := range []struct {
		name            string
		snapshotFreezeH uint64
		wantPlanned     bool
	}{
		{"stale snapshot defers", 10_000, false},
		{"current-era snapshot settles", 20_000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)

			const currentEraH = uint64(20_000)
			openEraWindowForTest(t, ctx, sm, currentEraH)

			r.NoError(staking.TestOnlyPutCandidateRewardSnapshotFor(sm, candAddr, &staking.CandidateRewardSnapshot{
				// All to voters, so the commission leg never touches the fund
				// and this test stays about the snapshot.
				EpochCommissionBasisPoints: 0,
				BlockCommissionBasisPoints: 0,
				TotalWeight:                big.NewInt(1_000_000),
				FreezeHeight:               tc.snapshotFreezeH,
				SelfStakeBucketIdx:         7,
			}))

			out := &epochGrantResult{
				transactionLogs: make([]*action.TransactionLog, 0),
				rewardLogs:      make([]*action.Log, 0),
				debit:           big.NewInt(0),
			}
			r.NoError(p.distributeEpochCommissions(ctx, sm, epochCommissionInputs{
				rewardedCandidates: []*state.Candidate{cand},
				addrs:              []address.Address{identityset.Address(2)},
				amounts:            []*big.Int{big.NewInt(1_000)},
			}, out))

			poolBeforePlan, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
			r.NoError(err)
			r.Zero(big.NewInt(1_000).Cmp(poolBeforePlan))

			work, err := p.freezePendingPoolDrainWork(ctx, sm, currentEraH)
			r.NoError(err)
			if tc.wantPlanned {
				r.Len(work, 1)
				r.Zero(big.NewInt(1_000).Cmp(work[0].VoterAmountFrozen))
				r.Zero(big.NewInt(1_000_000).Cmp(work[0].TotalWeight))
			} else {
				r.Empty(work)
			}

			poolAfterPlan, err := p.readPendingBlockRewardPool(ctx, sm, candAddr.Bytes())
			r.NoError(err)
			r.Zero(poolBeforePlan.Cmp(poolAfterPlan),
				"plan construction must leave the pending pool unchanged")
		})
	}
}
