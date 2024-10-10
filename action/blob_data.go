// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

// BlobTxData represents an EIP-4844 transaction
type BlobTxData struct {
	blobFeeCap *uint256.Int // a.k.a. maxFeePerBlobGas
	blobHashes []common.Hash

	// A blob transaction can optionally contain blobs. This field must be set when BlobTx
	// is used to create a transaction for signing
	sidecar *types.BlobTxSidecar
}

func NewBlobTxData(blobFeeCap *uint256.Int, blobHashes []common.Hash, sidecar *types.BlobTxSidecar) *BlobTxData {
	return &BlobTxData{
		blobFeeCap: blobFeeCap,
		blobHashes: blobHashes,
		sidecar:    sidecar,
	}
}

func (tx *BlobTxData) gasFeeCap() *uint256.Int {
	p := uint256.Int{}
	if tx.blobFeeCap != nil {
		p.Set(tx.blobFeeCap)
	}
	return &p
}

func (tx *BlobTxData) hashes() []common.Hash {
	return tx.blobHashes
}

func (tx *BlobTxData) gas() uint64 {
	return params.BlobTxBlobGasPerBlob * uint64(len(tx.blobHashes))
}

func (tx *BlobTxData) blobHashesProto() [][]byte {
	size := len(tx.blobHashes)
	if tx.blobHashes == nil || size == 0 {
		return nil
	}
	hashes := make([][]byte, size)
	for i := 0; i < size; i++ {
		hashes[i] = tx.blobHashes[i][:]
	}
	return hashes
}

func ToProtoSideCar(sidecar *types.BlobTxSidecar) *iotextypes.BlobTxSidecar {
	if sidecar == nil || len(sidecar.Blobs) == 0 {
		return nil
	}

	pbSidecar := iotextypes.BlobTxSidecar{}
	if size := len(sidecar.Blobs); size > 0 {
		pbSidecar.Blobs = make([][]byte, size)
		for i := 0; i < size; i++ {
			pbSidecar.Blobs[i] = sidecar.Blobs[i][:]
		}
	}
	if size := len(sidecar.Commitments); size > 0 {
		pbSidecar.Commitments = make([][]byte, size)
		for i := 0; i < size; i++ {
			pbSidecar.Commitments[i] = sidecar.Commitments[i][:]
		}
	}
	if size := len(sidecar.Proofs); size > 0 {
		pbSidecar.Proofs = make([][]byte, size)
		for i := 0; i < size; i++ {
			pbSidecar.Proofs[i] = sidecar.Proofs[i][:]
		}
	}
	return &pbSidecar
}

func (tx *BlobTxData) toProto() *iotextypes.BlobTxData {
	blob := iotextypes.BlobTxData{}
	if tx.blobFeeCap != nil {
		blob.BlobFeeCap = tx.blobFeeCap.String()
	}
	blob.BlobHashes = tx.blobHashesProto()
	blob.BlobTxSidecar = ToProtoSideCar(tx.sidecar)
	return &blob
}

func fromProtoBlobTxData(pb *iotextypes.BlobTxData) (*BlobTxData, error) {
	if pb == nil {
		return nil, ErrNilProto
	}
	blob := BlobTxData{
		blobFeeCap: &uint256.Int{},
	}
	if fee := pb.GetBlobFeeCap(); len(fee) > 0 {
		if err := blob.blobFeeCap.SetFromDecimal(fee); err != nil {
			return nil, errors.Wrap(err, "invalid blob fee")
		}
	}
	if bh := pb.GetBlobHashes(); bh != nil {
		blob.blobHashes = make([]common.Hash, len(bh))
		for i := 0; i < len(bh); i++ {
			blob.blobHashes[i] = common.BytesToHash(bh[i])
		}
	}
	if sc := pb.GetBlobTxSidecar(); sc != nil {
		var err error
		if blob.sidecar, err = FromProtoBlobTxSideCar(sc); err != nil {
			return nil, err
		}
	}
	return &blob, nil
}

func FromProtoBlobTxSideCar(pb *iotextypes.BlobTxSidecar) (*types.BlobTxSidecar, error) {
	if pb == nil || len(pb.Blobs) == 0 {
		return nil, ErrNilProto
	}
	sidecar := types.BlobTxSidecar{
		Blobs:       make([]kzg4844.Blob, len(pb.Blobs)),
		Commitments: make([]kzg4844.Commitment, len(pb.Commitments)),
		Proofs:      make([]kzg4844.Proof, len(pb.Proofs)),
	}
	for i := range pb.Blobs {
		sidecar.Blobs[i] = *(*kzg4844.Blob)(pb.Blobs[i])
	}
	for i := range pb.Commitments {
		sidecar.Commitments[i] = *(*kzg4844.Commitment)(pb.Commitments[i])
	}
	for i := range pb.Proofs {
		sidecar.Proofs[i] = *(*kzg4844.Proof)(pb.Proofs[i])
	}
	return &sidecar, nil
}

func (tx *BlobTxData) SanityCheck() error {
	if price := tx.blobFeeCap; price != nil && price.Sign() < 0 {
		return errors.Wrap(ErrNegativeValue, "negative blob fee cap")
	}
	if len(tx.blobHashes) == 0 {
		return errors.New("blobless blob transaction")
	}
	if permitted := params.MaxBlobGasPerBlock / params.BlobTxBlobGasPerBlob; len(tx.blobHashes) > permitted {
		return errors.Errorf("too many blobs in transaction: have %d, permitted %d", len(tx.blobHashes), params.MaxBlobGasPerBlock/params.BlobTxBlobGasPerBlob)
	}
	return nil
}

func (tx *BlobTxData) ValidateSidecar() error {
	if tx.sidecar == nil {
		return errors.New("sidecar is missing")
	}
	return verifySidecar(tx.sidecar, tx.blobHashes)
}

func verifySidecar(sidecar *types.BlobTxSidecar, hashes []common.Hash) error {
	size := len(hashes)
	// Verify the size of hashes, commitments and proofs
	if len(sidecar.Blobs) != size {
		return errors.New("number of blobs and hashes mismatch")
	}
	if len(sidecar.Commitments) != size {
		return errors.New("number of blobs and commitments mismatch")
	}
	if len(sidecar.Proofs) != size {
		return errors.New("number of blobs and proofs mismatch")
	}
	// Blob quantities match up, validate that the provers match with the
	// transaction hash before getting to the cryptography
	hasher := sha256.New()
	for i, vhash := range hashes {
		computed := kzg4844.CalcBlobHashV1(hasher, &sidecar.Commitments[i])
		if vhash != computed {
			return errors.Errorf("blob %d: computed hash %x mismatches transaction one %x", i, computed, vhash)
		}
	}
	// Blob commitments match with the hashes in the transaction, verify the
	// blobs themselves via KZG
	for i := range sidecar.Blobs {
		if err := kzg4844.VerifyBlobProof(sidecar.Blobs[i], sidecar.Commitments[i], sidecar.Proofs[i]); err != nil {
			return errors.Errorf("invalid blob %d: %v", i, err)
		}
	}
	return nil
}
