// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	. "github.com/iotexproject/iotex-core/pkg/util/assertions"
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
	withBlob.BlobFeeCap = uint256.MustFromBig(blobData.blobFeeCap)
	withBlob.BlobHashes = blobData.blobHashes
	withBlob.Sidecar = blobData.sidecar
	r.NotNil(withBlob.Sidecar)

	signer := types.NewCancunSigner(withBlob.ChainID.ToBig())
	raw := signer.Hash(types.NewTx(withBlob))
	withBlobSigned := types.MustSignNewTx(sk, signer, withBlob)
	h := withBlobSigned.Hash()
	// without blob
	withBlob.Sidecar = nil
	r.Equal(raw, signer.Hash(types.NewTx(withBlob)))
	r.Equal(h, types.MustSignNewTx(sk, signer, withBlob).Hash())
	// remove blob from signed tx
	withBlobStripped := withBlobSigned.WithoutBlobTxSidecar()
	r.Equal(h, withBlobStripped.Hash())
}

func TestBlobTxSanity(t *testing.T) {
	r := require.New(t)

	blobData := createTestBlobTxData()
	r.NoError(blobData.SanityCheck())
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
		blobFeeCap: big.NewInt(15),
		blobHashes: sidecar.BlobHashes(),
		sidecar:    sidecar,
	}
	return blobData
}
