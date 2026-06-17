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
// The signed message is the domain tag plus the candidate's stable
// identity. The BLS public key is intentionally NOT in the message:
//
//   - The pairing verifier Verify(PK, msg, sig) already commits PK
//     into the signature equation. An attacker who does not know
//     sk_PK cannot produce a sig that verifies under PK over any
//     message, so the rogue-key registration attack ("register
//     pk_rogue without owning its discrete log") is blocked by basic
//     PoP correctness without needing pubkey-in-message.
//
//   - Cross-candidate replay (PoP for candidate A re-submitted under
//     candidate B) is blocked by the candidateID binding: distinct
//     candidates have distinct signing roots, so the same signature
//     never validates under two candidate identities.
//
//   - Cross-domain replay (e.g. PoP reused as a consensus signature)
//     is blocked by blsPopDomain.
//
//   - The classical same-message aggregation attack on PoP requires
//     two distinct honest signers to sign the *same* signing root. The
//     candidateID binding rules that out: the protocol enforces unique
//     candidate identifiers (see generateCandidateID + the
//     ContainsName / ContainsOwner / ContainsOperator checks at
//     register time), so no two honest delegates ever produce PoPs
//     over the same root.
//
// candidateID is the candidate's stable identity:
//
//   - At register: act.OwnerAddress() (or actCtx.Caller if omitted),
//     which becomes c.Identifier verbatim in the non-collision case
//     via generateCandidateID's owner-first fast path.
//   - At update: c.GetIdentifier(), which returns the immutable
//     Identifier for post-Xingu candidates and falls back to c.Owner
//     for pre-Xingu records. Stable across CandidateTransferOwnership.
func BLSPopSigningRoot(candidateID address.Address) []byte {
	// Refuse to produce an "unbound" root. Allowing nil candidateID to
	// silently fall through to a domain-only digest collapses the
	// scheme to a single global value that every signer would attest
	// over — exactly the shape that re-opens the same-message
	// aggregation attack. Force callers to commit to a candidate
	// identity.
	if candidateID == nil {
		return nil
	}
	h := sha256.New()
	h.Write([]byte(blsPopDomain))
	h.Write(candidateID.Bytes())
	return h.Sum(nil)
}

// SignBLSPop produces a proof-of-possession for the given BLS private
// key, binding it to the candidate's identity. Used by tooling
// (ioctl, SDK) to generate the bls_pop field on CandidateRegister /
// CandidateUpdate transactions.
//
// At registration time pass the proposed owner address (which becomes
// the candidate identifier); at update time pass the candidate's
// existing identifier (c.GetIdentifier()).
func SignBLSPop(sk *crypto.BLS12381PrivateKey, candidateID address.Address) ([]byte, error) {
	if sk == nil {
		return nil, errors.New("nil BLS private key")
	}
	if candidateID == nil {
		return nil, errors.New("nil candidate ID; PoP must bind to a candidate identity")
	}
	return sk.Sign(BLSPopSigningRoot(candidateID))
}

// VerifyBLSPop verifies the proof-of-possession against the provided
// pubkey and candidate identity. Returns nil on success.
func VerifyBLSPop(blsPubKey, blsPop []byte, candidateID address.Address) error {
	if len(blsPubKey) != crypto.BLSPubkeyLength {
		return errors.Errorf("invalid BLS pubkey length: got %d, want %d", len(blsPubKey), crypto.BLSPubkeyLength)
	}
	if len(blsPop) != crypto.BLSAggregateSignatureLength {
		return errors.Errorf("invalid BLS PoP length: got %d, want %d", len(blsPop), crypto.BLSAggregateSignatureLength)
	}
	if candidateID == nil {
		return errors.New("nil candidate ID; PoP must bind to a candidate identity")
	}
	pk, err := crypto.BLS12381PublicKeyFromBytes(blsPubKey)
	if err != nil {
		return errors.Wrap(err, "invalid BLS pubkey")
	}
	if !pk.Verify(BLSPopSigningRoot(candidateID), blsPop) {
		return errors.New("BLS proof-of-possession verification failed")
	}
	return nil
}
