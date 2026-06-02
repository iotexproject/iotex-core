// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/endorsement"
)

// buildDelegateSet returns a slice of n delegates with deterministic ECDSA +
// BLS key pairs (seeded from i+1). Convenient for aggregation tests.
func buildDelegateSet(t *testing.T, n int) ([]*Delegate, []crypto.PrivateKey, []*crypto.BLS12381PrivateKey) {
	t.Helper()
	delegates := make([]*Delegate, n)
	ecdsaKeys := make([]crypto.PrivateKey, n)
	blsKeys := make([]*crypto.BLS12381PrivateKey, n)
	for i := 0; i < n; i++ {
		ecdsa, bls := blsTestKeys(t, byte(i+1))
		delegates[i] = &Delegate{Address: ecdsa.PublicKey().Address().String(), BLSPubKey: bls.PublicKey()}
		ecdsaKeys[i] = ecdsa
		blsKeys[i] = bls
	}
	return delegates, ecdsaKeys, blsKeys
}

func TestAggregateCommitEndorsements_RoundTrip(t *testing.T) {
	require := require.New(t)
	delegates, ecdsaKeys, blsKeys := buildDelegateSet(t, 4)
	blkHash := hash.Hash256b([]byte("block"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	ts := time.Unix(1700000000, 0).UTC()

	// All four delegates sign the same COMMIT vote at the same timestamp.
	ens := make([]*endorsement.Endorsement, len(delegates))
	for i := range delegates {
		en, err := endorsement.EndorseBLS(vote, ts, ecdsaKeys[i].PublicKey(), blsKeys[i])
		require.NoError(err)
		ens[i] = en
	}

	aggSig, bitmap, err := aggregateCommitEndorsements(ens, delegates)
	require.NoError(err)
	require.Equal(crypto.BLSAggregateSignatureLength, len(aggSig))
	require.Equal(byte(0x0f), bitmap[0], "all four delegates' bits set")

	// Verify: signers from bitmap → BLS pubkeys → FastAggregateVerify.
	signers, err := bitmapSigners(bitmap, delegates)
	require.NoError(err)
	require.Equal(len(delegates), len(signers))
	pubKeys := make([]*crypto.BLS12381PublicKey, len(signers))
	for i, d := range signers {
		pubKeys[i] = d.BLSPubKey
	}
	parsed, err := crypto.BLSAggregateSignatureFromBytes(aggSig)
	require.NoError(err)
	msg, err := endorsement.SigningHash(vote, ts)
	require.NoError(err)
	require.True(parsed.Verify(pubKeys, msg))
}

func TestAggregateCommitEndorsements_PartialSet(t *testing.T) {
	require := require.New(t)
	delegates, ecdsaKeys, blsKeys := buildDelegateSet(t, 5)
	blkHash := hash.Hash256b([]byte("partial"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	ts := time.Unix(1700000001, 0).UTC()

	// Only delegates 0, 2, 4 sign — the other two are absent.
	signing := []int{0, 2, 4}
	ens := make([]*endorsement.Endorsement, 0, len(signing))
	for _, i := range signing {
		en, err := endorsement.EndorseBLS(vote, ts, ecdsaKeys[i].PublicKey(), blsKeys[i])
		require.NoError(err)
		ens = append(ens, en)
	}

	aggSig, bitmap, err := aggregateCommitEndorsements(ens, delegates)
	require.NoError(err)
	require.Equal(byte(0b00010101), bitmap[0], "bits 0, 2, 4 set")

	signers, err := bitmapSigners(bitmap, delegates)
	require.NoError(err)
	require.Equal(3, len(signers))
	require.Equal(delegates[0].Address, signers[0].Address)
	require.Equal(delegates[2].Address, signers[1].Address)
	require.Equal(delegates[4].Address, signers[2].Address)

	pubKeys := []*crypto.BLS12381PublicKey{
		delegates[0].BLSPubKey, delegates[2].BLSPubKey, delegates[4].BLSPubKey,
	}
	parsed, err := crypto.BLSAggregateSignatureFromBytes(aggSig)
	require.NoError(err)
	msg, err := endorsement.SigningHash(vote, ts)
	require.NoError(err)
	require.True(parsed.Verify(pubKeys, msg))
}

func TestAggregateCommitEndorsements_RejectsNonBLSSig(t *testing.T) {
	require := require.New(t)
	delegates, ecdsaKeys, _ := buildDelegateSet(t, 3)
	blkHash := hash.Hash256b([]byte("ecdsa"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	ens, err := endorsement.Endorse(vote, time.Unix(1700000002, 0), ecdsaKeys[0])
	require.NoError(err)
	require.Equal(1, len(ens))
	_, _, err = aggregateCommitEndorsements(ens, delegates)
	require.Error(err)
	require.Contains(err.Error(), "non-BLS signature")
}

func TestAggregateCommitEndorsements_RejectsUnknownEndorser(t *testing.T) {
	require := require.New(t)
	delegates, _, _ := buildDelegateSet(t, 3)
	stranger, strangerBLS := blsTestKeys(t, 0x7f)
	blkHash := hash.Hash256b([]byte("stranger"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	en, err := endorsement.EndorseBLS(vote, time.Unix(1700000003, 0), stranger.PublicKey(), strangerBLS)
	require.NoError(err)
	_, _, err = aggregateCommitEndorsements([]*endorsement.Endorsement{en}, delegates)
	require.Error(err)
	require.Contains(err.Error(), "not in the round's delegate set")
}

func TestBitmapSigners_BitBeyondDelegateCount(t *testing.T) {
	require := require.New(t)
	delegates, _, _ := buildDelegateSet(t, 3)
	bitmap := []byte{0b00010111} // bit 4 set (no delegate at index 4)
	_, err := bitmapSigners(bitmap, delegates)
	require.Error(err)
	require.Contains(err.Error(), "beyond the delegate count")
}

func TestBitmapSigners_EmptyBitmap(t *testing.T) {
	require := require.New(t)
	delegates, _, _ := buildDelegateSet(t, 3)
	signers, err := bitmapSigners(nil, delegates)
	require.NoError(err)
	require.Equal(0, len(signers))
}
