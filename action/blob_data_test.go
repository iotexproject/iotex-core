// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
)

func TestBlobTxHashing(t *testing.T) {
	r := require.New(t)

	var (
		sk       = MustNoErrorV(crypto.GenerateKey()).EcdsaPrivateKey().(*ecdsa.PrivateKey)
		blobData = createTestBlobTxData()
	)
	withBlob := &types.BlobTx{
		ChainID:   uint256.NewInt(1),
		Nonce:     5,
		GasTipCap: uint256.NewInt(1000),
		GasFeeCap: uint256.NewInt(3000),
		Gas:       21000,
		Value:     uint256.NewInt(3000000000000),
		// AccessList AccessList
		BlobFeeCap: uint256.NewInt(1000000000),
	}
	withBlob.BlobFeeCap = blobData.blobFeeCap
	withBlob.BlobHashes = blobData.blobHashes
	withBlob.Sidecar = blobData.sidecar
	r.NotNil(withBlob.Sidecar)

	signer := types.NewCancunSigner(withBlob.ChainID.ToBig())
	tx := types.NewTx(withBlob)
	raw := signer.Hash(tx)
	withBlobSigned := types.MustSignNewTx(sk, signer, withBlob)
	r.Equal(blobData.sidecar, withBlob.Sidecar)
	r.Equal(blobData.sidecar, tx.BlobTxSidecar())
	r.Equal(blobData.sidecar, withBlobSigned.BlobTxSidecar())
	h := withBlobSigned.Hash()
	// without blob
	withBlob.Sidecar = nil
	tx = types.NewTx(withBlob)
	woBlobSigned := types.MustSignNewTx(sk, signer, withBlob)
	r.Equal(raw, signer.Hash(tx))
	r.Nil(withBlob.Sidecar)
	r.Nil(tx.BlobTxSidecar())
	r.Nil(woBlobSigned.BlobTxSidecar())
	r.Equal(h, woBlobSigned.Hash())
	// remove blob from signed tx
	withBlobStripped := withBlobSigned.WithoutBlobTxSidecar()
	r.Nil(withBlobStripped.BlobTxSidecar())
	r.Equal(h, withBlobStripped.Hash())
}

func TestBlobTxData(t *testing.T) {
	r := require.New(t)
	blobData := createTestBlobTxData()

	t.Run("Proto", func(t *testing.T) {
		h := blobData.blobHashes
		b := blobData.sidecar.Blobs
		blobData.blobHashes = blobData.blobHashes[:0]
		r.Nil(blobData.blobHashesProto())
		blobData.sidecar.Blobs = blobData.sidecar.Blobs[:0]
		r.Nil(ToProtoSideCar(blobData.sidecar))
		blobData.blobHashes = h
		blobData.sidecar.Blobs = b
		pb := blobData.toProto()
		raw := MustNoErrorV(proto.Marshal(pb))
		r.Equal(131218, len(raw))
		recv := iotextypes.BlobTxData{}
		r.NoError(proto.Unmarshal(raw, &recv))
		decodeBlob := MustNoErrorV(fromProtoBlobTxData(&recv))
		r.Equal(blobData, decodeBlob)
	})
	t.Run("SanityCheck/ValidateSidecar", func(t *testing.T) {
		r.NoError(blobData.SanityCheck())
		blobData.blobFeeCap = uint256.NewInt(1)
		blobData.blobFeeCap.Lsh(blobData.blobFeeCap, 255)
		r.ErrorIs(blobData.SanityCheck(), ErrNegativeValue)
		r.NoError(blobData.ValidateSidecar())
		// check blob hashes size
		h := blobData.blobHashes
		blobData.blobHashes = blobData.blobHashes[:0]
		r.ErrorContains(blobData.ValidateSidecar(), "number of blobs and hashes mismatch")
		blobData.blobHashes = h
		// check Blobs, Commitments, Proofs size
		sidecar := blobData.sidecar
		sidecar.Blobs = append(sidecar.Blobs, kzg4844.Blob{})
		r.ErrorContains(blobData.ValidateSidecar(), "number of blobs and hashes mismatch")
		sidecar.Blobs = sidecar.Blobs[:1]
		sidecar.Commitments = append(sidecar.Commitments, kzg4844.Commitment{})
		r.ErrorContains(blobData.ValidateSidecar(), "number of blobs and commitments mismatch")
		sidecar.Commitments = sidecar.Commitments[:1]
		sidecar.Proofs = append(sidecar.Proofs, kzg4844.Proof{})
		r.ErrorContains(blobData.ValidateSidecar(), "number of blobs and proofs mismatch")
		sidecar.Proofs = sidecar.Proofs[:1]
		r.NoError(blobData.ValidateSidecar())
		// verify commitments hash
		b := sidecar.Commitments[0][3]
		sidecar.Commitments[0][3] = b + 1
		r.ErrorContains(blobData.ValidateSidecar(), "blob 0: computed hash 01fca1582898b9c172b690c0ea344713bb28199208d3553b5ed56f33e0f34034 mismatches transaction one")
		sidecar.Commitments[0][3] = b
		// verify blobs via KZG
		b = sidecar.Blobs[0][31]
		sidecar.Blobs[0][31] = b + 1
		r.ErrorContains(blobData.ValidateSidecar(), "invalid blob 0: can't verify opening proof")
		sidecar.Blobs[0][31] = b
		b = sidecar.Proofs[0][42]
		sidecar.Proofs[0][42] = b + 1
		r.ErrorContains(blobData.ValidateSidecar(), "invalid blob 0: invalid compressed coordinate: square root doesn't exist")
		sidecar.Proofs[0][42] = b
		b = sidecar.Proofs[0][47]
		sidecar.Proofs[0][47] = b + 1
		r.ErrorContains(blobData.ValidateSidecar(), "invalid blob 0: invalid point: subgroup check failed")
		sidecar.Proofs[0][47] = b
		r.NoError(blobData.ValidateSidecar())
	})
}

var (
	testBlob       = kzg4844.Blob{1, 2, 3, 4}
	testBlobCommit = MustNoErrorV(kzg4844.BlobToCommitment(testBlob))
	testBlobProof  = MustNoErrorV(kzg4844.ComputeBlobProof(testBlob, testBlobCommit))
)

func createTestBlobTxData() *BlobTxData {
	sidecar := &types.BlobTxSidecar{
		Blobs:       []kzg4844.Blob{testBlob},
		Commitments: []kzg4844.Commitment{testBlobCommit},
		Proofs:      []kzg4844.Proof{testBlobProof},
	}
	blobData := &BlobTxData{
		blobFeeCap: uint256.NewInt(15),
		blobHashes: sidecar.BlobHashes(),
		sidecar:    sidecar,
	}
	return blobData
}
