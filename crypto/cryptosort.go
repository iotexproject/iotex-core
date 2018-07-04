// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"sort"

	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/enc"
)

var (
	// cryptoSeed is a hardcoded seed that will be replaced by a seed produced dynamically.
	cryptoSeed = []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}
)

// Sort sorts a given slices of hashes cryptographically using blake2b hash function
func Sort(hashes [][]byte, nonce uint64) error {
	nb := make([]byte, 8)
	enc.MachineEndian.PutUint64(nb, nonce)

	sort.Slice(hashes[:], func(i, j int) bool {
		hi := blake2b.Sum256(append(append(hashes[i], cryptoSeed...), nb...))
		hj := blake2b.Sum256(append(append(hashes[j], cryptoSeed...), nb...))
		return bytes.Compare(hi[:], hj[:]) < 0
	})

	return nil
}
