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
	h.pubkey = priv.PublicKey()

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
	require.NotNil(h.PublicKey(), "PublicKey() returns the secp256k1 typed key")
}

func TestHeader_PostForkBLS(t *testing.T) {
	require := require.New(t)
	_, bls := blsHeaderKeys(t, 0x11)
	h := buildHeaderCore(t)
	h.pubkey = bls.PublicKey()
	require.Equal(crypto.BLSPubkeyLength, len(h.pubkey.Bytes()),
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

	// secp256k1-typed pubkey with a BLS-length signature: the typed key's
	// own Verify rejects the 96B input (secp256k1 expects 65B).
	ecdsa, _ := blsHeaderKeys(t, 0x21)
	h := buildHeaderCore(t)
	h.pubkey = ecdsa.PublicKey()
	h.blockSig = make([]byte, crypto.BLSAggregateSignatureLength)
	require.False(h.VerifySignature(),
		"BLS-length sig with secp256k1 pubkey must not verify")

	// BLS-typed pubkey with an ECDSA-length signature: BLS Verify rejects
	// the 65B input as a malformed G2 compressed signature.
	_, bls := blsHeaderKeys(t, 0x22)
	h = buildHeaderCore(t)
	h.pubkey = bls.PublicKey()
	h.blockSig = make([]byte, crypto.Secp256k1SigSizeWithRecID)
	require.False(h.VerifySignature(),
		"secp256k1-length sig with BLS pubkey must not verify")
}

func TestHeader_VerifySignature_EmptyPubkey(t *testing.T) {
	h := buildHeaderCore(t)
	h.blockSig = make([]byte, crypto.Secp256k1SigSizeWithRecID)
	require.False(t, h.VerifySignature(), "nil pubkey rejects verification")
}

func TestHeader_PostForkBLS_ProtoRoundTrip(t *testing.T) {
	require := require.New(t)
	_, bls := blsHeaderKeys(t, 0x33)
	h := buildHeaderCore(t)
	h.pubkey = bls.PublicKey()
	digest := h.HashHeaderCore()
	sig, err := bls.Sign(digest[:])
	require.NoError(err)
	h.blockSig = sig

	ser, err := h.Serialize()
	require.NoError(err)

	var restored Header
	require.NoError(restored.Deserialize(ser))
	// Length-dispatch at decode time must reconstruct a BLS-typed key, not
	// just preserve the bytes.
	_, ok := restored.pubkey.(*crypto.BLS12381PublicKey)
	require.True(ok, "decoded pubkey should be typed as BLS12381PublicKey")
	require.Equal(h.pubkey.Bytes(), restored.pubkey.Bytes())
	require.Equal(h.blockSig, restored.blockSig)
	require.True(restored.VerifySignature(),
		"BLS header survives proto round-trip and re-verifies")
	require.Equal(h.ProducerAddress(), restored.ProducerAddress())
}

func TestHeader_LoadFromProto_InvalidPubkey(t *testing.T) {
	// Decode-time length dispatch should reject pubkey bytes that can't be
	// parsed as either secp256k1 or BLS12-381 — surfaces the error rather
	// than silently storing garbage bytes.
	require := require.New(t)
	h := buildHeaderCore(t)
	h.pubkey = identityset.PrivateKey(7).PublicKey()
	h.blockSig = make([]byte, crypto.Secp256k1SigSizeWithRecID)
	pb := h.Proto()
	pb.ProducerPubkey = []byte{0x00, 0x01, 0x02} // neither 33/65 nor 48 bytes
	var restored Header
	err := restored.LoadFromBlockHeaderProto(pb)
	require.Error(err, "garbage pubkey bytes must be rejected at decode time")
}
