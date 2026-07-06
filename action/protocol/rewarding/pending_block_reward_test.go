// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"math/big"
	"sort"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestPendingBlockRewardRoundtrip verifies proto serialize/deserialize keeps
// amount, rewardAddr, and commissionRate identical, including zero and large
// amount edge cases.
func TestPendingBlockRewardRoundtrip(t *testing.T) {
	rewardAddr := identityset.Address(1)
	cases := []struct {
		name string
		in   pendingBlockReward
	}{
		{
			name: "zero amount",
			in:   pendingBlockReward{amount: big.NewInt(0), rewardAddr: rewardAddr, commissionRate: 500},
		},
		{
			name: "small amount",
			in:   pendingBlockReward{amount: big.NewInt(12345), rewardAddr: rewardAddr, commissionRate: 1000},
		},
		{
			name: "large amount",
			in: pendingBlockReward{
				amount:         new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1_000_000)),
				rewardAddr:     rewardAddr,
				commissionRate: commissionRateDenominator,
			},
		},
		{
			name: "nil reward addr",
			in:   pendingBlockReward{amount: big.NewInt(1), rewardAddr: nil, commissionRate: 0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blob, err := tc.in.Serialize()
			require.NoError(t, err)
			var out pendingBlockReward
			require.NoError(t, out.Deserialize(blob))
			require.Equal(t, tc.in.amount, out.amount)
			require.Equal(t, tc.in.commissionRate, out.commissionRate)
			if tc.in.rewardAddr == nil {
				require.Nil(t, out.rewardAddr)
			} else {
				require.Equal(t, tc.in.rewardAddr.String(), out.rewardAddr.String())
			}
		})
	}
}

// TestPendingBlockRewardDeserializeBadAmount rejects a non-numeric amount
// blob rather than silently zeroing.
func TestPendingBlockRewardDeserializeBadAmount(t *testing.T) {
	bad := &rewardingpb.PendingBlockReward{Amount: "not-a-number"}
	blob, err := proto.Marshal(bad)
	require.NoError(t, err)
	var out pendingBlockReward
	require.Error(t, out.Deserialize(blob))
}

// TestPendingBlockRewardIndexRoundtrip verifies serialize/deserialize preserves
// sort order and identity contents. Empty index round-trips to empty index.
func TestPendingBlockRewardIndexRoundtrip(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		in := pendingBlockRewardIndex{}
		blob, err := in.Serialize()
		require.NoError(t, err)
		var out pendingBlockRewardIndex
		require.NoError(t, out.Deserialize(blob))
		require.Empty(t, out.identities)
	})
	t.Run("populated", func(t *testing.T) {
		in := pendingBlockRewardIndex{
			identities: []address.Address{
				identityset.Address(3),
				identityset.Address(1),
				identityset.Address(2),
			},
		}
		// insert sort them first (mimics the invariant)
		sort.Slice(in.identities, func(i, j int) bool {
			return bytes.Compare(in.identities[i].Bytes(), in.identities[j].Bytes()) < 0
		})
		blob, err := in.Serialize()
		require.NoError(t, err)
		var out pendingBlockRewardIndex
		require.NoError(t, out.Deserialize(blob))
		require.Len(t, out.identities, 3)
		for i := range in.identities {
			require.Equal(t, in.identities[i].String(), out.identities[i].String())
		}
	})
}

// TestPendingBlockRewardIndexInsertSemantics: absent → inserted sorted;
// present → no-op; monotonic insert of a reversed sequence still yields
// sorted result.
func TestPendingBlockRewardIndexInsertSemantics(t *testing.T) {
	idx := pendingBlockRewardIndex{}
	a := identityset.Address(5)
	b := identityset.Address(3)
	c := identityset.Address(9)
	require.True(t, idx.insert(a))
	require.True(t, idx.insert(b))
	require.True(t, idx.insert(c))
	// duplicate insert no-op
	require.False(t, idx.insert(a))
	require.False(t, idx.insert(b))
	require.False(t, idx.insert(c))
	// sorted by bytes
	for i := 1; i < len(idx.identities); i++ {
		require.True(t, bytes.Compare(idx.identities[i-1].Bytes(), idx.identities[i].Bytes()) < 0)
	}
	require.Len(t, idx.identities, 3)
}

// TestCreditPendingBlockReward_CreateAndAccumulate: a first credit inserts an
// entry + adds to the index; a second credit for the same delegate accumulates
// the amount and does NOT double-insert the index.
func TestCreditPendingBlockReward_CreateAndAccumulate(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)
	cand := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 500,
	}

	require.NoError(t, p.creditPendingBlockReward(ctx, sm, cand, big.NewInt(1000)))
	entry := readPendingEntry(t, ctx, p, sm, candIdentity)
	require.Equal(t, big.NewInt(1000), entry.amount)
	require.Equal(t, rewardAddr.String(), entry.rewardAddr.String())
	require.Equal(t, uint64(500), entry.commissionRate)
	idx := readPendingIndex(t, ctx, p, sm)
	require.Len(t, idx.identities, 1)
	require.Equal(t, candIdentity.String(), idx.identities[0].String())

	// Second credit — refreshes commission rate to newer snapshot value.
	cand2 := *cand
	cand2.CommissionRate = 700
	require.NoError(t, p.creditPendingBlockReward(ctx, sm, &cand2, big.NewInt(500)))
	entry = readPendingEntry(t, ctx, p, sm, candIdentity)
	require.Equal(t, big.NewInt(1500), entry.amount)
	require.Equal(t, uint64(700), entry.commissionRate)
	idx = readPendingIndex(t, ctx, p, sm)
	require.Len(t, idx.identities, 1, "dedup: same identity must not re-insert")
}

// TestCreditPendingBlockReward_MultipleDelegates: two producers get separate
// pool entries; the index tracks both, sorted.
func TestCreditPendingBlockReward_MultipleDelegates(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	makeCand := func(idIdx, rewardIdx int) *state.Candidate {
		return &state.Candidate{
			RewardAddress:  identityset.Address(rewardIdx).String(),
			Identity:       identityset.Address(idIdx).String(),
			CommissionRate: 500,
		}
	}
	candA := makeCand(10, 1)
	candB := makeCand(11, 2)

	require.NoError(t, p.creditPendingBlockReward(ctx, sm, candA, big.NewInt(100)))
	require.NoError(t, p.creditPendingBlockReward(ctx, sm, candB, big.NewInt(200)))

	idx := readPendingIndex(t, ctx, p, sm)
	require.Len(t, idx.identities, 2)
	// invariant: sorted by bytes
	require.True(t, bytes.Compare(idx.identities[0].Bytes(), idx.identities[1].Bytes()) < 0)

	entryA := readPendingEntry(t, ctx, p, sm, identityset.Address(10))
	entryB := readPendingEntry(t, ctx, p, sm, identityset.Address(11))
	require.Equal(t, big.NewInt(100), entryA.amount)
	require.Equal(t, big.NewInt(200), entryB.amount)
}

// TestCreditPendingBlockReward_ZeroAmountNoop: crediting 0 does nothing —
// no entry, no index insert.
func TestCreditPendingBlockReward_ZeroAmountNoop(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	cand := &state.Candidate{
		RewardAddress:  identityset.Address(1).String(),
		Identity:       identityset.Address(10).String(),
		CommissionRate: 500,
	}
	require.NoError(t, p.creditPendingBlockReward(ctx, sm, cand, big.NewInt(0)))
	// index should not exist
	idx := pendingBlockRewardIndex{}
	_, err := p.state(ctx, sm, _pendingBlockRewardIndexKey, &idx)
	require.Error(t, err)
	require.Equal(t, state.ErrStateNotExist, errors.Cause(err))
}

// TestBlockRewardEligibleForVoterSplit covers the routing predicate.
func TestBlockRewardEligibleForVoterSplit(t *testing.T) {
	_, _, p := setupVoterRewardCtx(t, 0)
	fCtxOn := protocol.FeatureCtx{NoVoterRewardDistribution: false}
	fCtxOff := protocol.FeatureCtx{NoVoterRewardDistribution: true}
	candGood := &state.Candidate{
		Identity:       identityset.Address(10).String(),
		CommissionRate: 500,
	}
	require.True(t, p.blockRewardEligibleForVoterSplit(fCtxOn, candGood))
	require.False(t, p.blockRewardEligibleForVoterSplit(fCtxOff, candGood), "pre-flag")
	require.False(t, p.blockRewardEligibleForVoterSplit(fCtxOn, nil), "nil cand")
	require.False(t, p.blockRewardEligibleForVoterSplit(fCtxOn, &state.Candidate{CommissionRate: 500}), "empty identity")
	require.False(t, p.blockRewardEligibleForVoterSplit(fCtxOn, &state.Candidate{Identity: candGood.Identity}), "zero rate")
	require.False(t, p.blockRewardEligibleForVoterSplit(
		fCtxOn, &state.Candidate{Identity: candGood.Identity, CommissionRate: commissionRateDenominator + 1}),
		"rate above denominator")
}

// TestDrainPendingBlockRewards_EmptyPoolNoop: no index → drain returns nil,
// nothing gets deleted, no logs.
func TestDrainPendingBlockRewards_EmptyPoolNoop(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	logs, err := p.drainPendingBlockRewards(ctx, sm, state.CandidateList{}, 1, hash.ZeroHash256)
	require.NoError(t, err)
	require.Nil(t, logs)
}

// TestDrainPendingBlockRewards_TopN: delegate is in the top-N snapshot list at
// drain time → distributeVoterReward uses the fresher current-epoch commission
// rate; pool entry deleted; index cleared; per-address balances credit.
func TestDrainPendingBlockRewards_TopN(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)

	// Credit pool with 10% commission stored on the entry.
	credit := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 1000,
	}
	require.NoError(t, p.creditPendingBlockReward(ctx, sm, credit, big.NewInt(1_000_000)))

	// Voter snapshot (three equal voters).
	voters := []address.Address{identityset.Address(20), identityset.Address(21), identityset.Address(22)}
	weights := []*big.Int{big.NewInt(100), big.NewInt(100), big.NewInt(100)}
	require.NoError(t, staking.WriteVoterWeightSnapshotForTest(sm, candIdentity, voters, weights))

	// Current-epoch snapshot list holds the delegate with a DIFFERENT (fresher) rate.
	// The drain must prefer this rate over the frozen entry rate.
	freshCand := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 2000, // 20%
	}
	logs, err := p.drainPendingBlockRewards(
		ctx, sm, state.CandidateList{freshCand}, 42, hash.ZeroHash256)
	require.NoError(t, err)
	// 3 VOTER_REWARD + 1 DELEGATE_COMMISSION
	require.Len(t, logs, 4)

	// Fresh rate 20% → commission = 200_000; voterPool = 800_000; each voter = 266_666; dust = 2.
	require.Equal(t, big.NewInt(266_666), unclaimed(t, ctx, p, sm, voters[0]))
	require.Equal(t, big.NewInt(266_666), unclaimed(t, ctx, p, sm, voters[1]))
	require.Equal(t, big.NewInt(266_666), unclaimed(t, ctx, p, sm, voters[2]))
	// delegate = 200_000 + 2 = 200_002
	require.Equal(t, big.NewInt(200_002), unclaimed(t, ctx, p, sm, rewardAddr))

	// Pool entry and index both cleared.
	entry := pendingBlockReward{}
	_, err = p.state(ctx, sm, pendingBlockRewardKey(candIdentity), &entry)
	require.Equal(t, state.ErrStateNotExist, errors.Cause(err))
	idx := pendingBlockRewardIndex{}
	_, err = p.state(ctx, sm, _pendingBlockRewardIndexKey, &idx)
	require.Equal(t, state.ErrStateNotExist, errors.Cause(err))
}

// TestDrainPendingBlockRewards_OrphanWithSnapshot: delegate is NOT in the
// current-epoch top-N candidate list, but a voter-weight snapshot from an
// earlier epoch still exists. Drain uses the entry's frozen rate/rewardAddr
// and splits per the earlier snapshot.
func TestDrainPendingBlockRewards_OrphanWithSnapshot(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)

	credit := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 1000, // 10% frozen at credit time
	}
	require.NoError(t, p.creditPendingBlockReward(ctx, sm, credit, big.NewInt(1_000_000)))

	voters := []address.Address{identityset.Address(20), identityset.Address(21), identityset.Address(22)}
	weights := []*big.Int{big.NewInt(100), big.NewInt(100), big.NewInt(100)}
	require.NoError(t, staking.WriteVoterWeightSnapshotForTest(sm, candIdentity, voters, weights))

	// Empty current-epoch top-N list → orphan path.
	logs, err := p.drainPendingBlockRewards(
		ctx, sm, state.CandidateList{}, 42, hash.ZeroHash256)
	require.NoError(t, err)
	require.Len(t, logs, 4)

	// Frozen rate 10% → commission = 100_000; voterPool = 900_000; each voter = 300_000; dust = 0.
	require.Equal(t, big.NewInt(300_000), unclaimed(t, ctx, p, sm, voters[0]))
	require.Equal(t, big.NewInt(300_000), unclaimed(t, ctx, p, sm, voters[1]))
	require.Equal(t, big.NewInt(300_000), unclaimed(t, ctx, p, sm, voters[2]))
	require.Equal(t, big.NewInt(100_000), unclaimed(t, ctx, p, sm, rewardAddr))
}

// TestDrainPendingBlockRewards_OrphanNoSnapshot: delegate absent from top-N
// AND no voter-weight snapshot ever written → delegate keeps the full amount
// as a single DELEGATE_COMMISSION log (belt-and-suspenders orphan path).
func TestDrainPendingBlockRewards_OrphanNoSnapshot(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)

	credit := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 500,
	}
	require.NoError(t, p.creditPendingBlockReward(ctx, sm, credit, big.NewInt(1_000)))

	logs, err := p.drainPendingBlockRewards(
		ctx, sm, state.CandidateList{}, 1, hash.ZeroHash256)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, rewardingpb.RewardLog_DELEGATE_COMMISSION, decodeRewardLog(t, logs[0].Data).Type)
	require.Equal(t, big.NewInt(1_000), unclaimed(t, ctx, p, sm, rewardAddr))
}

// TestDrainPendingBlockRewards_LegacyFallbackBranch verifies the belt-and-
// suspenders branch inside the drain loop: if distributeVoterReward returns
// handled=false (i.e. the fresher top-N snapshot signals 0% commission), the
// drain must still pay out via a direct grantToAccount + EPOCH_REWARD log so
// pool funds are never stranded.
func TestDrainPendingBlockRewards_LegacyFallbackBranch(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)
	credit := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 500,
	}
	require.NoError(t, p.creditPendingBlockReward(ctx, sm, credit, big.NewInt(500)))

	freshCand := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 0,
	}
	logs, err := p.drainPendingBlockRewards(
		ctx, sm, state.CandidateList{freshCand}, 1, hash.ZeroHash256)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	rl := decodeRewardLog(t, logs[0].Data)
	require.Equal(t, rewardingpb.RewardLog_EPOCH_REWARD, rl.Type)
	require.Equal(t, big.NewInt(500), unclaimed(t, ctx, p, sm, rewardAddr))
}

// --- helpers ---

func readPendingEntry(
	t *testing.T, ctx context.Context, p *Protocol, sm protocol.StateManager, id address.Address,
) pendingBlockReward {
	t.Helper()
	entry := pendingBlockReward{}
	_, err := p.state(ctx, sm, pendingBlockRewardKey(id), &entry)
	require.NoError(t, err)
	return entry
}

func readPendingIndex(t *testing.T, ctx context.Context, p *Protocol, sm protocol.StateManager) pendingBlockRewardIndex {
	t.Helper()
	idx := pendingBlockRewardIndex{}
	_, err := p.state(ctx, sm, _pendingBlockRewardIndexKey, &idx)
	require.NoError(t, err)
	return idx
}
