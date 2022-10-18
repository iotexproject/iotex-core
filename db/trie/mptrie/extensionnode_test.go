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

func TestExtensionNodeProto(t *testing.T) {
	require := require.New(t)
	exnode := &extensionNode{
		path: []byte("io"),
		child: &leafNode{
			cacheNode: cacheNode{
				hashVal: []byte("aaaaaaaaaaa"),
			},
			key:   keyType("iotex"),
			value: []byte("coin")},
	}
	proto, err := exnode.proto(nil, true)
	require.NoError(err)
	nodepb, ok := proto.(*triepb.NodePb)
	require.True(ok)
	extend, ok := nodepb.Node.(*triepb.NodePb_Extend)
	require.True(ok)
	exnode1 := newExtensionNodeFromProtoPb(extend.Extend, nil)
	require.Equal(exnode.path, exnode1.path)
	hnode, ok := exnode1.child.(*hashNode)
	require.True(ok)
	require.Equal(exnode.child.(*leafNode).hashVal, hnode.hashVal)
}

func TestExtensionNodeDelete(t *testing.T) {
	var (
		require = require.New(t)
		cli     = &merklePatriciaTrie{
			async:     true,
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
		}
		items = []struct {
			k keyType
			v []byte
		}{
			// leafnode
			{keyType("iotex"), []byte("coin")},
			{keyType("ioabc"), []byte("abc")},
		}
		err error
	)

	children := make(map[byte]node)
	children['t'], err = newLeafNode(cli, items[0].k, items[0].v)
	require.NoError(err)
	children['a'], err = newLeafNode(cli, items[1].k, items[1].v)
	require.NoError(err)

	child, err := newBranchNode(cli, children, nil)
	require.NoError(err)
	exnode, err := newExtensionNode(cli, []byte("io"), child)
	require.NoError(err)

	node, err := exnode.Delete(cli, items[0].k, 0)
	require.NoError(err)
	lnode, ok := node.(*leafNode)
	require.True(ok)
	require.Equal(items[1].k, lnode.key)

	err = exnode.Flush(cli)
	require.NoError(err)
}

func TestExtensionNodeUpsert(t *testing.T) {
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
			// leafnode
			{keyType("iotex"), []byte("coin")},
			{keyType("ioabc"), []byte("abc")},
			// testdata
			{keyType("iodef"), []byte("link")},
			{keyType("block"), []byte("chain")},
			{keyType("iuppy"), []byte("dog")},
		}
		offset uint8
		err    error
	)

	checkLeaf := func(bnode *branchNode, key keyType, offset uint8, value []byte) {
		child, ok := bnode.children[key[offset]]
		require.True(ok)
		ln1, ok := child.(*leafNode)
		require.True(ok)
		require.Equal(value, ln1.value)
	}

	checkBranch := func(bnode *branchNode, key1, key2 keyType, offset1, offset2 uint8, value1, value2 []byte) {
		require.Len(bnode.children, 2)
		child, ok := bnode.children[key1[offset1]]
		require.True(ok)
		en, ok := child.(*extensionNode)
		require.True(ok)
		bn, ok := en.child.(*branchNode)
		require.True(ok)
		require.Len(bn.children, 2)
		checkLeaf(bn, key1, offset2, value1)
		checkLeaf(bn, key2, offset2, value2)
	}

	children := make(map[byte]node)
	children['t'], err = newLeafNode(cli, items[0].k, items[0].v)
	require.NoError(err)
	children['a'], err = newLeafNode(cli, items[1].k, items[1].v)
	require.NoError(err)
	child, err := newBranchNode(cli, children, nil)
	require.NoError(err)

	exnode, err := newExtensionNode(cli, keyType("io"), child)
	require.NoError(err)

	for i, item := range items {
		node, err := exnode.Upsert(cli, item.k, offset, item.v)
		require.NoError(err)

		switch i {
		case 2:
			exnode1, ok := node.(*extensionNode)
			require.True(ok)
			bnode, ok := exnode1.child.(*branchNode)
			require.True(ok)
			require.Len(bnode.children, 3)
			vn, ok := bnode.children[item.k[2]]
			require.True(ok)
			ln, ok := vn.(*leafNode)
			require.True(ok)
			require.Equal(item.v, ln.value)
			continue

		case 3:
			bnode, ok := node.(*branchNode)
			require.True(ok)
			checkBranch(bnode, items[0].k, items[1].k, 0, 2, items[0].v, items[1].v)
			checkLeaf(bnode, item.k, 0, item.v)

		case 4:
			enode, ok := node.(*extensionNode)
			require.True(ok)
			require.Equal([]byte("i"), enode.path)
			bnode, ok := enode.child.(*branchNode)
			require.True(ok)
			checkBranch(bnode, items[0].k, items[1].k, 1, 2, items[0].v, items[1].v)
			checkLeaf(bnode, item.k, 1, item.v)
		}
	}
}
