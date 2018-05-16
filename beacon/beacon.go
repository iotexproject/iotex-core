// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package beacon

import (
	"hash"

	"github.com/golang/glog"
	"golang.org/x/crypto/blake2b"
)

const (
	startSeed = "9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567"
)

// Beacon contains a seed which can be updated every epoch
type Beacon struct {
	seed []byte
	hash hash.Hash
}

// NewBeacon creates new beacon with initial string
func NewBeacon() (Beacon, error) {
	hash, err := blake2b.New(64, nil)
	b := Beacon{seed: []byte(startSeed), hash: hash}

	if err != nil {
		glog.Error("Beacon hash function failed to initialize")
	}

	return b, err
}

// GetSeed returns the current seed of the beacon
func (b *Beacon) GetSeed() []byte {
	return b.seed
}

// NextEpoch advances the beacon to the next epoch
func (b *Beacon) NextEpoch() {
	b.hash.Reset()
	b.hash.Write(b.seed)
	b.seed = b.hash.Sum(nil)
}
