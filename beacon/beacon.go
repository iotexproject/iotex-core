// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package beacon

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

// Beacon contains a seed which can be updated every epoch
type Beacon struct {
	Seed string
	Hash hash.Hash
}

// NewBeacon creates new beacon with initial string
func NewBeacon(seed string) Beacon {
	hash := sha256.New()
	b := Beacon{Seed: seed, Hash: hash}

	return b
}

// GetSeed returns the current seed of the beacon
func (b *Beacon) GetSeed() string {
	return b.Seed
}

// NextEpoch advances the beacon to the next epoch
func (b *Beacon) NextEpoch() {
	b.Hash.Reset()
	b.Hash.Write([]byte(b.Seed))
	b.Seed = hex.EncodeToString(b.Hash.Sum(nil))
}
