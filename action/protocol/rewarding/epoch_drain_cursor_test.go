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
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestEpochDrainCursor_RoundTrip — Serialize→Deserialize preserves
// every field, including the frozen delegate work list with big.Int
// pool balances and the circular address-walk resume point.
func TestEpochDrainCursor_RoundTrip(t *testing.T) {
	r := require.New(t)

	in := epochDrainCursor{
		epochDrainPlan: epochDrainPlan{
			TargetEra:      42,
			FreezeHeight:   900,
			SettlementSeed: []byte{7, 8, 9},
			Delegates: []epochDrainDelegateWork{
				{
					CandidateIdentifier: identityset.Address(1).Bytes(),
					VoterAmountFrozen:   big.NewInt(1_000),
					TotalWeight:         big.NewInt(400),
					SelfStakeBucketIdx:  4,
				},
				{
					CandidateIdentifier: identityset.Address(2).Bytes(),
					VoterAmountFrozen:   big.NewInt(2_500_000),
					TotalWeight:         big.NewInt(1_000_000),
					SelfStakeBucketIdx:  noSelfStakeBucketIndex,
				},
			},
		},
		epochDrainProgress: epochDrainProgress{
			ScanPhase:       voterScanHead,
			ResumeVoter:     identityset.Address(9).Bytes(),
			Distributed:     []*big.Int{big.NewInt(11), big.NewInt(22)},
			CompletedHeight: 12345,
		},
	}

	raw, err := in.Serialize()
	r.NoError(err)
	r.NotEmpty(raw)

	var out epochDrainCursor
	r.NoError(out.Deserialize(raw))
	r.Equal(in.TargetEra, out.TargetEra)
	r.Equal(in.FreezeHeight, out.FreezeHeight)
	r.Equal(in.ScanPhase, out.ScanPhase)
	r.Equal(in.ResumeVoter, out.ResumeVoter)
	r.Equal(in.SettlementSeed, out.SettlementSeed)
	r.Equal(in.CompletedHeight, out.CompletedHeight)
	r.Len(out.Delegates, 2)
	for i := range in.Delegates {
		r.Equal(in.Delegates[i].CandidateIdentifier, out.Delegates[i].CandidateIdentifier)
		r.Zero(in.Delegates[i].VoterAmountFrozen.Cmp(out.Delegates[i].VoterAmountFrozen),
			"delegate %d pool amount mismatch: in=%s out=%s",
			i, in.Delegates[i].VoterAmountFrozen, out.Delegates[i].VoterAmountFrozen)
		r.Zero(in.Delegates[i].TotalWeight.Cmp(out.Delegates[i].TotalWeight))
		r.Equal(in.Delegates[i].SelfStakeBucketIdx, out.Delegates[i].SelfStakeBucketIdx)
		// The per-delegate running total rides the read-only view field.
		r.Zero(in.Distributed[i].Cmp(out.Distributed[i]))
	}

	view := &rewardingpb.EpochDrainCursor{}
	r.NoError(proto.Unmarshal(raw, view))
	r.Equal(settlementStartVoter(in.SettlementSeed), view.GetStartVoter())
	r.Equal(uint32(voterScanHead), view.GetScanPhase())
}

// TestEpochDrainCursor_ScanPhaseRejectsOutOfRange pins the wire guard. The
// phase is encoded as uint32, so values outside tail/head/done must fail rather
// than silently becoming a valid-looking resume position.
func TestEpochDrainCursor_ScanPhaseRejectsOutOfRange(t *testing.T) {
	r := require.New(t)

	for _, phase := range []voterScanPhase{voterScanTail, voterScanHead, voterScanDone} {
		decoded, err := decodeVoterScanPhase(uint32(phase))
		r.NoError(err)
		r.Equal(phase, decoded)
	}
	_, err := decodeVoterScanPhase(uint32(voterScanDone) + 1)
	r.Error(err)

	raw, err := proto.Marshal(&rewardingpb.EpochDrainCursor{
		TargetEra: 1, ScanPhase: uint32(voterScanDone) + 1,
	})
	r.NoError(err)
	var cursor epochDrainCursor
	r.Error(cursor.Deserialize(raw))

	raw, err = proto.Marshal(&rewardingpb.EpochDrainProgress{
		ScanPhase: uint32(voterScanDone) + 1, SchemaVersion: _epochDrainProgressVersion,
	})
	r.NoError(err)
	var progress epochDrainProgress
	r.Error(progress.Deserialize(raw))
}

func TestEpochDrainProgressRejectsRetiredShardCursor(t *testing.T) {
	r := require.New(t)
	// Field 8 was shards_done. Its presence is deliberately irrelevant: a
	// missing schema version is enough to reject every retired cursor, including
	// one whose shards_done value encoded to the protobuf zero value.
	retired := []byte{0x40, 0x01}
	progress := &epochDrainProgress{}
	err := progress.Deserialize(retired)
	r.ErrorContains(err, "unsupported epoch drain progress version 0")
}

// TestEpochDrainCursor_ScanPhaseLifecycle pins the only legal completion
// transition: neither address range alone is complete; done is.
func TestEpochDrainCursor_ScanPhaseLifecycle(t *testing.T) {
	r := require.New(t)

	c := &epochDrainCursor{}
	r.Equal(voterScanTail, c.ScanPhase)
	r.False(c.drainFinished())
	c.ScanPhase = voterScanHead
	r.False(c.drainFinished())
	c.ScanPhase = voterScanDone
	r.True(c.drainFinished())
}

func TestRewardEraEpochRange(t *testing.T) {
	r := require.New(t)
	r.Equal(uint64(0), rewardEraStartEpoch(0, 24))
	r.Equal(uint64(1), rewardEraStartEpoch(12, 24))
	r.Equal(uint64(1), rewardEraStartEpoch(24, 24))
	r.Equal(uint64(25), rewardEraStartEpoch(48, 24))

}

func TestSettlementSeed(t *testing.T) {
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
}

// TestSettlementStartVoter pins the circular walk's random boundary: the first
// 20 seed bytes, copied into owned storage and zero-padded for short fixtures.
func TestSettlementStartVoter(t *testing.T) {
	r := require.New(t)
	distinct := make(map[string]bool)
	for i := 0; i < 64; i++ {
		seed := hash.Hash256b([]byte{byte(i)})
		got := settlementStartVoter(seed[:])
		r.Equal(seed[:20], got)
		r.Equal(got, settlementStartVoter(seed[:]), "start voter must be a pure function of the seed")
		distinct[string(got)] = true
	}
	r.Greater(len(distinct), 60, "different seeds must spread across the address space")

	short := []byte{1, 2, 3}
	want := make([]byte, 20)
	copy(want, short)
	got := settlementStartVoter(short)
	r.Equal(want, got)
	got[0] = 9
	r.Equal(byte(1), short[0], "the derived start must not alias the seed")
}

// TestEpochDrainCursor_EmptyDelegates — a cursor with no delegate work
// (era boundary hit but pool was empty) round-trips as an empty slice,
// not a nil.
func TestEpochDrainCursor_EmptyDelegates(t *testing.T) {
	r := require.New(t)

	in := epochDrainCursor{epochDrainPlan: epochDrainPlan{TargetEra: 1}}
	raw, err := in.Serialize()
	r.NoError(err)

	var out epochDrainCursor
	r.NoError(out.Deserialize(raw))
	r.Equal(uint64(1), out.TargetEra)
	r.Equal(voterScanTail, out.ScanPhase)
	r.Empty(out.Delegates)
}

// TestEpochDrainCursor_ZeroPoolAmount — a delegate whose pool balance
// is zero at era-boundary setup freeze round-trips as a big.Int with Sign() == 0
// (not nil), so chunk callers can safely call amt.Sign() without a
// nil check.
func TestEpochDrainCursor_ZeroPoolAmount(t *testing.T) {
	r := require.New(t)

	in := epochDrainCursor{
		epochDrainPlan: epochDrainPlan{TargetEra: 3, Delegates: []epochDrainDelegateWork{
			{
				CandidateIdentifier: identityset.Address(5).Bytes(),
				VoterAmountFrozen:   new(big.Int),
			},
		}},
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
		epochDrainPlan: epochDrainPlan{TargetEra: 99, Delegates: []epochDrainDelegateWork{
			{
				CandidateIdentifier: identityset.Address(1).Bytes(),
				VoterAmountFrozen:   big.NewInt(10_000),
			},
		}},
		epochDrainProgress: epochDrainProgress{ScanPhase: voterScanHead},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, in))

	got, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(got)
	r.Equal(in.TargetEra, got.TargetEra)
	r.Equal(in.ScanPhase, got.ScanPhase)
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
		epochDrainPlan: epochDrainPlan{TargetEra: 10, Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(1).Bytes(), VoterAmountFrozen: big.NewInt(1)},
			{CandidateIdentifier: identityset.Address(2).Bytes(), VoterAmountFrozen: big.NewInt(2)},
		}},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, first))

	second := &epochDrainCursor{
		epochDrainPlan: epochDrainPlan{TargetEra: 10, Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(3).Bytes(), VoterAmountFrozen: big.NewInt(9)},
		}},
		epochDrainProgress: epochDrainProgress{
			ScanPhase:   voterScanHead,
			ResumeVoter: identityset.Address(4).Bytes(),
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, second))

	got, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(got)
	r.Equal(voterScanHead, got.ScanPhase)
	r.Equal(identityset.Address(4).Bytes(), got.ResumeVoter,
		"second write must replace ResumeVoter, not merge")
	r.Len(got.Delegates, 1, "second write must replace, not append")
	r.Equal(identityset.Address(3).Bytes(), got.Delegates[0].CandidateIdentifier)

	// Starting a new range clears the resume point; the cleared value must
	// round-trip as empty rather than carrying the prior address forward.
	third := &epochDrainCursor{
		epochDrainPlan: epochDrainPlan{TargetEra: 10, Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(4).Bytes(), VoterAmountFrozen: big.NewInt(7)},
		}},
		epochDrainProgress: epochDrainProgress{ScanPhase: voterScanTail},
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

	cursor := &epochDrainCursor{
		epochDrainPlan: epochDrainPlan{
			TargetEra:      24,
			FreezeHeight:   900,
			SettlementSeed: settlementSeed[:],
			Delegates: []epochDrainDelegateWork{
				{
					CandidateIdentifier: identityset.Address(1).Bytes(),
					VoterAmountFrozen:   big.NewInt(100),
					TotalWeight:         big.NewInt(10),
				},
				{
					CandidateIdentifier: identityset.Address(2).Bytes(),
					VoterAmountFrozen:   big.NewInt(200),
					TotalWeight:         big.NewInt(20),
				},
			},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))

	planBefore := &epochDrainPlan{}
	_, err := p.state(ctx, sm, state.EpochDrainPlanKey, planBefore)
	r.NoError(err)
	planBytesBefore, err := planBefore.Serialize()
	r.NoError(err)

	cursor.ScanPhase = voterScanHead
	cursor.ResumeVoter = identityset.Address(7).Bytes()
	cursor.Distributed = []*big.Int{new(big.Int), big.NewInt(75)}
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
	r.Equal(voterScanHead, got.ScanPhase)
	r.Equal(identityset.Address(7).Bytes(), got.ResumeVoter)
	r.Equal(settlementStartVoter(settlementSeed[:]), settlementStartVoter(got.SettlementSeed))
	r.Zero(got.distributedAt(0).Sign())
	r.Zero(big.NewInt(75).Cmp(got.distributedAt(1)))
}

// TestEpochDrainCursor_ProgressWithoutPlanIsRefused pins the composed read:
// the running per-delegate totals are only meaningful next to the frozen work
// list they index into, so a half-written settlement is an error rather than a
// cursor with an empty delegate list (which would drain nothing while leaving
// every pool indefinitely pending).
func TestEpochDrainCursor_ProgressWithoutPlanIsRefused(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	r.NoError(p.putState(ctx, sm, state.EpochDrainCursorKey, &epochDrainProgress{}))
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
		&epochDrainProgress{Distributed: []*big.Int{big.NewInt(1), big.NewInt(2)}},
	)
	r.ErrorContains(err, "distributed totals")
}
