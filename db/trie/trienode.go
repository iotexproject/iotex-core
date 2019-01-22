// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

type (
	keyType []byte

	// NodeType is the type of a trie node
	NodeType int
)

const (
	// HASH is a node storing the hash
	HASH NodeType = iota + 1
	// BRANCH is an internal node type of length 1 and multiple child
	BRANCH
	// LEAF is a leaf node with value
	LEAF
	// EXTENSION is a spefic type of branch with only one child
	EXTENSION
)

// Node defines the interface of a trie node
// Note: all the key-value pairs should be of the same length of keys
type Node interface {
	// Children returns the list of children
	Children() []Node

	hash(Trie, bool) ([]byte, error)
	search(Trie, keyType, uint8) (Node, error)
	delete(Trie, keyType, uint8) (Node, error)
	upsert(Trie, keyType, uint8, []byte) (Node, error)

	isDirty() bool
}

// LeafNode defines the interface of a trie leaf node
type LeafNode interface {
	Key() []byte
	Value() []byte
}

// HashNode defines the interface of a hashed node
type HashNode interface {
	Hash() []byte
}

type nodeCache struct {
	hn    *hashNode
	dirty bool
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
