// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

func TestLeafNodeProto(t *testing.T) {
	require := require.New(t)
	lnode := &leafNode{
		key:   []byte("iotex"),
		value: []byte("chain"),
	}
	proto, err := lnode.proto(nil, true)
	require.NoError(err)
	nodepb, ok := proto.(*triepb.NodePb)
	require.True(ok)
	leaf, ok := nodepb.Node.(*triepb.NodePb_Leaf)
	require.True(ok)
	lnode1 := newLeafNodeFromProtoPb(leaf.Leaf, nil)
	require.Equal(lnode.key, lnode1.key)
	require.Equal(lnode.value, lnode1.value)
}

func TestLeafNodeDelete(t *testing.T) {
	var (
		require = require.New(t)
		cli     = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
		}
		key   = keyType("iotex")
		value = []byte("chain")
	)

	lnode, err := newLeafNode(cli, key, value)
	require.NoError(err)

	_, err = lnode.Delete(cli, key, 0)
	require.NoError(err)
	err = lnode.Flush(cli)
	require.NoError(err)
}

func TestLeafNodeUpsert(t *testing.T) {
	var (
		require = require.New(t)
		cli     = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
		}
		items = []struct {
			k keyType
			v []byte
		}{
			{keyType("iotex"), []byte("chain")},
			{keyType("block"), []byte("chain")},
			{keyType("ioabc"), []byte("chabc")},
		}
		offset uint8
		bnode  *branchNode
		ok     bool
	)

	checkChild := func(key keyType, offset uint8, value []byte) {
		child, ok := bnode.children[key[offset]]
		require.True(ok)
		ln1, ok := child.(*leafNode)
		require.True(ok)
		require.Equal(value, ln1.value)
	}

	lnode, err := newLeafNode(cli, keyType("iotex"), []byte("coin"))
	require.NoError(err)

	for i, item := range items {
		node, err := lnode.Upsert(cli, item.k, offset, item.v)
		require.NoError(err)

		switch i {
		case 0:
			newleaf, ok := node.(*leafNode)
			require.True(ok)
			require.Equal(item.k, newleaf.key)
			require.Equal(item.v, newleaf.value)
			continue

		case 1:
			bnode, ok = node.(*branchNode)
			require.True(ok)
			require.Len(bnode.children, 2)
			checkChild(keyType("iotex"), 0, []byte("coin"))
			checkChild(keyType("block"), 0, []byte("chain"))

		case 2:
			enode, ok := node.(*extensionNode)
			require.True(ok)
			require.Equal([]byte("io"), enode.path)
			bnode, ok = enode.child.(*branchNode)
			require.True(ok)
			require.Len(bnode.children, 2)
			checkChild(keyType("iotex"), 2, []byte("coin"))
			checkChild(keyType("ioabc"), 2, []byte("chabc"))
		}
	}
}

func TestLeafBase(t *testing.T) {
	var (
		require = require.New(t)
		cli     = &merklePatriciaTrie{
			hashFunc: DefaultHashFunc,
			kvStore:  trie.NewMemKVStore(),
		}
		key1       = keyType{1, 2, 3, 4, 5}
		expectKeys = []keyType{
			key1,                      // same key
			{1, 2, 5, 6, 7, 8, 9, 10}, // longer length
			{1, 2, 5},                 // shorter length
			{1, 2, 5, 6, 7},           // matched length
		}
	)

	// key is nil
	leaf, err := newLeafNode(cli, keyType{}, []byte("0"))
	require.NoError(err)
	node1, err := leaf.Search(cli, keyType{}, 0)
	require.NoError(err)
	require.Equal([]byte("0"), node1.(*leafNode).value)
	_, err = leaf.Delete(cli, keyType{1, 2, 3}, 0)
	require.Equal(trie.ErrNotExist, err)
	node2, err := leaf.Delete(cli, keyType{}, 0)
	require.NoError(err)
	require.Nil(node2)

	// key is not nil
	for i, expected := range expectKeys {
		leaf, err := newLeafNode(cli, key1, []byte("1"))
		require.NoError(err)
		node, err := leaf.Search(cli, key1, 0)
		require.NoError(err)
		require.Equal([]byte("1"), node.(*leafNode).value)
		// insert two
		node1, err := leaf.Upsert(cli, expected, 0, []byte("2"))
		require.NoError(err)
		if i == 0 {
			// same key
			leaf1, ok := node1.(*leafNode)
			require.True(ok)
			require.Equal([]byte("2"), leaf1.value)
			node2, err := node1.Delete(cli, key1, 0)
			require.NoError(err)
			require.Nil(node2)
			continue
		}
		// different key
		ext1, ok := node1.(*extensionNode)
		require.True(ok)
		node3, err := ext1.Search(cli, expected, 0)
		require.NoError(err)
		require.Equal([]byte("2"), node3.(*leafNode).value)
		node4, err := ext1.Search(cli, key1, 0)
		require.NoError(err)
		require.Equal([]byte("1"), node4.(*leafNode).value)
		// delete one
		node5, err := node1.Delete(cli, key1, 0)
		require.NoError(err)
		_, err = node5.Search(cli, key1, 0)
		require.Equal(trie.ErrNotExist, err)
		_, err = node5.Search(cli, expected, 0)
		require.NoError(err)
	}
}
