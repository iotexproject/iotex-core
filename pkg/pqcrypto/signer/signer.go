// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package signer is the wallet side of pqcrypto: key construction, wallet-profile
// key derivation, and signing. Consensus code must not import it.
package signer

import (
	"bytes"
	"fmt"

	"github.com/cloudflare/circl/sign/slhdsa"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

func circlID(alg pqcrypto.AlgID) (slhdsa.ID, error) {
	switch alg {
	case pqcrypto.AlgSLHDSA192s:
		return slhdsa.SHA2_192s, nil
	default:
		return 0, fmt.Errorf("%w: %s", pqcrypto.ErrUnsupportedAlg, alg)
	}
}

// SeedSize returns the FIPS 205 keygen seed size, 3n bytes, for alg
// (SK.seed || SK.prf || PK.seed), or 0 if unsupported.
func SeedSize(alg pqcrypto.AlgID) int {
	// FIPS 205: the public key is PK.seed || PK.root, so n = pkSize/2.
	return 3 * alg.PublicKeySize() / 2
}

// PrivateKey is a post-quantum signing key.
type PrivateKey struct {
	alg pqcrypto.AlgID
	key slhdsa.PrivateKey
	pub *pqcrypto.PublicKey
}

// NewKeyFromSeeds runs FIPS 205 slh_keygen_internal (Algorithm 18) on
// seeds = SK.seed || SK.prf || PK.seed.
func NewKeyFromSeeds(alg pqcrypto.AlgID, seeds []byte) (*PrivateKey, error) {
	id, err := circlID(alg)
	if err != nil {
		return nil, err
	}
	if len(seeds) != SeedSize(alg) {
		return nil, fmt.Errorf("%s seeds must be %d bytes, got %d", alg, SeedSize(alg), len(seeds))
	}
	// GenerateKey reads SK.seed, SK.prf and PK.seed from the reader in that
	// order (FIPS 205 Algorithm 21), so a reader over the seeds yields
	// slh_keygen_internal(SK.seed, SK.prf, PK.seed).
	pub, priv, err := slhdsa.GenerateKey(bytes.NewReader(seeds), id)
	if err != nil {
		return nil, err
	}
	raw, err := pub.MarshalBinary()
	if err != nil {
		return nil, err
	}
	pk, err := pqcrypto.ParsePublicKey(alg, raw)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{alg: alg, key: priv, pub: pk}, nil
}

// PublicKey returns the matching public key.
func (k *PrivateKey) PublicKey() *pqcrypto.PublicKey { return k.pub }

// Sign returns a deterministic pure-mode signature over msg under context.
func (k *PrivateKey) Sign(context, msg []byte) ([]byte, error) {
	return slhdsa.SignDeterministic(&k.key, slhdsa.NewMessage(msg), context)
}
