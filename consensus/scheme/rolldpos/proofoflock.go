// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/iotex-core/v2/endorsement"
)

// ProofOfLock carries the endorsements that justify a locked block. Exactly
// one of endorsements or blsEndorsements is populated for any given proof:
// pre-fork the proof rides on secp256k1 endorsements; once BLS aggregation is
// activated (IIP-52) it rides on BLS endorsements.
type ProofOfLock struct {
	endorsements    []*endorsement.Endorsement
	blsEndorsements []*endorsement.BLSEndorsement
}

// NewProofOfLock builds an ECDSA-style proof of lock.
func NewProofOfLock(ens []*endorsement.Endorsement) *ProofOfLock {
	return &ProofOfLock{endorsements: ens}
}

// NewBLSProofOfLock builds a BLS-style proof of lock.
func NewBLSProofOfLock(ens []*endorsement.BLSEndorsement) *ProofOfLock {
	return &ProofOfLock{blsEndorsements: ens}
}

// IsBLS reports whether this proof of lock carries BLS endorsements.
func (p *ProofOfLock) IsBLS() bool {
	return p != nil && len(p.blsEndorsements) > 0
}

// Endorsements returns the secp256k1 endorsements; nil for the BLS variant.
func (p *ProofOfLock) Endorsements() []*endorsement.Endorsement {
	if p == nil {
		return nil
	}
	return p.endorsements
}

// BLSEndorsements returns the BLS endorsements; nil for the ECDSA variant.
func (p *ProofOfLock) BLSEndorsements() []*endorsement.BLSEndorsement {
	if p == nil {
		return nil
	}
	return p.blsEndorsements
}

// Len returns the number of endorsements regardless of signature scheme.
func (p *ProofOfLock) Len() int {
	if p == nil {
		return 0
	}
	return len(p.endorsements) + len(p.blsEndorsements)
}
