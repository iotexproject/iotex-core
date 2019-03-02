// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"github.com/iotexproject/iotex-core/pkg/hash"
)

// Merkle tree struct
type Merkle struct {
	root hash.Hash256
	leaf []hash.Hash256
	size int
}

// NewMerkleTree creates a merkle tree given hashed leaves
func NewMerkleTree(leaves []hash.Hash256) *Merkle {
	size := len(leaves)
	if size == 0 {
		return nil
	}

	mk := &Merkle{
		leaf: make([]hash.Hash256, (size+1)>>1<<1),
		size: size,
	}

	copy(mk.leaf, leaves)

	if size == 1 {
		mk.root = mk.leaf[0]
		return mk
	}

	// copy the last hash if original size is odd number
	if size != len(mk.leaf) {
		mk.leaf[size] = mk.leaf[size-1]
		mk.size = len(mk.leaf)
	}

	return mk
}

// HashTree calculates the root hash of a merkle tree
func (mk *Merkle) HashTree() hash.Hash256 {
	if mk.root != hash.ZeroHash256 {
		return mk.root
	}

	length := mk.size >> 1
	merkle := make([]hash.Hash256, length)

	// first round, compute hash from original leaf
	for i := 0; i < length; i++ {
		h := mk.leaf[i<<1][:]
		h = append(h, mk.leaf[i<<1+1][:]...)
		merkle[i] = hash.Hash256b(h)
	}

	for length > 1 {
		if length&1 != 0 {
			merkle = append(merkle, merkle[length-1])
			length++
		}

		length >>= 1
		for i := 0; i < length; i++ {
			h := merkle[i<<1][:]
			h = append(h, merkle[i<<1+1][:]...)
			merkle[i] = hash.Hash256b(h)
		}
		merkle = merkle[0:length]
	}

	mk.root = merkle[0]
	return mk.root
}
