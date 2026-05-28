// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

// stubDocument is a tiny Document implementation for tests: it hashes a
// fixed payload so two stubDocument values with the same payload produce
// the same hash.
type stubDocument struct {
	payload []byte
}

func (d *stubDocument) Hash() ([]byte, error) {
	h := hash.Hash256b(d.payload)
	return h[:], nil
}

// blsTestKeys derives a deterministic (ECDSA producer, BLS) key pair from a
// seed byte. Used in place of identityset to avoid pulling the test package
// into the endorsement package's tests.
func blsTestKeys(t *testing.T, seed byte) (crypto.PrivateKey, *crypto.BLS12381PrivateKey) {
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

func TestEndorseBLS_RoundTrip(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x11)
	doc := &stubDocument{payload: []byte("hello-bls")}
	ts := time.Unix(1700000000, 0).UTC()

	en, err := EndorseBLS(doc, ts, ecdsa.PublicKey(), bls)
	require.NoError(err)
	require.NotNil(en)
	require.Equal(crypto.BLSAggregateSignatureLength, len(en.Signature()),
		"BLS signature should be 96 bytes (G2 compressed)")
	require.True(en.Timestamp().Equal(ts), "timestamp preserved")
	require.Equal(ecdsa.PublicKey().HexString(), en.Endorser().HexString(),
		"endorser stays on the secp256k1 pubkey for address derivation")

	require.True(VerifyBLSEndorsement(doc, en, bls.PublicKey()),
		"verify with the matching BLS pubkey passes")
}

func TestVerifyBLSEndorsement_WrongPubKey(t *testing.T) {
	require := require.New(t)
	ecdsa, signer := blsTestKeys(t, 0x22)
	_, other := blsTestKeys(t, 0x33)
	doc := &stubDocument{payload: []byte("doc-a")}

	en, err := EndorseBLS(doc, time.Unix(1700000000, 0), ecdsa.PublicKey(), signer)
	require.NoError(err)
	require.False(VerifyBLSEndorsement(doc, en, other.PublicKey()),
		"a different BLS pubkey must not verify")
}

func TestVerifyBLSEndorsement_TamperedDoc(t *testing.T) {
	require := require.New(t)
	ecdsa, signer := blsTestKeys(t, 0x44)
	doc := &stubDocument{payload: []byte("original")}
	other := &stubDocument{payload: []byte("tampered")}

	en, err := EndorseBLS(doc, time.Unix(1700000000, 0), ecdsa.PublicKey(), signer)
	require.NoError(err)
	require.False(VerifyBLSEndorsement(other, en, signer.PublicKey()),
		"verify against a different document must fail")
}

func TestVerifyBLSEndorsement_TamperedSig(t *testing.T) {
	require := require.New(t)
	ecdsa, signer := blsTestKeys(t, 0x55)
	doc := &stubDocument{payload: []byte("payload")}

	en, err := EndorseBLS(doc, time.Unix(1700000000, 0), ecdsa.PublicKey(), signer)
	require.NoError(err)

	// flip a bit in the signature and rewrap
	tamperedSig := en.Signature()
	tamperedSig[0] ^= 0x01
	tampered := NewEndorsement(en.Timestamp(), en.Endorser(), tamperedSig)
	require.False(VerifyBLSEndorsement(doc, tampered, signer.PublicKey()),
		"a tampered BLS signature must not verify")
}

func TestVerifyBLSEndorsement_NilInputs(t *testing.T) {
	require := require.New(t)
	_, signer := blsTestKeys(t, 0x66)
	doc := &stubDocument{payload: []byte("payload")}

	require.False(VerifyBLSEndorsement(doc, nil, signer.PublicKey()),
		"nil endorsement must not verify")
	require.False(VerifyBLSEndorsement(doc, &Endorsement{}, nil),
		"nil pubkey must not verify")
}
