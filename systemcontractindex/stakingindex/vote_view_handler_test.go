// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package stakingindex

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// recordingSink captures every Apply call so tests can assert exactly which
// (cand, voter, Δweight) tuples the handler emits. Order matters — vote_view
// tests rely on the order in which PutBucket emits deltas.
type recordingSink struct {
	calls []struct {
		cand  address.Address
		voter address.Address
		delta *big.Int
	}
}

func (r *recordingSink) Apply(cand, voter address.Address, delta *big.Int) {
	r.calls = append(r.calls, struct {
		cand  address.Address
		voter address.Address
		delta *big.Int
	}{cand, voter, new(big.Int).Set(delta)})
}

// emptyBucketReader returns ErrBucketNotExist for every DeductBucket call —
// the vote-view handler treats "no prior bucket" as a fresh Put, which is
// exactly what we want for the initial-Put subtests.
type emptyBucketReader struct{}

func (emptyBucketReader) DeductBucket(_ address.Address, _ uint64) (*contractstaking.Bucket, error) {
	return nil, errors.Wrap(contractstaking.ErrBucketNotExist, "bucket not exist")
}

// unitWeight returns 1 IOTX unit for any bucket — makes deltas easy to
// eyeball. The handler only sees the return; how it's computed is irrelevant
// for these tests.
func unitWeight(_ *contractstaking.Bucket) *big.Int { return big.NewInt(1000) }

// scaledWeight returns StakedAmount as the weight — needed for the
// weight-change subtest where the delta is (new - old) staked amount.
func scaledWeight(b *contractstaking.Bucket) *big.Int {
	if b == nil || b.StakedAmount == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(b.StakedAmount)
}

// activeBucket builds a plain, non-muted, non-unstaked bucket at the given
// (cand, owner, amount). All time fields are set to the sentinel so
// `Muted || UnstakedAt < maxStakingNumber` stays false.
func activeBucket(cand, owner address.Address, amount int64) *contractstaking.Bucket {
	return &contractstaking.Bucket{
		Candidate:      cand,
		Owner:          owner,
		StakedAmount:   big.NewInt(amount),
		StakedDuration: 30,
		CreatedAt:      1,
		UnlockedAt:     maxStakingNumber,
		UnstakedAt:     maxStakingNumber,
	}
}

// New-bucket path: PutBucket with no prior bucket must emit a single +newW
// delta at (bkt.Candidate, bkt.Owner).
func TestVoteViewEventHandler_PutBucketFreshEmitsPositiveDelta(t *testing.T) {
	require := require.New(t)
	sink := &recordingSink{}
	store := newBucketStore(emptyBucketReader{})
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), unitWeight, sink)
	require.NoError(err)

	contract := identityset.Address(0)
	cand := identityset.Address(1)
	voter := identityset.Address(2)
	bkt := activeBucket(cand, voter, 1000)

	require.NoError(h.PutBucket(contract, 42, bkt))
	require.Len(sink.calls, 1)
	require.Equal(cand.String(), sink.calls[0].cand.String())
	require.Equal(voter.String(), sink.calls[0].voter.String())
	require.Zero(big.NewInt(1000).Cmp(sink.calls[0].delta))
}

// Same-candidate + same-owner weight change: PutBucket with an existing
// bucket for (cand, owner) at a different weight must emit exactly one
// delta = newW - oldW at (cand, owner).
func TestVoteViewEventHandler_PutBucketSameCandSameOwnerEmitsDiff(t *testing.T) {
	require := require.New(t)
	sink := &recordingSink{}
	store := newBucketStore(emptyBucketReader{})
	// scaledWeight so the weight change equals the amount change.
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), scaledWeight, sink)
	require.NoError(err)

	contract := identityset.Address(0)
	cand := identityset.Address(1)
	voter := identityset.Address(2)

	// Seed: put original bucket (fresh — sink call 0 records +1000).
	orig := activeBucket(cand, voter, 1000)
	require.NoError(h.PutBucket(contract, 7, orig))
	require.Len(sink.calls, 1)

	// Replace with same (cand, voter) but larger amount → Δ = +500.
	upd := activeBucket(cand, voter, 1500)
	require.NoError(h.PutBucket(contract, 7, upd))
	require.Len(sink.calls, 2)
	require.Equal(cand.String(), sink.calls[1].cand.String())
	require.Equal(voter.String(), sink.calls[1].voter.String())
	require.Zero(big.NewInt(500).Cmp(sink.calls[1].delta))
}

// Same-candidate + owner change (bucket transferred): PutBucket must emit
// -oldW at (cand, oldOwner) and +newW at (cand, newOwner).
func TestVoteViewEventHandler_PutBucketOwnerChangeSplits(t *testing.T) {
	require := require.New(t)
	sink := &recordingSink{}
	store := newBucketStore(emptyBucketReader{})
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), unitWeight, sink)
	require.NoError(err)

	contract := identityset.Address(0)
	cand := identityset.Address(1)
	oldOwner := identityset.Address(2)
	newOwner := identityset.Address(3)

	// Seed at old owner.
	require.NoError(h.PutBucket(contract, 7, activeBucket(cand, oldOwner, 1000)))
	require.Len(sink.calls, 1)

	// Transfer to new owner (still under cand).
	require.NoError(h.PutBucket(contract, 7, activeBucket(cand, newOwner, 1000)))
	require.Len(sink.calls, 3, "owner change must emit exactly two deltas")

	// Order per handler impl: -oldW at old owner, then +newW at new owner.
	require.Equal(cand.String(), sink.calls[1].cand.String())
	require.Equal(oldOwner.String(), sink.calls[1].voter.String())
	require.Zero(big.NewInt(-1000).Cmp(sink.calls[1].delta), "old-owner side must be negative")

	require.Equal(cand.String(), sink.calls[2].cand.String())
	require.Equal(newOwner.String(), sink.calls[2].voter.String())
	require.Zero(big.NewInt(1000).Cmp(sink.calls[2].delta), "new-owner side must be positive")
}

// Candidate change (delegate change): PutBucket must emit -oldW at
// (oldCand, oldOwner) and +newW at (newCand, newOwner). This is the case
// where a voter re-delegates their bucket to a different candidate.
func TestVoteViewEventHandler_PutBucketCandidateChangeSplits(t *testing.T) {
	require := require.New(t)
	sink := &recordingSink{}
	store := newBucketStore(emptyBucketReader{})
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), unitWeight, sink)
	require.NoError(err)

	contract := identityset.Address(0)
	oldCand := identityset.Address(1)
	newCand := identityset.Address(2)
	voter := identityset.Address(3)

	// Seed under oldCand.
	require.NoError(h.PutBucket(contract, 7, activeBucket(oldCand, voter, 1000)))
	require.Len(sink.calls, 1)

	// Re-delegate to newCand, same owner.
	require.NoError(h.PutBucket(contract, 7, activeBucket(newCand, voter, 1000)))
	require.Len(sink.calls, 3)

	require.Equal(oldCand.String(), sink.calls[1].cand.String())
	require.Equal(voter.String(), sink.calls[1].voter.String())
	require.Zero(big.NewInt(-1000).Cmp(sink.calls[1].delta))

	require.Equal(newCand.String(), sink.calls[2].cand.String())
	require.Equal(voter.String(), sink.calls[2].voter.String())
	require.Zero(big.NewInt(1000).Cmp(sink.calls[2].delta))
}

// DeleteBucket must emit -oldW at (org.Candidate, org.Owner).
func TestVoteViewEventHandler_DeleteBucketEmitsNegative(t *testing.T) {
	require := require.New(t)
	sink := &recordingSink{}
	store := newBucketStore(emptyBucketReader{})
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), unitWeight, sink)
	require.NoError(err)

	contract := identityset.Address(0)
	cand := identityset.Address(1)
	voter := identityset.Address(2)

	require.NoError(h.PutBucket(contract, 7, activeBucket(cand, voter, 1000)))
	require.Len(sink.calls, 1)

	require.NoError(h.DeleteBucket(contract, 7))
	require.Len(sink.calls, 2)
	require.Equal(cand.String(), sink.calls[1].cand.String())
	require.Equal(voter.String(), sink.calls[1].voter.String())
	require.Zero(big.NewInt(-1000).Cmp(sink.calls[1].delta))
}

// Deleting a non-existent bucket must be a no-op — no sink call and no
// error. This matches the ErrBucketNotExist swallow in the handler.
func TestVoteViewEventHandler_DeleteMissingBucketIsNoOp(t *testing.T) {
	require := require.New(t)
	sink := &recordingSink{}
	store := newBucketStore(emptyBucketReader{})
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), unitWeight, sink)
	require.NoError(err)

	require.NoError(h.DeleteBucket(identityset.Address(0), 999))
	require.Empty(sink.calls)
}

// A muted or already-unstaked bucket contributes zero weight; the sink
// must not receive a spurious zero-delta call (emitVoterDelta filters
// Sign()==0).
func TestVoteViewEventHandler_MutedBucketEmitsNoDelta(t *testing.T) {
	require := require.New(t)
	sink := &recordingSink{}
	store := newBucketStore(emptyBucketReader{})
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), unitWeight, sink)
	require.NoError(err)

	contract := identityset.Address(0)
	cand := identityset.Address(1)
	voter := identityset.Address(2)

	muted := activeBucket(cand, voter, 1000)
	muted.Muted = true
	require.NoError(h.PutBucket(contract, 7, muted))
	require.Empty(sink.calls, "muted bucket must not emit a delta")
}

// A nil sink must be tolerated: emitVoterDelta nil-guards it, so PutBucket
// and DeleteBucket still succeed on their non-sink work (updating the
// candidate votes and bucket store).
func TestVoteViewEventHandler_NilSinkTolerated(t *testing.T) {
	require := require.New(t)
	store := newBucketStore(emptyBucketReader{})
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), unitWeight, nil)
	require.NoError(err)

	contract := identityset.Address(0)
	cand := identityset.Address(1)
	voter := identityset.Address(2)

	require.NoError(h.PutBucket(contract, 7, activeBucket(cand, voter, 1000)))
	require.NoError(h.DeleteBucket(contract, 7))
}

// End-to-end integration with a staking.VoterWeightView as the sink:
// putting a bucket, transferring it, then deleting it must leave the view
// with zero entries — same invariant as running the same events through
// the native handler path.
func TestVoteViewEventHandler_SinkIntegrationBalancesToZero(t *testing.T) {
	require := require.New(t)
	view := staking.NewVoterWeightView()
	store := newBucketStore(emptyBucketReader{})
	h, err := newVoteViewEventHandler(store, newCandidateVotesWithBuffer(newCandidateVotes()), unitWeight, view)
	require.NoError(err)

	contract := identityset.Address(0)
	cand := identityset.Address(1)
	voterA := identityset.Address(2)
	voterB := identityset.Address(3)

	// Put fresh bucket → +1000 for voterA.
	require.NoError(h.PutBucket(contract, 7, activeBucket(cand, voterA, 1000)))
	// Transfer to voterB → -1000 at voterA, +1000 at voterB.
	require.NoError(h.PutBucket(contract, 7, activeBucket(cand, voterB, 1000)))
	// Delete → -1000 at voterB.
	require.NoError(h.DeleteBucket(contract, 7))

	// After balanced ops the map still holds zero-weight entries for both
	// voters until Commit prunes them. Weights() filters zero entries out
	// so the observable result is empty either way.
	require.Empty(view.Weights(cand), "sink-fed view must be empty after balanced create/transfer/delete")
	view.Commit()
	require.Nil(view.Weights(cand), "after Commit, zero entries must be pruned entirely")
}
