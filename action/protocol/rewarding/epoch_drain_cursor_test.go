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
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestEpochDrainCursor_RoundTrip — Serialize→Deserialize preserves
// every field, including the frozen delegate work list with big.Int
// pool balances and the resume index.
func TestEpochDrainCursor_RoundTrip(t *testing.T) {
	r := require.New(t)

	in := epochDrainCursor{
		TargetEra:          42,
		DelegateIndex:      7,
		VoterIndex:         123,
		SettlementSeed:     []byte{7, 8, 9},
		DelegateStartIndex: 1,
		Delegates: []epochDrainDelegateWork{
			{
				CandidateIdentifier: identityset.Address(1).Bytes(),
				VoterAmountFrozen:   big.NewInt(1_000),
				RewardAddress:       identityset.Address(11).Bytes(),
				EpochCommission:     big.NewInt(300),
				TotalWeight:         big.NewInt(400),
				SnapshotHash:        []byte{1, 2, 3},
				LastWeightedIndex:   2,
				HasWeightedEntries:  true,
				VoterStartIndex:     17,
			},
			{
				CandidateIdentifier: identityset.Address(2).Bytes(),
				VoterAmountFrozen:   big.NewInt(2_500_000),
				RewardAddress:       identityset.Address(12).Bytes(),
				EpochCommission:     big.NewInt(750_000),
				TotalWeight:         big.NewInt(1_000_000),
				SnapshotHash:        []byte{4, 5, 6},
				LastWeightedIndex:   9,
				HasWeightedEntries:  true,
				VoterStartIndex:     31,
			},
		},
	}

	raw, err := in.Serialize()
	r.NoError(err)
	r.NotEmpty(raw)

	var out epochDrainCursor
	r.NoError(out.Deserialize(raw))
	r.Equal(in.TargetEra, out.TargetEra)
	r.Equal(in.DelegateIndex, out.DelegateIndex)
	r.Equal(in.VoterIndex, out.VoterIndex)
	r.Equal(in.SettlementSeed, out.SettlementSeed)
	r.Equal(in.DelegateStartIndex, out.DelegateStartIndex)
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
		r.Equal(in.Delegates[i].LastWeightedIndex, out.Delegates[i].LastWeightedIndex)
		r.Equal(in.Delegates[i].HasWeightedEntries, out.Delegates[i].HasWeightedEntries)
		r.Equal(in.Delegates[i].VoterStartIndex, out.Delegates[i].VoterStartIndex)
	}
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

func TestRotateDelegateWork(t *testing.T) {
	delegates := make([]epochDrainDelegateWork, 4)
	for i := range delegates {
		delegates[i].CandidateIdentifier = []byte{byte(i)}
	}

	r := require.New(t)
	r.Equal([]byte{2}, rotateDelegateWork(delegates, 2)[0].CandidateIdentifier)
	r.Equal([]byte{3}, rotateDelegateWork(delegates, 6)[1].CandidateIdentifier)
	r.Equal([]byte{0}, rotateDelegateWork(delegates, 0)[0].CandidateIdentifier)
	r.Empty(rotateDelegateWork(nil, 10))
	// Rotation must not mutate the canonical input list.
	r.Equal([]byte{0}, delegates[0].CandidateIdentifier)
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
	r.Zero(cursor.DelegateStartIndex)
	r.Zero(cursor.Delegates[0].VoterStartIndex)
}

// TestEpochDrainCursor_EmptyDelegates — a cursor with no delegate work
// (era boundary hit but pool was empty) round-trips as an empty slice,
// not a nil.
func TestEpochDrainCursor_EmptyDelegates(t *testing.T) {
	r := require.New(t)

	in := epochDrainCursor{TargetEra: 1, DelegateIndex: 0}
	raw, err := in.Serialize()
	r.NoError(err)

	var out epochDrainCursor
	r.NoError(out.Deserialize(raw))
	r.Equal(uint64(1), out.TargetEra)
	r.Equal(uint32(0), out.DelegateIndex)
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
		TargetEra:     99,
		DelegateIndex: 3,
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
	r.Equal(in.DelegateIndex, got.DelegateIndex)
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
		TargetEra:     10,
		DelegateIndex: 0,
		Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(1).Bytes(), VoterAmountFrozen: big.NewInt(1)},
			{CandidateIdentifier: identityset.Address(2).Bytes(), VoterAmountFrozen: big.NewInt(2)},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, first))

	second := &epochDrainCursor{
		TargetEra:     10,
		DelegateIndex: 1,
		VoterIndex:    88,
		Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(3).Bytes(), VoterAmountFrozen: big.NewInt(9)},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, second))

	got, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(got)
	r.Equal(uint32(1), got.DelegateIndex)
	r.Equal(uint32(88), got.VoterIndex, "second write must replace VoterIndex, not merge")
	r.Len(got.Delegates, 1, "second write must replace, not append")
	r.Equal(identityset.Address(3).Bytes(), got.Delegates[0].CandidateIdentifier)

	// Overwrite with a fresh delegate (VoterIndex must reset to 0).
	third := &epochDrainCursor{
		TargetEra:     10,
		DelegateIndex: 2,
		VoterIndex:    0,
		Delegates: []epochDrainDelegateWork{
			{CandidateIdentifier: identityset.Address(4).Bytes(), VoterAmountFrozen: big.NewInt(7)},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, third))
	got, err = p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(got)
	r.Equal(uint32(0), got.VoterIndex, "zero VoterIndex must round-trip as 0, not carry the prior value")
}
