// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/iotexproject/go-pkgs/crypto"
)

// HeaderSigner is the minimal contract for producing a block header's
// signature. Two production schemes satisfy it via thin adapters that
// wrap their respective private-key types: secp256k1 (ECDSAHeaderSigner,
// the pre-fork path) and BLS12-381 (BLSHeaderSigner, post-fork under
// the BLS Producer Identity follow-up to IIP-52).
//
// Builder.SignAndBuild takes a HeaderSigner so the choice of scheme is
// the caller's responsibility — the block package does not need to know
// which fork is active.
type HeaderSigner interface {
	// PubKey returns the corresponding public key as a Verifier. The
	// returned value is the same concrete type that Header.pubkey will
	// carry post-signing, so VerifySignature operates uniformly across
	// schemes.
	PubKey() Verifier
	// Sign returns the signature over msg using the signer's scheme.
	Sign(msg []byte) ([]byte, error)
}

// ECDSAHeaderSigner adapts a secp256k1 crypto.PrivateKey to HeaderSigner.
// Used pre-fork (and for any historical block replay) where headers are
// signed with the producer's secp256k1 key.
type ECDSAHeaderSigner struct {
	sk crypto.PrivateKey
}

// NewECDSAHeaderSigner wraps a secp256k1 private key for header signing.
// nil is permitted to ease test plumbing; Builder.SignAndBuild rejects
// signers whose PubKey or Sign would panic / return error.
func NewECDSAHeaderSigner(sk crypto.PrivateKey) *ECDSAHeaderSigner {
	return &ECDSAHeaderSigner{sk: sk}
}

// PubKey returns the secp256k1 public key as a Verifier. crypto.PublicKey
// already satisfies the interface (Bytes + Verify), so no extra wrapping
// is needed.
func (s *ECDSAHeaderSigner) PubKey() Verifier {
	return s.sk.PublicKey()
}

// Sign signs msg with the wrapped secp256k1 key.
func (s *ECDSAHeaderSigner) Sign(msg []byte) ([]byte, error) {
	return s.sk.Sign(msg)
}

// BLSHeaderSigner adapts a *crypto.BLS12381PrivateKey to HeaderSigner.
// Used post-fork when the producer signs the header with their BLS key.
type BLSHeaderSigner struct {
	sk *crypto.BLS12381PrivateKey
}

// NewBLSHeaderSigner wraps a BLS12-381 private key for header signing.
func NewBLSHeaderSigner(sk *crypto.BLS12381PrivateKey) *BLSHeaderSigner {
	return &BLSHeaderSigner{sk: sk}
}

// PubKey returns the BLS12-381 public key as a Verifier.
// *crypto.BLS12381PublicKey already satisfies the interface.
func (s *BLSHeaderSigner) PubKey() Verifier {
	return s.sk.PublicKey()
}

// Sign signs msg with the wrapped BLS key.
func (s *BLSHeaderSigner) Sign(msg []byte) ([]byte, error) {
	return s.sk.Sign(msg)
}
