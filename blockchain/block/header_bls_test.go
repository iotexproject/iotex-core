// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// blsHeaderKeys deterministically builds an ECDSA + BLS keypair for tests.
func blsHeaderKeys(t *testing.T, seed byte) (crypto.PrivateKey, *crypto.BLS12381PrivateKey) {
	t.Helper()
	ikm := make([]byte, 32)
	for i := range ikm {
		ikm[i] = seed
	}
	ecdsa, err := crypto.BytesToPrivateKey(ikm)
	require.NoError(t, err)
	bls, err := crypto.GenerateBLS12381PrivateKey(ikm)
	require.NoError(t, err)
	return ecdsa, bls
}

// buildHeaderCore returns a Header populated with the same core fields used
// by the existing header tests, ready for the caller to plug in producerPubkey
// + blockSig and then HashHeaderCore / sign.
func buildHeaderCore(t *testing.T) Header {
	t.Helper()
	bf, err := bloom.NewBloomFilterLegacy(2048, 3)
	require.NoError(t, err)
	return Header{
		version:          1,
		height:           42,
		gasUsed:          1000,
		timestamp:        time.Unix(1700000000, 0).UTC(),
		prevBlockHash:    hash.Hash256b([]byte("prev")),
		txRoot:           hash.Hash256b([]byte("tx")),
		deltaStateDigest: hash.Hash256b([]byte("delta")),
		receiptRoot:      hash.Hash256b([]byte("receipt")),
		logsBloom:        bf,
		baseFee:          big.NewInt(1000),
	}
}

func TestHeader_PreForkSecp256k1(t *testing.T) {
	require := require.New(t)
	priv := identityset.PrivateKey(7)
	h := buildHeaderCore(t)
	h.producerPubkey = priv.PublicKey().Bytes()

	digest := h.HashHeaderCore()
	sig, err := priv.Sign(digest[:])
	require.NoError(err)
	require.Equal(crypto.Secp256k1SigSizeWithRecID, len(sig),
		"secp256k1 signature should be 65 bytes")
	h.blockSig = sig

	require.True(h.VerifySignature(), "secp256k1 header should verify")
	require.Equal(priv.PublicKey().Address().String(), h.ProducerAddress(),
		"pre-fork ProducerAddress is the io1... iotex address")
	require.Equal(priv.PublicKey().Bytes(), h.ProducerPubKey(),
		"ProducerPubKey returns the raw secp256k1 pubkey bytes")
	require.NotNil(h.PublicKey(), "PublicKey() decodes secp256k1 pubkey")
}

func TestHeader_PostForkBLS(t *testing.T) {
	require := require.New(t)
	_, bls := blsHeaderKeys(t, 0x11)
	h := buildHeaderCore(t)
	h.producerPubkey = bls.PublicKey().Bytes()
	require.Equal(crypto.BLSPubkeyLength, len(h.producerPubkey),
		"BLS pubkey should be 48 bytes")

	digest := h.HashHeaderCore()
	sig, err := bls.Sign(digest[:])
	require.NoError(err)
	require.Equal(crypto.BLSAggregateSignatureLength, len(sig),
		"BLS signature should be 96 bytes")
	h.blockSig = sig

	require.True(h.VerifySignature(), "BLS header should verify")
	require.Equal(hex.EncodeToString(bls.PublicKey().Bytes()), h.ProducerAddress(),
		"post-fork ProducerAddress is the hex of the BLS pubkey")
	require.Equal(bls.PublicKey().Bytes(), h.ProducerPubKey(),
		"ProducerPubKey returns the raw BLS pubkey bytes")
	require.Nil(h.PublicKey(),
		"PublicKey() returns nil for BLS-signed headers (BLS pubkey is not secp256k1)")
}

func TestHeader_VerifySignature_WrongScheme(t *testing.T) {
	require := require.New(t)

	// ECDSA pubkey paired with a BLS-length signature: dispatch hits the BLS
	// branch, BLS12381PublicKeyFromBytes rejects the 33/65B input.
	ecdsa, _ := blsHeaderKeys(t, 0x21)
	h := buildHeaderCore(t)
	h.producerPubkey = ecdsa.PublicKey().Bytes()
	h.blockSig = make([]byte, crypto.BLSAggregateSignatureLength) // length-correct, content invalid
	require.False(h.VerifySignature(),
		"BLS-length sig with secp256k1 pubkey must not verify")

	// BLS pubkey paired with an ECDSA-length signature: dispatch hits the
	// secp256k1 branch, BytesToPublicKey rejects the 48B input.
	_, bls := blsHeaderKeys(t, 0x22)
	h = buildHeaderCore(t)
	h.producerPubkey = bls.PublicKey().Bytes()
	h.blockSig = make([]byte, crypto.Secp256k1SigSizeWithRecID)
	require.False(h.VerifySignature(),
		"secp256k1-length sig with BLS pubkey must not verify")
}

func TestHeader_VerifySignature_EmptyPubkey(t *testing.T) {
	h := buildHeaderCore(t)
	h.blockSig = make([]byte, crypto.Secp256k1SigSizeWithRecID)
	require.False(t, h.VerifySignature(), "empty producerPubkey rejects verification")
}

func TestHeader_PostForkBLS_ProtoRoundTrip(t *testing.T) {
	require := require.New(t)
	_, bls := blsHeaderKeys(t, 0x33)
	h := buildHeaderCore(t)
	h.producerPubkey = bls.PublicKey().Bytes()
	digest := h.HashHeaderCore()
	sig, err := bls.Sign(digest[:])
	require.NoError(err)
	h.blockSig = sig

	ser, err := h.Serialize()
	require.NoError(err)

	var restored Header
	require.NoError(restored.Deserialize(ser))
	require.Equal(h.producerPubkey, restored.producerPubkey)
	require.Equal(h.blockSig, restored.blockSig)
	require.True(restored.VerifySignature(),
		"BLS header survives proto round-trip and re-verifies")
	require.Equal(h.ProducerAddress(), restored.ProducerAddress())
}
