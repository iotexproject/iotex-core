// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestEpochDrainCursor_RoundTrip — Serialize→Deserialize preserves
// every field, including the frozen delegate work list with big.Int
// pool balances and the shard-walk resume point.
func TestEpochDrainCursor_RoundTrip(t *testing.T) {
	r := require.New(t)

	in := epochDrainCursor{
		TargetEra:       42,
		StartEpoch:      19,
		EndEpoch:        42,
		StartShard:      7,
		ShardsDone:      123,
		ResumeVoter:     identityset.Address(9).Bytes(),
		SettlementSeed:  []byte{7, 8, 9},
		Completed:       true,
		CompletedHeight: 12345,
		Delegates: []epochDrainDelegateWork{
			{
				CandidateIdentifier: identityset.Address(1).Bytes(),
				VoterAmountFrozen:   big.NewInt(1_000),
				RewardAddress:       identityset.Address(11).Bytes(),
				EpochCommission:     big.NewInt(300),
				TotalWeight:         big.NewInt(400),
				SnapshotHash:        []byte{1, 2, 3},
				FreezeHeight:        900,
				SelfStakeBucketIdx:  4,
			},
			{
				CandidateIdentifier: identityset.Address(2).Bytes(),
				VoterAmountFrozen:   big.NewInt(2_500_000),
				RewardAddress:       identityset.Address(12).Bytes(),
				EpochCommission:     big.NewInt(750_000),
				TotalWeight:         big.NewInt(1_000_000),
				SnapshotHash:        []byte{4, 5, 6},
				FreezeHeight:        900,
				SelfStakeBucketIdx:  noSelfStakeBucketIndex,
			},
		},
		Distributed: []*big.Int{big.NewInt(11), big.NewInt(22)},
	}

	raw, err := in.Serialize()
	r.NoError(err)
	r.NotEmpty(raw)

	var out epochDrainCursor
	r.NoError(out.Deserialize(raw))
	r.Equal(in.TargetEra, out.TargetEra)
	r.Equal(in.StartEpoch, out.StartEpoch)
	r.Equal(in.EndEpoch, out.EndEpoch)
	r.Equal(in.StartShard, out.StartShard)
	r.Equal(in.ShardsDone, out.ShardsDone)
	r.Equal(in.ResumeVoter, out.ResumeVoter)
	r.Equal(in.SettlementSeed, out.SettlementSeed)
	r.Equal(in.Completed, out.Completed)
	r.Equal(in.CompletedHeight, out.CompletedHeight)
	r.Len(out.Delegates, 2)
	for i := range in.Delegates {
		r.Equal(in.Delegates[i].CandidateIdentifier, out.Delegates[i].CandidateIdentifier)
		r.Equal(in.Delegates[i].RewardAddress, out.Delegates[i].RewardAddress)
		r.Zero(in.Delegates[i].VoterAmountFrozen.Cmp(out.Delegates[i].VoterAmountFrozen),
			"delegate %d pool amount mismatch: in=%s out=%s",
			i, in.Delegates[i].VoterAmountFrozen, out.Delegates[i].VoterAmountFrozen)
		r.Zero(in.Delegates[i].EpochCommission.Cmp(out.Delegates[i].EpochCommission),
			"delegate %d epoch commission mismatch: in=%s out=%s",
			i, in.Delegates[i].EpochCommission, out.Delegates[i].EpochCommission)
		r.Zero(in.Delegates[i].TotalWeight.Cmp(out.Delegates[i].TotalWeight))
		r.Equal(in.Delegates[i].SnapshotHash, out.Delegates[i].SnapshotHash)
		r.Equal(in.Delegates[i].FreezeHeight, out.Delegates[i].FreezeHeight)
		r.Equal(in.Delegates[i].SelfStakeBucketIdx, out.Delegates[i].SelfStakeBucketIdx)
		// The per-delegate running total rides the read-only view field.
		r.Zero(in.Distributed[i].Cmp(out.Distributed[i]))
	}
}

// TestEpochDrainCursor_ShardCountsRejectOutOfRange pins the decode guards.
// ShardsDone is on the wire as a uint32 because 256 — the "drain finished"
// value — does not fit a uint8, which makes an out-of-range value
// representable and therefore something that has to be refused rather than
// silently truncated into a valid-looking position.
func TestEpochDrainCursor_ShardCountsRejectOutOfRange(t *testing.T) {
	r := require.New(t)

	done, err := decodeShardsDone(uint32(totalShards))
	r.NoError(err, "the full shard count is the legal 'finished' value")
	r.Equal(totalShards, done)
	_, err = decodeShardsDone(uint32(totalShards) + 1)
	r.Error(err)

	_, err = decodeShardCount(uint32(totalShards)-1, "start shard")
	r.NoError(err)
	_, err = decodeShardCount(uint32(totalShards), "start shard")
	r.Error(err, "a start shard equal to the shard count addresses no shard")

	raw, err := proto.Marshal(&rewardingpb.EpochDrainCursor{
		TargetEra: 1, ShardsDone: uint32(totalShards) + 5,
	})
	r.NoError(err)
	var cursor epochDrainCursor
	r.Error(cursor.Deserialize(raw))
}

// TestEpochDrainCursor_ShardWalkOrder pins the wrap-around walk: starting at
// StartShard, ShardsDone shards later, modulo the shard count, and finished
// once every shard has been visited exactly once.
func TestEpochDrainCursor_ShardWalkOrder(t *testing.T) {
	r := require.New(t)

	c := &epochDrainCursor{StartShard: 250}
	seen := make(map[byte]bool, totalShards)
	for i := uint16(0); i < totalShards; i++ {
		r.False(c.drainFinished(), "not finished after %d of %d shards", i, totalShards)
		shard := c.currentShard()
		r.Falsef(seen[shard], "shard %d visited twice", shard)
		seen[shard] = true
		c.ShardsDone++
	}
	r.True(c.drainFinished())
	r.Len(seen, int(totalShards))
	r.Equal(byte(250), (&epochDrainCursor{StartShard: 250}).currentShard())
	r.Equal(byte(0), (&epochDrainCursor{StartShard: 250, ShardsDone: 6}).currentShard())
}

func TestRewardEraEpochRange(t *testing.T) {
	r := require.New(t)
	r.Equal(uint64(0), rewardEraStartEpoch(0, 24))
	r.Equal(uint64(1), rewardEraStartEpoch(12, 24))
	r.Equal(uint64(1), rewardEraStartEpoch(24, 24))
	r.Equal(uint64(25), rewardEraStartEpoch(48, 24))

	legacy := &epochDrainCursor{TargetEra: 48}
	start, end := legacy.epochRange(24)
	r.Equal(uint64(25), start)
	r.Equal(uint64(48), end)
	carried := &epochDrainCursor{TargetEra: 48, StartEpoch: 1, EndEpoch: 48}
	start, end = carried.epochRange(24)
	r.Equal(uint64(1), start)
	r.Equal(uint64(48), end)
}

func TestSettlementSeedAndOffsets(t *testing.T) {
	r := require.New(t)
	parent := hash.Hash256b([]byte("parent-a"))
	ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
		Tip: protocol.TipInfo{Hash: parent},
	})

	seed := settlementSeed(ctx, 42)
	r.Equal(seed, settlementSeed(ctx, 42))
	r.NotEqual(seed, settlementSeed(ctx, 43))

	otherCtx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
		Tip: protocol.TipInfo{Hash: hash.Hash256b([]byte("parent-b"))},
	})
	r.NotEqual(seed, settlementSeed(otherCtx, 42))
	r.Zero(settlementListOffset(seed[:], 0))
	r.Less(settlementListOffset(seed[:], 7), uint32(7))
	r.Equal(settlementListOffset(seed[:], 7), settlementListOffset(seed[:], 7))
}

// TestSettlementStartShard pins the rotation the drain starts from: derived
// from the settlement seed alone, in range, and stable for a given seed.
func TestSettlementStartShard(t *testing.T) {
	r := require.New(t)
	for i := 0; i < 64; i++ {
		seed := hash.Hash256b([]byte{byte(i)})
		got := settlementStartShard(seed[:])
		r.Equal(got, settlementStartShard(seed[:]), "start shard must be a pure function of the seed")
		r.Less(uint16(got), totalShards)
	}
	// Every shard is reachable, so no address prefix owns the head of the walk.
	distinct := make(map[uint8]bool)
	for i := 0; i < 4096; i++ {
		seed := hash.Hash256b([]byte{byte(i), byte(i >> 8)})
		distinct[settlementStartShard(seed[:])] = true
	}
	r.Greater(len(distinct), 200, "seed must spread across the shard space")
}

func TestEpochDrainCursor_OldWireDefaultsToCanonicalOrder(t *testing.T) {
	r := require.New(t)
	raw, err := proto.Marshal(&rewardingpb.EpochDrainCursor{
		TargetEra: 9,
		Delegates: []*rewardingpb.EpochDrainDelegateWork{{
			CandidateIdentifier: identityset.Address(1).Bytes(),
		}},
	})
	r.NoError(err)

	var cursor epochDrainCursor
	r.NoError(cursor.Deserialize(raw))
	r.Empty(cursor.SettlementSeed)
	r.Zero(cursor.StartShard)
	r.Zero(cursor.ShardsDone)
	r.Empty(cursor.ResumeVoter)
	// A record with no freeze height predates the era metadata; its zero
	// self-stake index is an absence, not "bucket 0".
	r.Zero(cursor.Delegates[0].FreezeHeight)
	r.Equal(uint64(staking.NoSelfStakeBucketIndex), cursor.Delegates[0].SelfStakeBucketIdx)
	r.False(cursor.Delegates[0].hasFrozenEra())
}

// TestEpochDrainCursor_EmptyDelegates — a cursor with no delegate work
// (era boundary hit but pool was empty) round-trips as an empty slice,
// not a nil.
func TestEpochDrainCursor_EmptyDelegates(t *testing.T) {
	r := require.New(t)

	in := epochDrainCursor{TargetEra: 1}
	raw, err := in.Serialize()
	r.NoError(err)

	var out epochDrainCursor
	r.NoError(out.Deserialize(raw))
	r.Equal(uint64(1), out.TargetEra)
	r.Zero(out.ShardsDone)
	r.Empty(out.Delegates)
}

// TestEpochDrainCursor_ZeroPoolAmount — a delegate whose pool balance
// is zero at Phase A freeze round-trips as a big.Int with Sign() == 0
// (not nil), so chunk callers can safely call amt.Sign() without a
// nil check.
func TestEpochDrainCursor_ZeroPoolAmount(t *testing.T) {
	r := require.New(t)

	in := epochDrainCursor{
		TargetEra: 3,
		Delegates: []epochDrainDelegateWork{
			{
				CandidateIdentifier: identityset.Address(5).Bytes(),
				VoterAmountFrozen:   new(big.Int),
			},
		},
	}
	raw, err := in.Serialize()
	r.NoError(err)

	var out epochDrainCursor
	r.NoError(out.Deserialize(raw))
	r.Len(out.Delegates, 1)
	r.NotNil(out.Delegates[0].VoterAmountFrozen)
	r.Equal(0, out.Delegates[0].VoterAmountFrozen.Sign())
}

// TestEpochDrainCursor_ReadMissingReturnsNil — an unpopulated cursor
// key returns (nil, nil) so callers can use presence as the drain-in-
// progress signal without a distinct sentinel error.
func TestEpochDrainCursor_ReadMissingReturnsNil(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	c, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.Nil(c)
}

// TestEpochDrainCursor_WriteReadDelete — full lifecycle. Write persists
// the cursor; read returns byte-equal fields; delete makes read return
// (nil, nil); a second delete is a no-op (idempotency).
func TestEpochDrainCursor_WriteReadDelete(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	in := &epochDrainCursor{
		TargetEra:  99,
		ShardsDone: 3,
		Delegates: []epochDrainDelegateWork{
			{
				CandidateIdentifier: identityset.Address(1).Bytes(),
				VoterAmountFrozen:   big.NewInt(10_000),
			},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, in))

	got, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(got)
	r.Equal(in.TargetEra, got.TargetEra)
	r.Equal(in.ShardsDone, got.ShardsDone)
	r.Len(got.Delegates, 1)
	r.Equal(in.Delegates[0].CandidateIdentifier, got.Delegates[0].CandidateIdentifier)
	r.Zero(in.Delegates[0].VoterAmountFrozen.Cmp(got.Delegates[0].VoterAmountFrozen))

	r.NoError(p.deleteEpochDrainCursor(ctx, sm))
	got, err = p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.Nil(got)

	// Second delete: no error even though the key is already gone.
	r.NoError(p.deleteEpochDrainCursor(ctx, sm))
}

// TestEpochDrainCursor_WriteOverwrites — writing a new cursor over an
// existing entry replaces it wholesale (proto3 wire semantics for
// repeated fields could otherwise merge lists on some codecs).
func TestEpochDrainCursor_WriteOverwrites(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	first := &epochDrainCursor{
		TargetEra: 10,
		Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(1).Bytes(), VoterAmountFrozen: big.NewInt(1)},
			{CandidateIdentifier: identityset.Address(2).Bytes(), VoterAmountFrozen: big.NewInt(2)},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, first))

	second := &epochDrainCursor{
		TargetEra:   10,
		ShardsDone:  1,
		ResumeVoter: identityset.Address(4).Bytes(),
		Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(3).Bytes(), VoterAmountFrozen: big.NewInt(9)},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, second))

	got, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(got)
	r.Equal(uint16(1), got.ShardsDone)
	r.Equal(identityset.Address(4).Bytes(), got.ResumeVoter,
		"second write must replace ResumeVoter, not merge")
	r.Len(got.Delegates, 1, "second write must replace, not append")
	r.Equal(identityset.Address(3).Bytes(), got.Delegates[0].CandidateIdentifier)

	// Finishing a shard clears the resume point; the cleared value must
	// round-trip as empty rather than carrying the prior address forward.
	third := &epochDrainCursor{
		TargetEra:  10,
		ShardsDone: 2,
		Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(4).Bytes(), VoterAmountFrozen: big.NewInt(7)},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, third))
	got, err = p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(got)
	r.Empty(got.ResumeVoter, "a cleared resume point must not carry the prior value")
}

func TestEpochDrainCursor_ProgressWritePreservesPlan(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	settlementSeed := hash.Hash256b([]byte("settlement-seed"))
	snapshot1 := hash.Hash256b([]byte("snapshot-1"))
	snapshot2 := hash.Hash256b([]byte("snapshot-2"))

	cursor := &epochDrainCursor{
		TargetEra:      24,
		StartEpoch:     1,
		EndEpoch:       24,
		SettlementSeed: settlementSeed[:],
		StartShard:     31,
		Delegates: []epochDrainDelegateWork{
			{
				CandidateIdentifier: identityset.Address(1).Bytes(),
				VoterAmountFrozen:   big.NewInt(100),
				RewardAddress:       identityset.Address(11).Bytes(),
				TotalWeight:         big.NewInt(10),
				SnapshotHash:        snapshot1[:],
			},
			{
				CandidateIdentifier: identityset.Address(2).Bytes(),
				VoterAmountFrozen:   big.NewInt(200),
				RewardAddress:       identityset.Address(12).Bytes(),
				TotalWeight:         big.NewInt(20),
				SnapshotHash:        snapshot2[:],
			},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))

	planBefore := &epochDrainPlan{}
	_, err := p.state(ctx, sm, state.EpochDrainPlanKey, planBefore)
	r.NoError(err)
	planBytesBefore, err := planBefore.Serialize()
	r.NoError(err)

	cursor.ShardsDone = 40
	cursor.ResumeVoter = identityset.Address(7).Bytes()
	cursor.Distributed = []*big.Int{new(big.Int), big.NewInt(75)}
	markDelegateSkipped(cursor, 0)
	r.NoError(p.writeEpochDrainProgress(ctx, sm, cursor))

	planAfter := &epochDrainPlan{}
	_, err = p.state(ctx, sm, state.EpochDrainPlanKey, planAfter)
	r.NoError(err)
	planBytesAfter, err := planAfter.Serialize()
	r.NoError(err)
	r.Equal(planBytesBefore, planBytesAfter)

	progress := &epochDrainProgress{}
	_, err = p.state(ctx, sm, state.EpochDrainCursorKey, progress)
	r.NoError(err)
	progressBytes, err := progress.Serialize()
	r.NoError(err)
	r.Less(len(progressBytes), len(planBytesBefore))

	got, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.Equal(uint16(40), got.ShardsDone)
	r.Equal(identityset.Address(7).Bytes(), got.ResumeVoter)
	r.Equal(uint8(31), got.StartShard, "the immutable plan supplies the rotation")
	r.True(delegateSkipped(got, 0))
	r.Zero(got.distributedAt(0).Sign())
	r.Zero(big.NewInt(75).Cmp(got.distributedAt(1)))
}

// TestEpochDrainCursor_ProgressWithoutPlanIsRefused pins the composed read:
// the running per-delegate totals are only meaningful next to the frozen work
// list they index into, so a half-written settlement is an error rather than a
// cursor with an empty delegate list (which would drain nothing and then sweep
// every pool to the orphan path).
func TestEpochDrainCursor_ProgressWithoutPlanIsRefused(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	r.NoError(p.putState(ctx, sm, state.EpochDrainCursorKey, &epochDrainProgress{TargetEra: 5}))
	_, err := p.readEpochDrainCursor(ctx, sm)
	r.ErrorContains(err, "progress exists without a plan")
}

// TestEpochDrainCursor_DistributedLongerThanPlanIsRefused guards the payout
// clamp's index: a progress record carrying more running totals than the plan
// has delegates cannot be aligned positionally, and guessing an alignment
// would mis-attribute money already paid.
func TestEpochDrainCursor_DistributedLongerThanPlanIsRefused(t *testing.T) {
	r := require.New(t)
	_, err := epochDrainCursorFromState(
		&epochDrainPlan{TargetEra: 1, Delegates: make([]epochDrainDelegateWork, 1)},
		&epochDrainProgress{TargetEra: 1, Distributed: []*big.Int{big.NewInt(1), big.NewInt(2)}},
	)
	r.ErrorContains(err, "distributed totals")
}
