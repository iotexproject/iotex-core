// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/pkg/enc"
)

var (
	// CryptoSeed is a hardcoded seed that will be replaced by a seed produced dynamically.
	CryptoSeed = []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}
)

// Sort sorts a given slices of hashes cryptographically using hash function
func Sort(hashes [][]byte, nonce uint64) {
	nb := make([]byte, 8)
	enc.MachineEndian.PutUint64(nb, nonce)

	hashMap := make(map[string]hash.Hash256)
	for _, h := range hashes {
		hash256 := hash.Hash256b(append(append(h, CryptoSeed...), nb...))
		hashMap[string(h)] = hash256
	}

	sort.Slice(hashes, func(i, j int) bool {
		hi := hashMap[string(hashes[i])]
		hj := hashMap[string(hashes[j])]
		return bytes.Compare(hi[:], hj[:]) < 0
	})
}

// SortCandidates sorts a given slices of hashes cryptographically using hash function
func SortCandidates(candidates []string, epochNum uint64, cryptoSeed []byte) {
	nb := make([]byte, 8)
	enc.MachineEndian.PutUint64(nb, epochNum)

	hashMap := make(map[string]hash.Hash256)
	for _, cand := range candidates {
		hash256 := hash.Hash256b(append(append([]byte(cand), cryptoSeed...), nb...))
		hashMap[cand] = hash256
	}

	sort.Slice(candidates, func(i, j int) bool {
		hi := hashMap[candidates[i]]
		hj := hashMap[candidates[j]]
		return bytes.Compare(hi[:], hj[:]) < 0
	})
}
