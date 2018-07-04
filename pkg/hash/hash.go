// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hash

const (
	// HashSize defines the size of hash
	HashSize = 32
	// PKHashSize defines the size of public-key hash
	PKHashSize = 20
	// DKGHashSize defines the size of a DKG hash
	DKGHashSize = 20
)

var (
	// ZeroHash32B is 32-bytes of all zero
	ZeroHash32B = Hash32B{}
)

type (
	// Hash32B is 32-byte hash
	Hash32B [HashSize]byte
	// PKHash is 20-byte hash
	PKHash [PKHashSize]byte
	// DKGHash is 20-byte hash
	DKGHash [DKGHashSize]byte
)
