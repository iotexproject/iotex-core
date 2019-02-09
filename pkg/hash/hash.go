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
)

// Hash160b returns 160-bit (20-byte) hash of input
func Hash160b(input []byte) Hash160 {
	// use Blake2b algorithm
	digest := blake2b.Sum256(input)
	var hash Hash160
	copy(hash[:], digest[7:27])
	return hash
}

// Hash256b returns 256-bit (32-byte) hash of input
func Hash256b(input []byte) Hash256 {
	// use Blake2b algorithm
	digest := blake2b.Sum256(input)
	var hash Hash256
	copy(hash[:], digest[:])
	return hash
}

// SetBytes copies the byte slice into hash
func (h *Hash256) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-32:]
	}
	copy(h[32-len(b):], b)
}
