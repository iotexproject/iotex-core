// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
)

// blsKeyForTest deterministically derives a BLS keypair from a seed so
// the tests are reproducible.
func blsKeyForTest(t *testing.T, seed string) *crypto.BLS12381PrivateKey {
	t.Helper()
	h := sha256.Sum256([]byte(seed))
	sk, err := crypto.GenerateBLS12381PrivateKey(h[:])
	require.NoError(t, err)
	return sk
}

func addrForTest(t *testing.T, seed string) address.Address {
	t.Helper()
	h := sha256.Sum256([]byte(seed))
	addr, err := address.FromBytes(h[:20])
	require.NoError(t, err)
	return addr
}

func TestBLSPop_RoundTrip(t *testing.T) {
	require := require.New(t)
	sk := blsKeyForTest(t, "honest-delegate")
	owner := addrForTest(t, "owner-address")

	pop, err := SignBLSPop(sk, owner)
	require.NoError(err)
	require.Len(pop, crypto.BLSAggregateSignatureLength)

	require.NoError(VerifyBLSPop(sk.PublicKey().Bytes(), pop, owner))
}

func TestBLSPop_RejectInvalidLength(t *testing.T) {
	require := require.New(t)
	sk := blsKeyForTest(t, "x")
	owner := addrForTest(t, "owner")

	require.Error(VerifyBLSPop([]byte{0x00}, make([]byte, 96), owner),
		"short BLS pubkey rejected")
	require.Error(VerifyBLSPop(sk.PublicKey().Bytes(), []byte{0x00}, owner),
		"short PoP rejected")
}

func TestBLSPop_RejectWrongCandidateID(t *testing.T) {
	// A PoP issued for one candidate must not verify under a different
	// candidate ID — closes the replay window where an attacker
	// repackages a CandidateRegister tx with a different identity.
	require := require.New(t)
	sk := blsKeyForTest(t, "delegate")
	candA := addrForTest(t, "candidate-A")
	candB := addrForTest(t, "candidate-B")

	pop, err := SignBLSPop(sk, candA)
	require.NoError(err)
	require.NoError(VerifyBLSPop(sk.PublicKey().Bytes(), pop, candA),
		"sanity: PoP verifies for the candidate it was signed for")
	require.Error(VerifyBLSPop(sk.PublicKey().Bytes(), pop, candB),
		"PoP must NOT verify under a different candidate ID")
}

// TestBLSPop_StableAcrossOwnershipTransfer locks in the property that
// motivated switching update-path PoP binding from c.Owner to
// c.GetIdentifier(): a PoP signed at registration (bound to the
// original owner = future identifier in the non-collision case) MUST
// still verify when the same identifier is the binding at update time
// — even if the candidate's current owner has changed via
// CandidateTransferOwnership.
//
// The test models this by signing once with candidateID = original
// owner, then verifying with the same identifier even though the
// "current owner" in the surrounding state (not modeled here, but
// implicit) would be different.
func TestBLSPop_StableAcrossOwnershipTransfer(t *testing.T) {
	require := require.New(t)
	sk := blsKeyForTest(t, "delegate")
	originalOwner := addrForTest(t, "original-owner")

	// Sign once at registration time, binding to the original owner.
	// For post-Xingu non-collision candidates this becomes c.Identifier
	// verbatim (generateCandidateID returns owner when free).
	pop, err := SignBLSPop(sk, originalOwner)
	require.NoError(err)

	// Later, the candidate is transferred (originalOwner → newOwner) and
	// the same delegate submits a BLS-related update. The handler now
	// passes c.GetIdentifier() — which is still originalOwner — to
	// VerifyBLSPop. The same PoP must still validate.
	require.NoError(VerifyBLSPop(sk.PublicKey().Bytes(), pop, originalOwner),
		"PoP signed under the original owner / identifier must validate "+
			"unchanged when the candidate's owner has been transferred")
}

func TestBLSPop_RejectWrongPubkey(t *testing.T) {
	// A PoP for one BLS pubkey must not verify against another — closes
	// the case where an attacker steals a PoP from someone else's tx and
	// pairs it with their own pubkey.
	require := require.New(t)
	skA := blsKeyForTest(t, "delegate-A")
	skB := blsKeyForTest(t, "delegate-B")
	owner := addrForTest(t, "owner")

	popA, err := SignBLSPop(skA, owner)
	require.NoError(err)
	require.Error(VerifyBLSPop(skB.PublicKey().Bytes(), popA, owner),
		"PoP for pubkey A must NOT verify against pubkey B")
}

// TestBLSPop_RogueKeyAttackBlocked is the security regression guard for
// the BLS rogue-key aggregate forgery against IIP-52's
// FastAggregateVerify path.
//
// Threat model: an attacker reads N honest delegates' BLS public keys
// from chain. They pick a private key x they fully control and would
// like to register
//
//	pk_rogue = g^x − Σ(other delegates' pubkeys)
//
// in G1. If accepted, the rogue pubkey causes
// FastAggregateVerify(all_pubkeys, σ, msg) to collapse the aggregated
// pubkey to g^x, letting one delegate forge a quorum certificate with
// a single signature.
//
// Constructing pk_rogue requires only public information. What does
// NOT require only public information is knowing its discrete log —
// pk_rogue's secret key is (x − Σ sk_i), and the attacker does not
// know any sk_i.
//
// The PoP mitigation: registration requires a BLS signature over
// BLSPopSigningRoot(candidateID) that verifies under pk_rogue.
// Producing this signature requires pk_rogue's secret key — the
// pairing-based Verify(pk_rogue, msg, sig) only accepts a signature
// produced with sk_rogue. The attacker has no such key.
//
// The cleanest test of this property is the abstract one: any pubkey
// for which the actor does not know the secret cannot be the subject
// of a valid PoP — regardless of how the pubkey was constructed. We
// model "the attacker tries to register some pubkey they do not own"
// by using a freshly generated pubkey as pk_rogue and verifying that
// no PoP signed by any other secret key (whether the attacker's or
// anyone else's) validates against it.
func TestBLSPop_RogueKeyAttackBlocked(t *testing.T) {
	require := require.New(t)

	// 1. N=24 honest delegates' pubkeys, simulating the on-chain active
	//    set the attacker reads. The exact bytes are not assertion-load-
	//    bearing — they only need to be distinct, valid BLS pubkeys.
	const N = 24
	honestPubKeys := make([][]byte, N)
	for i := 0; i < N; i++ {
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(i))
		sk := blsKeyForTest(t, "honest-"+string(buf))
		honestPubKeys[i] = sk.PublicKey().Bytes()
	}

	// 2. The attacker's secret. They can sign anything with this, but
	//    not under any other key's pubkey.
	attackerSK := blsKeyForTest(t, "attacker-secret")

	// 3. pk_rogue stand-in: a freshly generated pubkey whose secret the
	//    attacker does NOT have. (In the wild, this would be the
	//    explicit g^x − Σ pk_i subtraction; for PoP-verification
	//    purposes the relevant property is identical — pk for which
	//    attacker has no sk.)
	pkRogue := blsKeyForTest(t, "rogue-target-keypair").PublicKey().Bytes()
	rogueOwner := addrForTest(t, "rogue-owner")

	// 4. Attacker attempts to register pkRogue. The only PoP they can
	//    produce is one signed with attackerSK over the canonical
	//    signing root for the rogue candidate. VerifyBLSPop checks the
	//    signature under pkRogue (the pubkey the attacker is trying to
	//    register), and the pairing check requires the signer to be
	//    pkRogue's secret holder — which the attacker is not.
	attackerForgedPop, err := attackerSK.Sign(BLSPopSigningRoot(rogueOwner))
	require.NoError(err)
	require.Error(VerifyBLSPop(pkRogue, attackerForgedPop, rogueOwner),
		"PoP signed under attackerSK must NOT validate as possession of pkRogue. "+
			"This is the exact property that blocks the rogue-key registration")

	// 5. Also reject if the attacker simply pairs the pubkey with a
	//    blank signature — defensive cover for an actor who skips the
	//    sign step entirely.
	require.Error(VerifyBLSPop(pkRogue, make([]byte, crypto.BLSAggregateSignatureLength), rogueOwner),
		"all-zeros signature must not validate")

	// 6. Control: a delegate that DOES know their key's secret can
	//    produce a valid PoP — the gate is not blanket-rejecting BLS
	//    registrations, only those without possession.
	legitSK := blsKeyForTest(t, "honest-registrant")
	legitPop, err := SignBLSPop(legitSK, rogueOwner)
	require.NoError(err)
	require.NoError(VerifyBLSPop(legitSK.PublicKey().Bytes(), legitPop, rogueOwner),
		"control: a delegate that knows their own secret can register normally")
}

// TestBLSPop_RejectNilCandidateID locks in the contract that the three
// PoP entry points refuse to operate without a candidate-identity
// binding. Allowing nil candidateID to silently fall through would
// collapse the scheme to a single domain-only digest that every
// signer would attest over — re-opening the same-message aggregation
// attack the candidateID binding exists to block.
func TestBLSPop_RejectNilCandidateID(t *testing.T) {
	require := require.New(t)
	sk := blsKeyForTest(t, "any-delegate")
	pk := sk.PublicKey().Bytes()

	// BLSPopSigningRoot returns nil.
	require.Nil(BLSPopSigningRoot(nil),
		"signing root with nil candidateID must be nil — refuse to produce an unbound digest")

	// SignBLSPop returns an error.
	_, err := SignBLSPop(sk, nil)
	require.Error(err)
	require.Contains(err.Error(), "nil candidate ID")

	// VerifyBLSPop returns an error even before any cryptographic work.
	require.Error(
		VerifyBLSPop(pk, make([]byte, crypto.BLSAggregateSignatureLength), nil),
		"verifier must reject nil candidateID without dispatching the BLS pairing")
}
