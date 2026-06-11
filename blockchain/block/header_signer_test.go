// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"crypto/sha256"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestECDSAHeaderSigner(t *testing.T) {
	require := require.New(t)
	sk := identityset.PrivateKey(7)
	signer := NewECDSAHeaderSigner(sk)

	// PubKey returns the secp256k1 public key as a Verifier; it must be
	// the concrete crypto.PublicKey type so the eventual VerifySignature
	// hits the ECDSA path.
	pk, ok := signer.PubKey().(crypto.PublicKey)
	require.True(ok, "ECDSA signer's pubkey must satisfy crypto.PublicKey")
	require.Equal(sk.PublicKey().Bytes(), pk.Bytes())

	msg := sha256.Sum256([]byte("hello"))
	sig, err := signer.Sign(msg[:])
	require.NoError(err)
	require.True(pk.Verify(msg[:], sig),
		"signature from ECDSAHeaderSigner verifies under the wrapped pubkey")
}

func TestBLSHeaderSigner(t *testing.T) {
	require := require.New(t)
	ikm := sha256.Sum256([]byte("bls-signer"))
	sk, err := crypto.GenerateBLS12381PrivateKey(ikm[:])
	require.NoError(err)
	signer := NewBLSHeaderSigner(sk)

	// PubKey returns the BLS pubkey as a Verifier; concrete type must be
	// *crypto.BLS12381PublicKey so VerifySignature hits the BLS path.
	pk, ok := signer.PubKey().(*crypto.BLS12381PublicKey)
	require.True(ok, "BLS signer's pubkey must be *crypto.BLS12381PublicKey")
	require.Equal(sk.PublicKey().Bytes(), pk.Bytes())

	msg := sha256.Sum256([]byte("hello"))
	sig, err := signer.Sign(msg[:])
	require.NoError(err)
	require.True(pk.Verify(msg[:], sig),
		"signature from BLSHeaderSigner verifies under the wrapped pubkey")
}

// TestSignAndBuild_NilSigner locks in the defensive nil check —
// SignAndBuild must reject rather than panic when handed a nil signer
// (e.g. from a misconfigured caller).
func TestSignAndBuild_NilSigner(t *testing.T) {
	require := require.New(t)
	b := NewBuilder(RunnableActions{}).SetHeight(1)
	_, err := b.SignAndBuild(nil)
	require.Error(err)
	require.Contains(err.Error(), "nil header signer")
}

// TestSignAndBuild_EndToEnd verifies that the signed block's stored
// pubkey + signature reverify correctly via VerifySignature, for both
// schemes — i.e. HeaderSigner integrates cleanly with the Verifier path.
func TestSignAndBuild_EndToEnd(t *testing.T) {
	require := require.New(t)

	t.Run("ECDSA", func(t *testing.T) {
		signer := NewECDSAHeaderSigner(identityset.PrivateKey(7))
		blk, err := NewBuilder(RunnableActions{}).
			SetHeight(1).
			SetVersion(1).
			SignAndBuild(signer)
		require.NoError(err)
		require.True(blk.Header.VerifySignature(),
			"ECDSA-signed block round-trips through VerifySignature")
	})

	t.Run("BLS", func(t *testing.T) {
		ikm := sha256.Sum256([]byte("bls-end-to-end"))
		sk, err := crypto.GenerateBLS12381PrivateKey(ikm[:])
		require.NoError(err)
		signer := NewBLSHeaderSigner(sk)
		blk, err := NewBuilder(RunnableActions{}).
			SetHeight(1).
			SetVersion(1).
			SignAndBuild(signer)
		require.NoError(err)
		require.True(blk.Header.VerifySignature(),
			"BLS-signed block round-trips through VerifySignature — proves "+
				"the HeaderSigner abstraction wires into the Verifier path "+
				"correctly for both schemes")
	})
}
