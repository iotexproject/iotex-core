// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// Test key layout: _voterWeightSnap tag + 20-byte identity = 21 bytes total.
func TestVoterWeightSnapKeyLayout(t *testing.T) {
	require := require.New(t)
	candID := identityset.Address(3)

	key := voterWeightSnapKey(candID)
	require.Len(key, 21)
	require.Equal(_voterWeightSnap, key[0])
	require.Equal(candID.Bytes(), key[1:])
}

// Encoding must be deterministic: same input → byte-identical output.
// Byte-equality skip in writeVoterWeightSnapshot depends on this.
func TestEncodeVoterWeightSnapshotDeterministic(t *testing.T) {
	require := require.New(t)

	// Two logically identical inputs, both pre-sorted by voter address bytes.
	// identityset.Address(3), (4), (5) happen to sort ascending by raw bytes:
	// 0ddf... < 2da1... < 4985... .
	entries := []snapshotEntry{
		{voter: identityset.Address(3), weight: big.NewInt(100)},
		{voter: identityset.Address(4), weight: big.NewInt(200)},
		{voter: identityset.Address(5), weight: big.NewInt(999)},
	}
	// Sanity-check that the caller sorted them.
	for i := 1; i < len(entries); i++ {
		require.Less(bytes.Compare(entries[i-1].voter.Bytes(), entries[i].voter.Bytes()), 0)
	}

	_, blob1, err := encodeVoterWeightSnapshot(entries)
	require.NoError(err)
	require.NotEmpty(blob1)

	// Rebuild an equivalent slice (fresh pointers, same values) to make sure
	// determinism doesn't rely on pointer identity.
	entries2 := []snapshotEntry{
		{voter: identityset.Address(3), weight: new(big.Int).SetInt64(100)},
		{voter: identityset.Address(4), weight: new(big.Int).SetInt64(200)},
		{voter: identityset.Address(5), weight: new(big.Int).SetInt64(999)},
	}
	_, blob2, err := encodeVoterWeightSnapshot(entries2)
	require.NoError(err)
	require.Equal(blob1, blob2, "identical logical state must produce byte-identical blobs")
}

// Empty entries → (nil, nil, nil). Writer interprets this as "delete blob."
func TestEncodeVoterWeightSnapshotEmpty(t *testing.T) {
	require := require.New(t)

	pb, blob, err := encodeVoterWeightSnapshot(nil)
	require.NoError(err)
	require.Nil(pb)
	require.Nil(blob)

	pb, blob, err = encodeVoterWeightSnapshot([]snapshotEntry{})
	require.NoError(err)
	require.Nil(pb)
	require.Nil(blob)
}

// Roundtrip: encode → decode reproduces the input.
func TestVoterWeightSnapshotRoundtrip(t *testing.T) {
	require := require.New(t)

	entries := []snapshotEntry{
		{voter: identityset.Address(1), weight: big.NewInt(1)},
		{voter: identityset.Address(2), weight: big.NewInt(42)},
		{voter: identityset.Address(3), weight: new(big.Int).Lsh(big.NewInt(1), 128)}, // big number
	}
	pb, _, err := encodeVoterWeightSnapshot(entries)
	require.NoError(err)

	got, err := decodeVoterWeightSnapshot(pb)
	require.NoError(err)
	require.Len(got, 3)
	for i, e := range entries {
		require.True(bytes.Equal(got[i].Voter.Bytes(), e.voter.Bytes()),
			"voter mismatch at index %d", i)
		require.Zero(got[i].Weight.Cmp(e.weight), "weight mismatch at index %d", i)
	}
}

// Decode nil / empty proto → (nil, nil), never a spurious error.
func TestDecodeVoterWeightSnapshotEmpty(t *testing.T) {
	require := require.New(t)

	got, err := decodeVoterWeightSnapshot(nil)
	require.NoError(err)
	require.Nil(got)
}

// Reader roundtrip through the state manager: write → read gives the same
// entries. Also validates non-existent snapshot returns (nil, nil).
func TestVoterWeightsFromSnapshotRoundtrip(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	candID := identityset.Address(7)

	// Non-existent → (nil, nil).
	got, err := VoterWeightsFromSnapshot(sm, candID)
	require.NoError(err)
	require.Nil(got)

	entries := []snapshotEntry{
		{voter: identityset.Address(1), weight: big.NewInt(10)},
		{voter: identityset.Address(4), weight: big.NewInt(20)},
	}
	require.NoError(writeVoterWeightSnapshot(sm, candID, entries))

	got, err = VoterWeightsFromSnapshot(sm, candID)
	require.NoError(err)
	require.Len(got, 2)
}

// Byte-equality skip: writing the same entries twice must not touch state
// on the second call. We assert this by comparing the raw blob bytes before
// and after the second write.
func TestWriteVoterWeightSnapshotSkipsUnchanged(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	candID := identityset.Address(9)
	entries := []snapshotEntry{
		{voter: identityset.Address(1), weight: big.NewInt(10)},
		{voter: identityset.Address(2), weight: big.NewInt(20)},
	}
	require.NoError(writeVoterWeightSnapshot(sm, candID, entries))

	before, err := readSnapshotBlobBytes(sm, voterWeightSnapKey(candID))
	require.NoError(err)

	// Second write with same logical entries — should skip.
	require.NoError(writeVoterWeightSnapshot(sm, candID, entries))
	after, err := readSnapshotBlobBytes(sm, voterWeightSnapKey(candID))
	require.NoError(err)

	require.Equal(before, after)
}

// Empty entries after a non-empty write → the blob is deleted, subsequent
// reads return (nil, nil).
func TestWriteVoterWeightSnapshotDeletesOnEmpty(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	candID := identityset.Address(11)
	entries := []snapshotEntry{
		{voter: identityset.Address(3), weight: big.NewInt(5)},
	}
	require.NoError(writeVoterWeightSnapshot(sm, candID, entries))

	// Confirm blob exists.
	got, err := VoterWeightsFromSnapshot(sm, candID)
	require.NoError(err)
	require.Len(got, 1)

	// Now overwrite with empty → should delete.
	require.NoError(writeVoterWeightSnapshot(sm, candID, nil))
	got, err = VoterWeightsFromSnapshot(sm, candID)
	require.NoError(err)
	require.Nil(got)

	// And underlying State call returns ErrStateNotExist.
	blob := &voterWeightSnapshotBlob{}
	_, err = sm.State(blob,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(voterWeightSnapKey(candID)),
	)
	require.True(errors.Is(err, state.ErrStateNotExist) || errors.Cause(err) == state.ErrStateNotExist,
		"expected ErrStateNotExist after delete, got %v", err)
}

// End-to-end buildVoterWeightBaseFromState: seed a candidate with three
// native buckets (one self-stake, one unstaked, one normal voter) and one
// contract bucket from a mock indexer. Assert only the voter buckets show
// up in the built view, weights are aggregated per voter, and the output
// is sorted by voter address bytes. Same shape as the old scan-based path
// this replaces — the initial view build must agree with what per-handler
// deltas would have produced from an empty view.
func TestBuildVoterWeightBaseFromState(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	csr := newCandidateStateReader(sm)

	candOwner := identityset.Address(1)
	voterA := identityset.Address(2)
	voterB := identityset.Address(3)
	otherCand := identityset.Address(4)

	cand := &Candidate{
		Owner:              candOwner,
		Operator:           candOwner,
		Reward:             candOwner,
		Identifier:         candOwner,
		Name:               "cand",
		Votes:              big.NewInt(0),
		SelfStakeBucketIdx: 0, // set after we know the self-stake bucket index
		SelfStake:          big.NewInt(0),
	}

	// Bucket 0: self-stake by candOwner → excluded from voter weights.
	selfBkt := NewVoteBucket(candOwner, candOwner, big.NewInt(1000), 30, time.Now(), false)
	selfIdx, err := csm.putBucketAndIndex(selfBkt)
	require.NoError(err)
	cand.SelfStakeBucketIdx = selfIdx

	// Bucket 1: voter A, 200 IOTX, 30-day, non-auto → contributes weight to A.
	bktA := NewVoteBucket(candOwner, voterA, big.NewInt(200), 30, time.Now(), false)
	_, err = csm.putBucketAndIndex(bktA)
	require.NoError(err)

	// Bucket 2: voter B, unstaked → excluded. isUnstaked() checks
	// UnstakeStartTime.After(StakeStartTime); pin both explicitly so the
	// assertion doesn't rely on time.Now() advancing between calls.
	stakeStart := time.Now().UTC()
	bktB := NewVoteBucket(candOwner, voterB, big.NewInt(500), 30, stakeStart, false)
	bktB.UnstakeStartTime = stakeStart.Add(time.Hour)
	_, err = csm.putBucketAndIndex(bktB)
	require.NoError(err)
	require.True(bktB.isUnstaked(), "bktB must be marked unstaked for the exclusion assertion to be meaningful")

	// Bucket 3: voter A, 100 IOTX, another contribution → aggregates with bktA.
	bktA2 := NewVoteBucket(candOwner, voterA, big.NewInt(100), 30, time.Now(), false)
	_, err = csm.putBucketAndIndex(bktA2)
	require.NoError(err)

	// Bucket 4: voter A staking to a DIFFERENT candidate. Never queried by our
	// candidate index lookup, but useful sanity: it shouldn't leak in.
	bktOther := NewVoteBucket(otherCand, voterA, big.NewInt(1_000_000), 30, time.Now(), false)
	_, err = csm.putBucketAndIndex(bktOther)
	require.NoError(err)

	// Mock a contract staking indexer that returns one voter-B bucket.
	// Contract buckets (Timestamped=false) use UnstakeStartBlockHeight for
	// the isUnstaked() check — the sentinel MaxDurationNumber means "not
	// unstaked" (a real unstake would write a real block height).
	contractBkt := &VoteBucket{
		Index:                   99,
		Candidate:               candOwner,
		Owner:                   voterB,
		StakedAmount:            big.NewInt(50),
		StakedDuration:          30 * 24 * time.Hour,
		ContractAddress:         "io1contract",
		UnstakeStartBlockHeight: MaxDurationNumber,
	}
	mockIndexer := NewMockContractStakingIndexer(ctrl)
	mockIndexer.EXPECT().
		BucketsByCandidate(gomock.Any(), gomock.Any()).
		Return([]*VoteBucket{contractBkt}, nil).
		AnyTimes()

	view, err := buildVoterWeightBaseFromState(
		csr,
		CandidateList{cand},
		[]ContractStakingIndexer{mockIndexer},
		0,
		genesis.Default.VoteWeightCalConsts,
	)
	require.NoError(err)
	require.NotNil(view)

	entries := view.Weights(cand.GetIdentifier())

	// Two voters expected: A (300 aggregate) and B (50 from contract).
	require.Len(entries, 2)

	// Sorted by voter address bytes.
	require.True(bytes.Compare(entries[0].Voter.Bytes(), entries[1].Voter.Bytes()) < 0)

	byAddr := map[string]VoterWeight{}
	for _, e := range entries {
		byAddr[e.Voter.String()] = e
	}
	require.Contains(byAddr, voterA.String())
	require.Contains(byAddr, voterB.String())

	// Weight for A is CalculateVoteWeight(200) + CalculateVoteWeight(100)
	// for two 30-day non-auto buckets. Compare via the same formula.
	expectedA := new(big.Int).Add(
		CalculateVoteWeight(genesis.Default.VoteWeightCalConsts, bktA, false),
		CalculateVoteWeight(genesis.Default.VoteWeightCalConsts, bktA2, false),
	)
	require.Zero(expectedA.Cmp(byAddr[voterA.String()].Weight), "voterA weight mismatch")

	expectedB := CalculateVoteWeight(genesis.Default.VoteWeightCalConsts, contractBkt, false)
	require.Zero(expectedB.Cmp(byAddr[voterB.String()].Weight), "voterB weight (contract) mismatch")
}
