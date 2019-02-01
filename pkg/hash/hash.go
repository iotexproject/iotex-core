// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hash

import "golang.org/x/crypto/blake2b"

const (
	// HashSize defines the size of hash
	HashSize = 32
	// PKHashSize defines the size of public-key hash
	PKHashSize = 20
	// CacheHashSize defines the size of local hash key
	CacheHashSize = 20
)

var (
	// ZeroHash32B is 32-bytes of all zero
	ZeroHash32B = Hash32B{}
	// ZeroPKHash is 20-bytes of all zero
	ZeroPKHash = PKHash{}
)

type (
	// Hash32B is 32-byte hash
	Hash32B [HashSize]byte
	// PKHash for account and smart contract address hash
	PKHash [PKHashSize]byte
	// CacheHash for 20-byte hash used in cache
	CacheHash [CacheHashSize]byte
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
func (h *Hash32B) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashSize:]
	}
	copy(h[HashSize-len(b):], b)
}
