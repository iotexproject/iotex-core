// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package pqcrypto verifies post-quantum signatures for consensus code.
//
// It holds no private keys and never signs; the wallet side lives in
// pqcrypto/signer, which consensus code must not import. The package is
// domain-neutral: callers pass their own signing context and build their own
// messages (see Transcript).
package pqcrypto

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cloudflare/circl/sign/slhdsa"
)

// AlgID identifies a post-quantum signature algorithm (IIP-64 §4.1.3).
type AlgID uint32

// AlgSLHDSA192s is SLH-DSA-SHA2-192s (NIST FIPS 205), pure interface.
const AlgSLHDSA192s AlgID = 0x12

var (
	// ErrUnsupportedAlg is returned for an algorithm ID this build does not know.
	ErrUnsupportedAlg = errors.New("unsupported post-quantum algorithm")
	// ErrInvalidPublicKey is returned for a malformed public key.
	ErrInvalidPublicKey = errors.New("invalid post-quantum public key")
)

type algSpec struct {
	name    string
	pkSize  int
	sigSize int
	circlID slhdsa.ID
}

// specs is read-only after init and is only ever indexed, never iterated.
var specs = map[AlgID]algSpec{
	AlgSLHDSA192s: {name: "SLH-DSA-SHA2-192s", pkSize: 48, sigSize: 16224, circlID: slhdsa.SHA2_192s},
}

// Supported reports whether this build can verify signatures for a.
func (a AlgID) Supported() bool {
	_, ok := specs[a]
	return ok
}

// String returns the algorithm name, or a hex tag for unknown IDs.
func (a AlgID) String() string {
	if s, ok := specs[a]; ok {
		return s.name
	}
	return fmt.Sprintf("alg(0x%x)", uint32(a))
}

// PublicKeySize returns the serialized public key size, or 0 if unsupported.
func (a AlgID) PublicKeySize() int { return specs[a].pkSize }

// SignatureSize returns the serialized signature size, or 0 if unsupported.
func (a AlgID) SignatureSize() int { return specs[a].sigSize }

// PublicKey is an immutable, parsed post-quantum public key.
type PublicKey struct {
	alg AlgID
	raw []byte
	key slhdsa.PublicKey
}

// ParsePublicKey parses b as a public key of algorithm alg. The parameter set
// is always taken from alg, never inferred from the key bytes.
func ParsePublicKey(alg AlgID, b []byte) (*PublicKey, error) {
	s, ok := specs[alg]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlg, alg)
	}
	if len(b) != s.pkSize {
		return nil, fmt.Errorf("%w: %s key must be %d bytes, got %d", ErrInvalidPublicKey, alg, s.pkSize, len(b))
	}
	pk := &PublicKey{alg: alg, raw: bytes.Clone(b)}
	pk.key.ID = s.circlID
	if err := pk.key.UnmarshalBinary(pk.raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPublicKey, err)
	}
	return pk, nil
}

// Alg returns the key's algorithm.
func (pk *PublicKey) Alg() AlgID { return pk.alg }

// Bytes returns a copy of the serialized key.
func (pk *PublicKey) Bytes() []byte { return bytes.Clone(pk.raw) }

// Equal reports whether pk and other are the same algorithm and key.
func (pk *PublicKey) Equal(other *PublicKey) bool {
	return other != nil && pk.alg == other.alg && bytes.Equal(pk.raw, other.raw)
}

// Verify reports whether sig is a valid pure-mode signature over msg under the
// given signing context. Wrong-size signatures and contexts longer than 255
// bytes are rejected.
func (pk *PublicKey) Verify(context, msg, sig []byte) bool {
	if len(sig) != specs[pk.alg].sigSize || len(context) > 255 {
		return false
	}
	return slhdsa.Verify(&pk.key, slhdsa.NewMessage(msg), sig, context)
}
