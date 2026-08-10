// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// The poll snapshot lives under one per-candidate key with no era qualifier.
// A candidate frozen in era N and skipped at era N+1's freeze therefore still
// reads back, carrying era N's FreezeHeight H'. Handed to the drain unchecked,
// H' becomes the evaluation height for weights whose buckets come from era
// N+1's copy-on-write window (opened at H): the numerator is bucket membership
// at H, the denominator is the candidate's Votes at H'. Nothing errors — the
// era just settles on a mixed basis.
//
// freezeDelegateDrainWork treats such a snapshot as absent, which is the path
// the freezer already produces for a candidate it never froze.

// TestFreezeDelegateDrainWork_StaleSnapshotTreatedAsAbsent is the guard.
func TestFreezeDelegateDrainWork_StaleSnapshotTreatedAsAbsent(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)

	const currentEraH = uint64(20_000)
	const previousEraH = uint64(10_000)
	openEraWindowForTest(t, ctx, sm, currentEraH)

	var stubHash hash.Hash256
	copy(stubHash[:], []byte{0xab, 0xcd})
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candAddr, &staking.CandidatePollSnapshot{
		OnchainRewardEnabled:       true,
		EpochCommissionBasisPoints: 3_000,
		BlockCommissionBasisPoints: 3_000,
		TotalWeight:                big.NewInt(1_000_000),
		FreezeHeight:               previousEraH,
		SelfStakeBucketIdx:         7,
		SnapshotHash:               stubHash,
	}))

	work, err := p.freezeDelegateDrainWork(
		sm, candAddr.Bytes(), identityset.Address(2),
		big.NewInt(0), big.NewInt(5_000), currentEraH,
	)
	r.NoError(err, "a stale snapshot is chain state, so it must degrade rather than halt the block")
	r.Zero(work.TotalWeight.Sign(),
		"a previous era's denominator must not be used against this era's frozen buckets")
	r.Zero(work.FreezeHeight,
		"a previous era's evaluation height must not reach the weight recompute")
	r.Equal(staking.NoSelfStakeBucketIndex, work.SelfStakeBucketIdx)
	r.Equal(hash.ZeroHash256[:], work.SnapshotHash)
	// The pool itself is untouched: it rolls into a later era rather than being
	// paid out on a mixed basis.
	r.Zero(big.NewInt(5_000).Cmp(work.VoterAmountFrozen))
}

// TestFreezeDelegateDrainWork_CurrentEraSnapshotStillUsed is the regression
// lock on the other side. The guard must key on the era, not on the mere
// presence of a FreezeHeight — an over-broad version would defer every
// delegate, forever, and IIP-59 would pay no voters at all.
func TestFreezeDelegateDrainWork_CurrentEraSnapshotStillUsed(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)

	const currentEraH = uint64(20_000)
	openEraWindowForTest(t, ctx, sm, currentEraH)

	var stubHash hash.Hash256
	copy(stubHash[:], []byte{0xab, 0xcd})
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candAddr, &staking.CandidatePollSnapshot{
		OnchainRewardEnabled:       true,
		EpochCommissionBasisPoints: 3_000,
		BlockCommissionBasisPoints: 3_000,
		TotalWeight:                big.NewInt(1_000_000),
		FreezeHeight:               currentEraH,
		SelfStakeBucketIdx:         7,
		SnapshotHash:               stubHash,
	}))

	work, err := p.freezeDelegateDrainWork(
		sm, candAddr.Bytes(), identityset.Address(2),
		big.NewInt(0), big.NewInt(5_000), currentEraH,
	)
	r.NoError(err)
	r.Zero(big.NewInt(1_000_000).Cmp(work.TotalWeight))
	r.Equal(currentEraH, work.FreezeHeight)
	r.Equal(uint64(7), work.SelfStakeBucketIdx)
	r.Equal(stubHash[:], work.SnapshotHash)
}

// TestFreezeDelegateDrainWork_MissingSnapshotStillDefers pins the pre-existing
// "no snapshot -> zero weight -> defer" path the guard degrades into. If this
// ever became an error, the guard would turn a chain-state condition into a
// halted block.
func TestFreezeDelegateDrainWork_MissingSnapshotStillDefers(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)
	openEraWindowForTest(t, ctx, sm, 20_000)

	work, err := p.freezeDelegateDrainWork(
		sm, candAddr.Bytes(), identityset.Address(2),
		big.NewInt(0), big.NewInt(5_000), 20_000,
	)
	r.NoError(err)
	r.Zero(work.TotalWeight.Sign())
	r.Zero(work.FreezeHeight)
}

// TestDistributeEpochCommissions_StaleSnapshotDefersPool walks the real caller,
// which is what proves the era height actually travels from the copy-on-write
// window into the guard. A unit test on freezeDelegateDrainWork alone would
// still pass if distributeEpochCommissions handed it a zero.
func TestDistributeEpochCommissions_StaleSnapshotDefersPool(t *testing.T) {
	for _, tc := range []struct {
		name            string
		snapshotFreezeH uint64
		wantDeferred    bool
	}{
		{"stale snapshot defers", 10_000, true},
		{"current-era snapshot settles", 20_000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)

			const currentEraH = uint64(20_000)
			openEraWindowForTest(t, ctx, sm, currentEraH)

			r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candAddr, &staking.CandidatePollSnapshot{
				OnchainRewardEnabled: true,
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
				isEraBoundary:      true,
			}, out))

			r.Len(out.cursorEntries, 1)
			work := out.cursorEntries[0]
			r.Zero(big.NewInt(1_000).Cmp(work.VoterAmountFrozen),
				"the voter pool accrues either way; only its settlement basis is in question")
			if tc.wantDeferred {
				r.Zero(work.TotalWeight.Sign())
				r.Zero(work.FreezeHeight)
			} else {
				r.Zero(big.NewInt(1_000_000).Cmp(work.TotalWeight))
				r.Equal(currentEraH, work.FreezeHeight)
			}
		})
	}
}
