// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestPendingBlockRewardPool_ReadMissingIsZero — reads against an
// unpopulated pool key return zero without an error so callers can treat
// "no pool entry" and "zero pool entry" identically.
func TestPendingBlockRewardPool_ReadMissingIsZero(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	amt, err := p.readPendingBlockRewardPool(ctx, sm, identityset.Address(1).Bytes())
	r.NoError(err)
	r.NotNil(amt)
	r.Equal(0, amt.Sign(), "missing pool entry must read as zero, got %s", amt.String())
}

// TestPendingBlockRewardPool_CreditZeroIsNoop — a nil or zero amount must
// not create an entry, and must not add the delegate to the index. Callers
// pass legacy zero rewards through the same helper and rely on this
// short-circuit.
func TestPendingBlockRewardPool_CreditZeroIsNoop(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candID := identityset.Address(3).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, nil))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(0)))

	amt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
	r.NoError(err)
	r.Equal(0, amt.Sign())

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Empty(ids)
}

// TestPendingBlockRewardPool_CreditAccumulates — multiple credits to the
// same delegate accumulate arithmetically; the index gets one entry.
func TestPendingBlockRewardPool_CreditAccumulates(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candID := identityset.Address(4).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(100)))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(250)))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(1)))

	amt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
	r.NoError(err)
	r.Equal(int64(351), amt.Int64())

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Len(ids, 1)
	r.Equal(candID, ids[0])
}

// TestPendingBlockRewardPool_IndexSorted — inserting candidate IDs in
// non-sorted order must yield a bytewise-sorted index for canonical
// enumeration at epoch close.
func TestPendingBlockRewardPool_IndexSorted(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	// Choose four addresses whose byte representations don't happen to
	// already be inserted in sorted order.
	inputs := [][]byte{
		identityset.Address(9).Bytes(),
		identityset.Address(2).Bytes(),
		identityset.Address(7).Bytes(),
		identityset.Address(4).Bytes(),
	}
	for _, id := range inputs {
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, id, big.NewInt(10)))
	}

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Len(ids, 4)
	for i := 1; i < len(ids); i++ {
		r.LessOrEqual(compareBytes(ids[i-1], ids[i]), 0,
			"index not sorted at position %d: %x vs %x", i, ids[i-1], ids[i])
	}
}

// TestPendingBlockRewardPool_Delete — draining removes both the entry and
// its index membership; subsequent reads see zero and empty.
func TestPendingBlockRewardPool_Delete(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candID := identityset.Address(5).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(500)))

	r.NoError(p.deletePendingBlockRewardPool(ctx, sm, candID))

	amt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
	r.NoError(err)
	r.Equal(0, amt.Sign())

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Empty(ids)
}

// TestPendingBlockRewardPool_DeleteIdempotent — after a real credit-then-
// delete cycle, a second delete against the same ID is a no-op. This
// mirrors what happens if the epoch drain gets replayed against an
// already-cleared pool.
func TestPendingBlockRewardPool_DeleteIdempotent(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candID := identityset.Address(7).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(7)))
	r.NoError(p.deletePendingBlockRewardPool(ctx, sm, candID))
	r.NoError(p.deletePendingBlockRewardPool(ctx, sm, candID))
}

// TestPendingBlockRewardPool_IndexIsolatesEntries — after adding two
// delegates and deleting one, only the survivor remains in the index and
// its balance stays intact.
func TestPendingBlockRewardPool_IndexIsolatesEntries(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	alice := identityset.Address(11).Bytes()
	bob := identityset.Address(12).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, alice, big.NewInt(100)))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, bob, big.NewInt(200)))

	r.NoError(p.deletePendingBlockRewardPool(ctx, sm, alice))

	amt, err := p.readPendingBlockRewardPool(ctx, sm, bob)
	r.NoError(err)
	r.Equal(int64(200), amt.Int64())

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Len(ids, 1)
	r.Equal(bob, ids[0])
}

// TestCandidateIdentifierBytes — the address-parse helper mirrors the
// identifier convention used by staking.PollSnapshotFor.
func TestCandidateIdentifierBytes(t *testing.T) {
	r := require.New(t)

	addr := identityset.Address(15)
	got, err := candidateIdentifierBytes(addr.String())
	r.NoError(err)
	r.Equal(addr.Bytes(), got)

	_, err = candidateIdentifierBytes("")
	r.Error(err)

	_, err = candidateIdentifierBytes("not-a-valid-address")
	r.Error(err)
}

func TestCandidateIdentifier(t *testing.T) {
	identity := identityset.Address(1).String()
	operator := identityset.Address(2).String()
	require.Equal(t, identity, candidateIdentifier(&state.Candidate{Identity: identity, Address: operator}))
	require.Equal(t, operator, candidateIdentifier(&state.Candidate{Address: operator}))
	require.Empty(t, candidateIdentifier(nil))
}

// TestRefundPendingBlockRewardPool — the orphan-fallback path increments
// unclaimedBalance but must not touch totalBalance, so the fund invariant
// unclaimed + Σ(per-address) + pool = totalBalance stays intact.
func TestRefundPendingBlockRewardPool(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	// Seed the fund with a known state so we can verify the deltas.
	seed := &fund{
		totalBalance:     big.NewInt(1_000),
		unclaimedBalance: big.NewInt(400),
	}
	r.NoError(p.putState(ctx, sm, _fundKey, seed))

	r.NoError(p.refundPendingBlockRewardPool(ctx, sm, big.NewInt(150)))

	got := &fund{}
	_, err := p.state(ctx, sm, _fundKey, got)
	r.NoError(err)
	r.Equal(int64(1_000), got.totalBalance.Int64(), "totalBalance must not move on refund")
	r.Equal(int64(550), got.unclaimedBalance.Int64(), "unclaimedBalance must grow by refund amount")

	// Nil or zero amount is a no-op.
	r.NoError(p.refundPendingBlockRewardPool(ctx, sm, nil))
	r.NoError(p.refundPendingBlockRewardPool(ctx, sm, big.NewInt(0)))
	_, err = p.state(ctx, sm, _fundKey, got)
	r.NoError(err)
	r.Equal(int64(550), got.unclaimedBalance.Int64())
}

// TestDrainOrphans_NoEntriesIsNoop — the sweep exits cleanly with no logs
// when the index is empty.
func TestDrainOrphans_NoEntriesIsNoop(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	txLogs, logs, err := p.drainPendingBlockRewardOrphans(ctx, sm, nil, 100, hash.ZeroHash256)
	r.NoError(err)
	r.Nil(txLogs)
	r.Nil(logs)
}

// TestDrainOrphans_RefundsWhenCandidateGone — an orphan pool ID whose
// staking.Candidate is not present in the base view (delegate fully
// unregistered) must land its balance back into fund.unclaimedBalance,
// emit a BLOCK_REWARD log with an empty addr, and clear the pool entry.
func TestDrainOrphans_RefundsWhenCandidateGone(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	// Seed a fund entry so refundPendingBlockRewardPool can read it.
	r.NoError(p.putState(ctx, sm, _fundKey, &fund{
		totalBalance:     big.NewInt(10_000),
		unclaimedBalance: big.NewInt(2_000),
	}))

	// Credit a pool entry for a delegate that is not in the staking view.
	// newVoterRewardCtx registers a fresh staking protocol with no
	// candidates, so GetCandidateByOwner returns nil and the fallback
	// refund path fires.
	ghost := identityset.Address(20).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, ghost, big.NewInt(250)))

	_, logs, err := p.drainPendingBlockRewardOrphans(ctx, sm, nil, 100, hash.ZeroHash256)
	r.NoError(err)
	r.Len(logs, 1)

	// Pool entry is drained, index is empty.
	amt, err := p.readPendingBlockRewardPool(ctx, sm, ghost)
	r.NoError(err)
	r.Equal(0, amt.Sign())
	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Empty(ids)

	// unclaimedBalance grew by the refund; totalBalance unchanged.
	f := &fund{}
	_, err = p.state(ctx, sm, _fundKey, f)
	r.NoError(err)
	r.Equal(int64(10_000), f.totalBalance.Int64())
	r.Equal(int64(2_250), f.unclaimedBalance.Int64())
}

func TestDrainOrphans_UsesOwnerInsteadOfLegacyRewardAddress(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	candID := identityset.Address(20)
	owner := identityset.Address(21)
	legacyReward := identityset.Address(22)

	r.NoError(p.putState(ctx, sm, _fundKey, &fund{
		totalBalance: big.NewInt(1_000), unclaimedBalance: big.NewInt(750),
	}))
	r.NoError(staking.TestOnlyPutCandidateRewardAddress(sm, candID, owner, legacyReward, false))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID.Bytes(), big.NewInt(250)))

	txLogs, logs, err := p.drainPendingBlockRewardOrphans(ctx, sm, nil, 100, hash.ZeroHash256)
	r.NoError(err)
	r.Len(logs, 1)
	r.Len(txLogs, 1)
	ownerBalance, _, err := p.UnclaimedBalance(ctx, sm, owner)
	r.NoError(err)
	r.Zero(ownerBalance.Sign())
	legacyBalance, _, err := p.UnclaimedBalance(ctx, sm, legacyReward)
	r.NoError(err)
	r.Zero(legacyBalance.Sign())
}

// TestDrainOrphans_SkipsVisited — pool entries whose ID string is in the
// visited map (already handled by the per-candidate loop) are left alone.
func TestDrainOrphans_SkipsVisited(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	// Seed a fund entry so any accidental refund would be observable.
	r.NoError(p.putState(ctx, sm, _fundKey, &fund{
		totalBalance:     big.NewInt(10_000),
		unclaimedBalance: big.NewInt(2_000),
	}))

	visited := identityset.Address(21).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, visited, big.NewInt(999)))

	_, logs, err := p.drainPendingBlockRewardOrphans(
		ctx, sm, map[string]bool{string(visited): true}, 100, hash.ZeroHash256,
	)
	r.NoError(err)
	r.Empty(logs, "visited pool IDs must be skipped by the orphan sweep")

	amt, err := p.readPendingBlockRewardPool(ctx, sm, visited)
	r.NoError(err)
	r.Equal(int64(999), amt.Int64(), "visited pool balance must survive the sweep")

	f := &fund{}
	_, err = p.state(ctx, sm, _fundKey, f)
	r.NoError(err)
	r.Equal(int64(2_000), f.unclaimedBalance.Int64(),
		"unclaimedBalance must not move for visited entries")
}

// TestPendingBlockRewardPool_DecrementPartial — subtracting less than the
// current balance leaves the entry with a positive residual and preserves
// the delegate's index membership so a future decrement can still find it.
func TestPendingBlockRewardPool_DecrementPartial(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candID := identityset.Address(6).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(500)))

	r.NoError(p.decrementPendingBlockRewardPool(ctx, sm, candID, big.NewInt(200)))

	amt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
	r.NoError(err)
	r.Equal(int64(300), amt.Int64(), "residual balance must equal credit minus decrement")

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Len(ids, 1)
	r.Equal(candID, ids[0], "index entry must survive a partial decrement")
}

// TestPendingBlockRewardPool_DecrementExactDeletes — decrementing by the
// exact remaining balance zeroes the entry, which the helper must treat as
// full drain: delete the entry and remove the delegate from the index.
func TestPendingBlockRewardPool_DecrementExactDeletes(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candID := identityset.Address(8).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(500)))

	r.NoError(p.decrementPendingBlockRewardPool(ctx, sm, candID, big.NewInt(500)))

	amt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
	r.NoError(err)
	r.Equal(0, amt.Sign())

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Empty(ids, "index membership must clear when the entry is exact-drained")
}

// TestPendingBlockRewardPool_DecrementClampsToBalance — a decrement larger
// than the current balance clamps to the balance (no negative amount ever
// persists) and treats the outcome as a full drain: entry + index membership
// gone. This is the guard against arithmetic slippage between the frozen
// cursor amount and the pool's live balance.
func TestPendingBlockRewardPool_DecrementClampsToBalance(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candID := identityset.Address(10).Bytes()
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(100)))

	r.NoError(p.decrementPendingBlockRewardPool(ctx, sm, candID, big.NewInt(9_999)))

	amt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
	r.NoError(err)
	r.Equal(0, amt.Sign(), "over-decrement must clamp to zero, never persist negative")

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Empty(ids)
}

// TestPendingBlockRewardPool_DecrementMissingIsNoop — decrementing against
// an unpopulated key returns cleanly. Nil / non-positive amounts short-
// circuit before touching state. This mirrors what happens when a chunk
// runs against a delegate whose block-side pool was never credited.
func TestPendingBlockRewardPool_DecrementMissingIsNoop(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candID := identityset.Address(13).Bytes()
	r.NoError(p.decrementPendingBlockRewardPool(ctx, sm, candID, big.NewInt(100)))
	r.NoError(p.decrementPendingBlockRewardPool(ctx, sm, candID, nil))
	r.NoError(p.decrementPendingBlockRewardPool(ctx, sm, candID, big.NewInt(0)))
	r.NoError(p.decrementPendingBlockRewardPool(ctx, sm, candID, big.NewInt(-5)))

	amt, err := p.readPendingBlockRewardPool(ctx, sm, candID)
	r.NoError(err)
	r.Equal(0, amt.Sign())

	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sm)
	r.NoError(err)
	r.Empty(ids)
}

// compareBytes is bytes.Compare wrapped for readability at call sites.
func compareBytes(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// _ suppresses unused-import warnings when tests are trimmed; the address
// package is used indirectly via identityset.Address(N).Bytes().
var _ = address.ZeroAddress
