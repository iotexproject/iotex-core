// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"github.com/ethereum/go-ethereum/crypto"
	gocrypto "github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
)

// systemSignerSeed is the input to keccak256 that deterministically derives the
// protocol-fixed ECDSA private key used to sign system actions (GrantReward,
// PutPollResult, ScheduleCandidateDeactivation) once the BLS Producer Identity
// fork is active.
//
// The seed string is a protocol parameter; the .v1 suffix reserves room for a
// future fork to rotate the key by deriving from .v2 etc.
//
// The derived private key is intentionally public. It is not a trust root:
//   - Action pool rejects external submissions whose sender equals
//     SystemSenderAddress (see actpool.add)
//   - Per-handler content rules (reward amount, poll result, deactivation
//     eligibility) enforce action validity independently of the signer
//
// The signature on a system action exists only to satisfy the SealedEnvelope
// wire format and the tx-hash pipeline; it does not authenticate the issuer.
const systemSignerSeed = "iotex.system.signer.v1"

var (
	systemSignerPrivKey gocrypto.PrivateKey
	// SystemSenderAddress is the iotex address derived from the protocol-fixed
	// system signer key. Post-fork it is the canonical Caller for every
	// system-issued action and the canonical SenderAddress on every system
	// action's SealedEnvelope.
	SystemSenderAddress address.Address
)

func init() {
	seed := crypto.Keccak256([]byte(systemSignerSeed))
	sk, err := gocrypto.BytesToPrivateKey(seed)
	if err != nil {
		panic("failed to derive system signer private key: " + err.Error())
	}
	systemSignerPrivKey = sk
	SystemSenderAddress = sk.PublicKey().Address()
}

// SignAsSystem seals an envelope using the protocol-fixed system signer key.
//
// secp256k1 signing under iotex's crypto stack is RFC 6979 deterministic-k, so
// every validator that calls SignAsSystem with the same envelope produces the
// same signature bytes and therefore the same tx hash. This is required for
// system actions to reach cross-validator consensus on their identity.
//
// SignAsSystem is intended for use only inside the state factory when building
// a block; system actions reaching the action pool from any other path are
// rejected on sender match.
func SignAsSystem(envelope action.Envelope) (*action.SealedEnvelope, error) {
	return action.Sign(envelope, systemSignerPrivKey)
}
