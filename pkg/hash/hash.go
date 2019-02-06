// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hash

import "golang.org/x/crypto/blake2b"

var (
	// ZeroHash256 is 256-bit of all zero
	ZeroHash256 = Hash256{}
	// ZeroHash160 is 160-bit of all zero
	ZeroHash160 = Hash160{}
)

type (
	// Hash256 is 256-bit hash
	Hash256 [32]byte
	// Hash160 for 160-bit hash used for account and smart contract address
	Hash160 [20]byte
	// CacheHash for 160-bit hash used in cache
	CacheHash [20]byte
)

// Hash160b returns 160-bit (20-byte) hash of input
func Hash160b(input []byte) []byte {
	// use Blake2b algorithm
	digest := blake2b.Sum256(input)
	return digest[7:27]
}

// Hash256b returns 256-bit (32-byte) hash of input
func Hash256b(input []byte) []byte {
	// use Blake2b algorithm
	digest := blake2b.Sum256(input)
	return digest[:]
}

// SetBytes copies the byte slice into hash
func (h *Hash256) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-32:]
	}
	copy(h[32-len(b):], b)
}
