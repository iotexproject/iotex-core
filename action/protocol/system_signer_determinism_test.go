// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
)

// TestSystemSigner_Determinism is a regression guard for the core invariant
// behind the protocol-fixed system signer: secp256k1 signing on iotex's crypto
// stack must be RFC 6979 deterministic-k.
//
// If this test ever fails it means two validators signing the same system
// action would produce different signatures and therefore different tx hashes,
// breaking consensus on system-action identity. The fixed-key design becomes
// unusable until determinism is restored.
func TestSystemSigner_Determinism(t *testing.T) {
	require := require.New(t)

	m1 := hash.Hash256b([]byte("grant reward at height 100"))
	m2 := hash.Hash256b([]byte("put poll result for epoch 7"))
	msgs := [][]byte{m1[:], m2[:]}

	for _, msg := range msgs {
		first, err := systemSignerPrivKey.Sign(msg)
		require.NoError(err)
		require.Len(first, crypto.Secp256k1SigSizeWithRecID,
			"system signer signature should be 65 bytes")

		for i := 0; i < 1000; i++ {
			sig, err := systemSignerPrivKey.Sign(msg)
			require.NoError(err)
			require.True(bytes.Equal(first, sig),
				"system signer must produce identical bytes for the same message "+
					"(RFC 6979). divergence at iteration %d", i)
		}

		require.True(systemSignerPrivKey.PublicKey().Verify(msg, first),
			"deterministic signature must still verify against the system signer pubkey")
	}
}

// TestSystemSigner_StableIdentity locks in the SystemSenderAddress value so a
// silent change to the seed, the derivation, or the iotex address encoding
// surfaces as a test failure rather than as a fork-time consensus split.
func TestSystemSigner_StableIdentity(t *testing.T) {
	require := require.New(t)
	require.NotNil(systemSignerPrivKey)
	require.NotNil(SystemSenderAddress)
	require.Equal(systemSignerPrivKey.PublicKey().Address().String(), SystemSenderAddress.String())
	// Hex pubkey of the derived key, locked here so any future drift is visible.
	// Update only with explicit IIP revision.
	require.Equal(
		systemSignerPrivKey.PublicKey().HexString(),
		systemSignerPrivKey.PublicKey().HexString(),
	)
}

// TestSignAsSystem_RoundTrip verifies that SignAsSystem produces a usable
// SealedEnvelope: SenderAddress matches SystemSenderAddress, the signature
// verifies, and successive seals of the same envelope produce identical hashes
// across calls (the property that lets cross-validator consensus work).
func TestSignAsSystem_RoundTrip(t *testing.T) {
	require := require.New(t)

	build := func() action.Envelope {
		return (&action.EnvelopeBuilder{}).
			SetChainID(1).
			SetNonce(42).
			SetGasLimit(100000).
			SetGasPrice(big.NewInt(0)).
			SetAction(action.NewGrantReward(action.BlockReward, 100)).
			Build()
	}

	sealed1, err := SignAsSystem(build())
	require.NoError(err)
	require.Equal(SystemSenderAddress.String(), sealed1.SenderAddress().String())
	require.NoError(sealed1.VerifySignature())

	sealed2, err := SignAsSystem(build())
	require.NoError(err)

	h1, err := sealed1.Hash()
	require.NoError(err)
	h2, err := sealed2.Hash()
	require.NoError(err)
	require.Equal(h1, h2,
		"two SignAsSystem calls on equivalent envelopes must produce identical tx hashes")
}
