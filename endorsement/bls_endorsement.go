// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
)

// EndorseBLS signs the document with a BLS12-381 private key and returns a
// standard Endorsement whose signature field carries the 96-byte BLS
// signature. The endorser public key embedded in the endorsement is the
// delegate's existing secp256k1 producer key; receivers derive the iotex
// address from it and resolve the BLS verifying key from candidate state.
//
// Used for consensus votes once BLS signature aggregation is activated
// (IIP-52). The wire format is unchanged from the pre-fork ECDSA path —
// signature length is the discriminator.
func EndorseBLS(
	doc Document,
	ts time.Time,
	endorserPubKey crypto.PublicKey,
	blsSigner *crypto.BLS12381PrivateKey,
) (*Endorsement, error) {
	hash, err := hashDocWithTime(doc, ts)
	if err != nil {
		return nil, err
	}
	sig, err := blsSigner.Sign(hash)
	if err != nil {
		return nil, err
	}
	return NewEndorsement(ts, endorserPubKey, sig), nil
}

// VerifyBLSEndorsement checks an Endorsement that carries a BLS12-381
// signature against the supplied BLS public key. Callers are responsible for
// resolving pubKey from the endorser's iotex address via candidate state.
//
// Use this in place of VerifyEndorsement when the endorsement is known to
// carry a BLS signature (typically branched on signature length:
// len(en.Signature()) == crypto.BLSAggregateSignatureLength).
func VerifyBLSEndorsement(doc Document, en *Endorsement, pubKey *crypto.BLS12381PublicKey) bool {
	if en == nil || pubKey == nil {
		return false
	}
	hash, err := hashDocWithTime(doc, en.Timestamp())
	if err != nil {
		return false
	}
	return pubKey.Verify(hash, en.Signature())
}
