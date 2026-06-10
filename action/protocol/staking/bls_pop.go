// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"crypto/sha256"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
)

// blsPopDomain is the iotex-specific domain separator for BLS
// proof-of-possession signatures at candidate register / update time.
//
// Domain separation matters for two reasons:
//   1. It prevents a PoP signature from being replayed as a consensus
//      vote signature (or vice versa) — even though both schemes use
//      the same BLS ciphersuite DST, the message they sign starts with
//      this iotex-application-level tag and so the resulting signing
//      root will never collide with a consensus signing root.
//   2. The version suffix ("v1") reserves room for a future fork to
//      rotate the PoP scheme without ambiguity.
const blsPopDomain = "IOTEX_BLS_POP_v1"

// BLSPopSigningRoot returns the bytes that a BLS proof-of-possession
// must be computed over for the given candidate.
//
// Binding three values into the signed message — the domain tag, the BLS
// public key itself, and the candidate owner address — closes the rogue
// key attack and two related replays:
//
//   - blsPubKey: forces the signer to know the private key for THIS
//     specific BLS pubkey. A rogue pubkey constructed as
//     g^x − Σ(other pubkeys) cannot produce a valid PoP because the
//     attacker does not know its discrete log.
//   - ownerAddress: prevents two distinct candidates from sharing a
//     single BLS keypair (and thus a single PoP) without each owner
//     independently re-attesting; also prevents a PoP submitted for
//     candidate A from being replayed for candidate B by a man-in-the-
//     middle who repackages a CandidateRegister tx.
//   - blsPopDomain: keeps PoP signatures disjoint from consensus
//     signatures, future PoP schemes, and any other BLS-signed iotex
//     message that may exist or be added later.
func BLSPopSigningRoot(blsPubKey []byte, ownerAddress address.Address) []byte {
	h := sha256.New()
	h.Write([]byte(blsPopDomain))
	h.Write(blsPubKey)
	if ownerAddress != nil {
		h.Write(ownerAddress.Bytes())
	}
	return h.Sum(nil)
}

// SignBLSPop produces a proof-of-possession for the given BLS private
// key, binding it to the candidate owner address. Used by tooling
// (ioctl, SDK) to generate the bls_pop field on CandidateRegister /
// CandidateUpdate transactions.
func SignBLSPop(sk *crypto.BLS12381PrivateKey, ownerAddress address.Address) ([]byte, error) {
	if sk == nil {
		return nil, errors.New("nil BLS private key")
	}
	pk := sk.PublicKey().Bytes()
	return sk.Sign(BLSPopSigningRoot(pk, ownerAddress))
}

// VerifyBLSPop verifies the proof-of-possession against the provided
// pubkey and owner. Returns nil on success.
func VerifyBLSPop(blsPubKey, blsPop []byte, ownerAddress address.Address) error {
	if len(blsPubKey) != crypto.BLSPubkeyLength {
		return errors.Errorf("invalid BLS pubkey length: got %d, want %d", len(blsPubKey), crypto.BLSPubkeyLength)
	}
	if len(blsPop) != crypto.BLSAggregateSignatureLength {
		return errors.Errorf("invalid BLS PoP length: got %d, want %d", len(blsPop), crypto.BLSAggregateSignatureLength)
	}
	pk, err := crypto.BLS12381PublicKeyFromBytes(blsPubKey)
	if err != nil {
		return errors.Wrap(err, "invalid BLS pubkey")
	}
	if !pk.Verify(BLSPopSigningRoot(blsPubKey, ownerAddress), blsPop) {
		return errors.New("BLS proof-of-possession verification failed")
	}
	return nil
}
