// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
)

// FuzzMerkleHashTree asserts that constructing a Merkle tree from arbitrary
// leaf bytes and computing its root never panics. The leaf-count parity
// branch (odd → duplicate last) is exercised by feeding non-multiple-of-32
// inputs as well.
func FuzzMerkleHashTree(f *testing.F) {
	f.Add([]byte{})                       // zero leaves
	f.Add(make([]byte, 32))               // 1 leaf, zero hash (regression for fixed root-sentinel bug)
	f.Add(append(make([]byte, 31), 0x01)) // 1 leaf, non-zero
	f.Add(make([]byte, 64))               // 2 leaves
	f.Add(make([]byte, 96))               // 3 leaves (odd; duplicate path)
	f.Add(make([]byte, 32*10))            // 10 leaves

	f.Fuzz(func(t *testing.T, data []byte) {
		n := len(data) / 32
		leaves := make([]hash.Hash256, n)
		for i := range n {
			copy(leaves[i][:], data[i*32:(i+1)*32])
		}
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewMerkleTree/HashTree panicked on %d leaves: %v", n, r)
			}
		}()
		if m := NewMerkleTree(leaves); m != nil {
			_ = m.HashTree()
		}
	})
}

// FuzzSort asserts that crypto.Sort never panics on arbitrary inputs.
func FuzzSort(f *testing.F) {
	f.Add([]byte{}, uint64(0))
	f.Add(make([]byte, 32), uint64(0))
	f.Add(make([]byte, 64), uint64(1))
	f.Add(make([]byte, 32*5), ^uint64(0))

	f.Fuzz(func(t *testing.T, data []byte, nonce uint64) {
		n := len(data) / 32
		hashes := make([][]byte, n)
		for i := range n {
			hashes[i] = make([]byte, 32)
			copy(hashes[i], data[i*32:(i+1)*32])
		}
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Sort panicked on %d hashes (nonce=%d): %v", n, nonce, r)
			}
		}()
		Sort(hashes, nonce)
	})
}

// FuzzSortCandidates asserts that crypto.SortCandidates never panics on
// arbitrary candidate strings, epoch numbers, or seed lengths.
func FuzzSortCandidates(f *testing.F) {
	f.Add("", uint64(0), []byte{})
	f.Add("alice", uint64(1), []byte{0x01, 0x02})
	f.Add("io1aaaaaaaaaaa", uint64(42), CryptoSeed)

	f.Fuzz(func(t *testing.T, c1 string, epoch uint64, seed []byte) {
		cands := []string{c1}
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("SortCandidates panicked (epoch=%d, seedLen=%d): %v", epoch, len(seed), r)
			}
		}()
		SortCandidates(cands, epoch, seed)
	})
}
