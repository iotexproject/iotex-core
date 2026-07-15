// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestEpochDrainCursor_ReadMissingIsNil — an absent cursor reads as
// (nil, nil). Callers use presence as the "drain in progress" signal, so
// a missing key must not surface as an error.
func TestEpochDrainCursor_ReadMissingIsNil(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	c, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.Nil(c)
}

// TestEpochDrainCursor_HeaderRoundTrip — the minimum-payload cursor
// (position fields only, no frozen lists) survives serialisation.
func TestEpochDrainCursor_HeaderRoundTrip(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	in := &epochDrainCursor{TargetEpoch: 42, DelegateIndex: 17}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, in))

	out, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(out)
	r.Equal(uint64(42), out.TargetEpoch)
	r.Equal(uint32(17), out.DelegateIndex)
	r.Empty(out.Delegates)
	r.Empty(out.FoundationBonus)
	r.Empty(out.Orphans)
}

// TestEpochDrainCursor_FrozenListsRoundTrip — the full Phase A payload
// (delegate work + foundation bonus + orphans) survives storage with
// exact per-field equality including has_reward_address = false for a
// delegate with no reward address.
func TestEpochDrainCursor_FrozenListsRoundTrip(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	rewardAddr := identityset.Address(1).Bytes()
	candID := identityset.Address(2).Bytes()
	targetAddr := identityset.Address(3).Bytes()

	in := &epochDrainCursor{
		TargetEpoch:   99,
		DelegateIndex: 2,
		Delegates: []epochDrainDelegateWork{
			{
				CandidateAddress: identityset.Address(2).String(),
				RewardAddress:    rewardAddr,
				HasRewardAddress: true,
				EpochAmount:      big.NewInt(1_000_000),
				PoolAmountFrozen: big.NewInt(250_000),
			},
			{
				// Delegate missing a reward address: has_reward_address=false
				// must round-trip cleanly, and the zero big.Int in the pool
				// slot must decode as sign 0 (not nil).
				CandidateAddress: identityset.Address(4).String(),
				HasRewardAddress: false,
				EpochAmount:      big.NewInt(500),
				PoolAmountFrozen: big.NewInt(0),
			},
		},
		FoundationBonus: []epochDrainFoundationBonusWork{{
			RewardAddressStr: identityset.Address(1).String(),
			RewardAddress:    rewardAddr,
			Amount:           big.NewInt(80),
		}},
		Orphans: []epochDrainOrphanWork{{
			CandidateIdentifier: candID,
			PoolAmountFrozen:    big.NewInt(77),
			TargetAddress:       targetAddr,
			TargetAddressStr:    identityset.Address(3).String(),
		}},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, in))

	out, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(out)
	r.Equal(uint64(99), out.TargetEpoch)
	r.Equal(uint32(2), out.DelegateIndex)

	r.Len(out.Delegates, 2)
	r.Equal(in.Delegates[0].CandidateAddress, out.Delegates[0].CandidateAddress)
	r.Equal(rewardAddr, out.Delegates[0].RewardAddress)
	r.True(out.Delegates[0].HasRewardAddress)
	r.Equal(int64(1_000_000), out.Delegates[0].EpochAmount.Int64())
	r.Equal(int64(250_000), out.Delegates[0].PoolAmountFrozen.Int64())

	r.False(out.Delegates[1].HasRewardAddress)
	r.Empty(out.Delegates[1].RewardAddress)
	r.Equal(int64(500), out.Delegates[1].EpochAmount.Int64())
	r.Equal(0, out.Delegates[1].PoolAmountFrozen.Sign())

	r.Len(out.FoundationBonus, 1)
	r.Equal(int64(80), out.FoundationBonus[0].Amount.Int64())
	r.Equal(identityset.Address(1).String(), out.FoundationBonus[0].RewardAddressStr)

	r.Len(out.Orphans, 1)
	r.Equal(candID, out.Orphans[0].CandidateIdentifier)
	r.Equal(int64(77), out.Orphans[0].PoolAmountFrozen.Int64())
	r.Equal(targetAddr, out.Orphans[0].TargetAddress)
}

// TestEpochDrainCursor_AdvanceOverwrite — write a Phase A cursor,
// advance DelegateIndex, verify the frozen lists are preserved
// through the intermediate rewrite.
func TestEpochDrainCursor_AdvanceOverwrite(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	c := &epochDrainCursor{
		TargetEpoch:   7,
		DelegateIndex: 0,
		Delegates: []epochDrainDelegateWork{
			{CandidateAddress: identityset.Address(1).String(), EpochAmount: big.NewInt(1)},
			{CandidateAddress: identityset.Address(2).String(), EpochAmount: big.NewInt(2)},
			{CandidateAddress: identityset.Address(3).String(), EpochAmount: big.NewInt(3)},
		},
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, c))

	c.DelegateIndex = 2
	r.NoError(p.writeEpochDrainCursor(ctx, sm, c))

	out, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.Equal(uint32(2), out.DelegateIndex)
	r.Len(out.Delegates, 3)
	r.Equal(identityset.Address(3).String(), out.Delegates[2].CandidateAddress)
}

// TestEpochDrainCursor_DeleteAfterWriteIdempotent — after Coda deletes
// the cursor, a second delete must not error. Mirrors deleteState's
// swallow of state.ErrStateNotExist.
func TestEpochDrainCursor_DeleteAfterWriteIdempotent(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{TargetEpoch: 1}))
	r.NoError(p.deleteEpochDrainCursor(ctx, sm))
	r.NoError(p.deleteEpochDrainCursor(ctx, sm))

	c, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.Nil(c)
}
