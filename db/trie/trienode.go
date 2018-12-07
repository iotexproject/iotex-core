// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

// EmptyBranchNodeHash is the root hash of an empty trie, which is the hash of an empty branch node
var EmptyBranchNodeHash = nodeHash(newEmptyBranchNode())

type keyType []byte

// NodeType is the type of a trie node
type NodeType int

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

	children(SameKeyLenTrieContext) ([]Node, error)
	search(SameKeyLenTrieContext, keyType, uint8) Node
	delete(SameKeyLenTrieContext, keyType, uint8) (Node, error)
	upsert(SameKeyLenTrieContext, keyType, uint8, []byte) (Node, error)

	serialize() []byte
}

func nodeHash(tn Node) hash.Hash32B {
	if tn == nil {
		panic("unexpected nil node to hash")
	}

	return blake2b.Sum256(tn.serialize())
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
