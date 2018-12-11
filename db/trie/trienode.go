// Copyright (c) 2018 IoTeX
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
	// BRANCH is an internal node type of length 1 and multiple child
	BRANCH NodeType = iota + 1
	// LEAF is a leaf node with value
	LEAF
	// EXTENSION is a spefic type of branch with only one child
	EXTENSION
)

// Node defines the interface of a trie node
// Note: all the key-value pairs should be of the same length of keys
type Node interface {
	// Type returns the type of a node
	Type() NodeType
	// Key returns the key of a node, only leaf has key
	Key() []byte
	// Value returns the value of a node, only leaf has value
	Value() []byte

	children(Trie) ([]Node, error)
	search(Trie, keyType, uint8) Node
	delete(Trie, keyType, uint8) (Node, error)
	upsert(Trie, keyType, uint8, []byte) (Node, error)

	serialize() []byte
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
