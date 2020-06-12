// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

type (
	keyType []byte
)

type node interface {
	search(keyType, uint8) node
	delete(keyType, uint8) (node, error)
	upsert(keyType, uint8, []byte) (node, error)

	serialize() []byte
}

type leaf interface {
	node
	// Key returns the key of a node, only leaf has key
	Key() []byte
	// Value returns the value of a node, only leaf has value
	Value() []byte
}

type extension interface {
	node
	child() (node, error)
}

type branch interface {
	node
	children() ([]node, error)
	markAsRoot()
}

// key1 should not be longer than key2
func commonPrefixLength(key1, key2 []byte) uint8 {
	match := uint8(0)
	len1 := uint8(len(key1))
	for match < len1 && key1[match] == key2[match] {
		match++
	}

	return match
}
