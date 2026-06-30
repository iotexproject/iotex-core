// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"math/big"
	"sort"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestVoterWeightSnapKey verifies the on-disk key layout: 1 byte tag +
// 20 byte candidate identifier. Stability matters because changing it
// breaks compatibility with already-snapshotted state at upgrade time
// (post-fork-activation only — pre-flag chains have no snapshots).
func TestVoterWeightSnapKey(t *testing.T) {
	r := require.New(t)
	cand := identityset.Address(3)

	key := voterWeightSnapKey(cand)
	r.Len(key, 21)
	r.Equal(_voterWeightSnap, key[0])
	r.True(bytes.Equal(cand.Bytes(), key[1:]))
}

// TestEncodeVoterWeightSnapshotDeterminism is the core invariant that
// makes the "unchanged-blob skip" in SnapshotVoterWeights correct:
// encoding the same logical voter list must produce byte-identical
// output. Without this, every PutPollResult would rewrite every
// candidate blob even when nothing changed.
func TestEncodeVoterWeightSnapshotDeterminism(t *testing.T) {
	r := require.New(t)

	build := func() []voterWeight {
		return []voterWeight{
			{voter: identityset.Address(2), weight: big.NewInt(100)},
			{voter: identityset.Address(5), weight: big.NewInt(250)},
			{voter: identityset.Address(7), weight: big.NewInt(50)},
		}
	}
	// Both lists must be in the same sorted order — sort upstream as
	// VoterWeightView does, so we mirror that here.
	a := build()
	sort.Slice(a, func(i, j int) bool {
		return bytes.Compare(a[i].voter.Bytes(), a[j].voter.Bytes()) < 0
	})
	b := build()
	sort.Slice(b, func(i, j int) bool {
		return bytes.Compare(b[i].voter.Bytes(), b[j].voter.Bytes()) < 0
	})

	_, blobA, err := encodeVoterWeightSnapshot(a)
	r.NoError(err)
	_, blobB, err := encodeVoterWeightSnapshot(b)
	r.NoError(err)
	r.True(bytes.Equal(blobA, blobB), "same input must produce byte-identical blobs")
	r.NotEmpty(blobA)
}

// TestEncodeVoterWeightSnapshotEmpty verifies the snapshot writer's
// "no voters → no blob" contract. SnapshotVoterWeights relies on this
// to know when to call DelState instead of PutState.
func TestEncodeVoterWeightSnapshotEmpty(t *testing.T) {
	r := require.New(t)
	pb, blob, err := encodeVoterWeightSnapshot(nil)
	r.NoError(err)
	r.Nil(blob)
	r.Nil(pb)

	pb, blob, err = encodeVoterWeightSnapshot([]voterWeight{})
	r.NoError(err)
	r.Nil(blob)
	r.Nil(pb)
}

// TestVoterWeightSnapshotRoundtrip exercises the writer-format /
// reader-format symmetry: a blob produced by encodeVoterWeightSnapshot
// must decode back to the same public VoterWeight list the rewarding
// protocol consumes (modulo weights being equal *big.Int values rather
// than the same pointers).
func TestVoterWeightSnapshotRoundtrip(t *testing.T) {
	r := require.New(t)

	input := []voterWeight{
		{voter: identityset.Address(2), weight: big.NewInt(100)},
		{voter: identityset.Address(5), weight: new(big.Int).Mul(big.NewInt(125), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))},
		{voter: identityset.Address(7), weight: big.NewInt(1)},
	}
	sort.Slice(input, func(i, j int) bool {
		return bytes.Compare(input[i].voter.Bytes(), input[j].voter.Bytes()) < 0
	})

	pb, blob, err := encodeVoterWeightSnapshot(input)
	r.NoError(err)
	r.NotEmpty(blob)
	r.NotNil(pb)
	r.Len(pb.Entries, len(input))

	out, err := decodeVoterWeightSnapshot(blob)
	r.NoError(err)
	r.Len(out, len(input))
	for i := range input {
		r.True(address.Equal(input[i].voter, out[i].Voter), "voter %d", i)
		r.Equal(0, input[i].weight.Cmp(out[i].Weight), "weight %d: want %s got %s",
			i, input[i].weight.String(), out[i].Weight.String())
	}
}

// TestDecodeVoterWeightSnapshotEmpty verifies the reader's "no blob →
// nil" contract, which lets SnapshotVoterWeightsByCandidate return nil
// for candidates with no stored snapshot without a separate
// "exists?" probe.
func TestDecodeVoterWeightSnapshotEmpty(t *testing.T) {
	r := require.New(t)
	out, err := decodeVoterWeightSnapshot(nil)
	r.NoError(err)
	r.Nil(out)

	out, err = decodeVoterWeightSnapshot([]byte{})
	r.NoError(err)
	r.Nil(out)
}
