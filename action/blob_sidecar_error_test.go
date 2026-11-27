// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"crypto/sha256"
	stderrors "errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/stretchr/testify/require"
)

func TestVerifySidecarErrorClassification(t *testing.T) {
	r := require.New(t)

	var blob kzg4844.Blob
	blob[0], blob[1], blob[2], blob[3] = 1, 2, 3, 4
	commit, err := kzg4844.BlobToCommitment(&blob)
	r.NoError(err)
	proof, err := kzg4844.ComputeBlobProof(&blob, commit)
	r.NoError(err)
	hashes := []common.Hash{kzg4844.CalcBlobHashV1(sha256.New(), &commit)}
	newSidecar := func() *types.BlobTxSidecar {
		return &types.BlobTxSidecar{
			Blobs:       []kzg4844.Blob{blob},
			Commitments: []kzg4844.Commitment{commit},
			Proofs:      []kzg4844.Proof{proof},
		}
	}
	r.NoError(verifySidecar(newSidecar(), hashes))

	// a well-formed proof computed for a different blob, so that the pairing
	// check itself runs and rejects it
	var other kzg4844.Blob
	other[0], other[1] = 9, 8
	otherCommit, err := kzg4844.BlobToCommitment(&other)
	r.NoError(err)
	otherProof, err := kzg4844.ComputeBlobProof(&other, otherCommit)
	r.NoError(err)

	t.Run("KZG verification failure", func(t *testing.T) {
		r := require.New(t)
		sc := newSidecar()
		sc.Proofs[0] = otherProof
		err := verifySidecar(sc, hashes)
		r.ErrorIs(err, ErrKZGVerification)
		r.NotErrorIs(err, ErrInvalidBlobSidecar)
		r.NotNil(stderrors.Unwrap(err))
		r.ErrorContains(err, "invalid blob 0: can't verify opening proof")

		// a proof that is not a valid point fails in KZG verification too
		sc = newSidecar()
		sc.Proofs[0][42]++
		r.ErrorIs(verifySidecar(sc, hashes), ErrKZGVerification)
	})

	t.Run("structural failures", func(t *testing.T) {
		mismatchedHash := newSidecar()
		mismatchedHash.Commitments[0] = otherCommit
		for name, sc := range map[string]*types.BlobTxSidecar{
			"blobs":       {Commitments: []kzg4844.Commitment{commit}, Proofs: []kzg4844.Proof{proof}},
			"commitments": {Blobs: []kzg4844.Blob{blob}, Proofs: []kzg4844.Proof{proof}},
			"proofs":      {Blobs: []kzg4844.Blob{blob}, Commitments: []kzg4844.Commitment{commit}},
			"hash":        mismatchedHash,
		} {
			err := verifySidecar(sc, hashes)
			require.ErrorIs(t, err, ErrInvalidBlobSidecar, name)
			require.NotErrorIs(t, err, ErrKZGVerification, name)
		}
		require.ErrorIs(t, (&BlobTxData{blobHashes: hashes}).ValidateSidecar(), ErrInvalidBlobSidecar)
	})
}
