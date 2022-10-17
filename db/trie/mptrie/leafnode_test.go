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
