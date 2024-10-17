// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/triepb"
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

func TestLeafOperation(t *testing.T) {
	var (
		require = require.New(t)
		cli     = &merklePatriciaTrie{
			hashFunc: DefaultHashFunc,
			kvStore:  trie.NewMemKVStore(),
		}
	)

	// create
	node, err := newLeafNode(cli, keyType("iotex"), []byte("coin"))
	require.NoError(err)

	// leaf.Upsert newLeafNode -> leaf
	node, err = node.Upsert(cli, keyType("iotex"), 0, []byte("chain"))
	require.NoError(err)
	leaf, ok := node.(*leafNode)
	require.True(ok)
	require.Equal(keyType("iotex"), leaf.key)
	require.Equal([]byte("chain"), leaf.value)

	// leaf.Upsert newBranchNode -> branch
	node, err = node.Upsert(cli, keyType("block"), 0, []byte("chain"))
	require.NoError(err)
	bnode, ok := node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	checkLeaf(require, bnode, keyType("iotex"), 0, []byte("chain"))
	checkLeaf(require, bnode, keyType("block"), 0, []byte("chain"))

	// branch.Upsert child.Upsert -> leaf.Upsert newExtensionNode -> extension
	node, err = node.Upsert(cli, keyType("ioabc123"), 0, []byte("chabc"))
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	enode, ok := bnode.children[byte('i')].(*extensionNode)
	require.True(ok)
	require.Equal([]byte("o"), enode.path)
	bnode, ok = enode.child.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	checkLeaf(require, bnode, keyType("iotex"), 2, []byte("chain"))
	checkLeaf(require, bnode, keyType("ioabc123"), 2, []byte("chabc"))

	// branch.Upsert child.Upsert -> branch
	node, err = node.Upsert(cli, keyType("ixy"), 0, []byte("dog"))
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2) // (block, (ixy, ext))
	node1, ok := bnode.children[byte('i')]
	require.True(ok)
	bnode1, ok := node1.(*branchNode)
	require.True(ok)
	require.Len(bnode1.children, 2) // ixy, ext
	node2, ok := bnode1.children[byte('o')]
	require.True(ok)
	enode2, ok := node2.(*extensionNode)
	require.True(ok)
	require.Equal([]byte(""), enode2.path)
	require.Len(enode2.child.(*branchNode).children, 2)
	checkLeaf(require, bnode1, keyType("ixy"), 1, []byte("dog"))

	// insert wrong key
	for _, key := range []keyType{
		keyType("i"), keyType("bloc"), keyType("iote"), keyType("ioabc12345678"),
	} {
		require.Panics(func() { node.Upsert(cli, key, 0, []byte("ch")) }, "index out of range in commonPrefixLength.")
	}

	// check key
	node1, err = node.Search(cli, keyType("iotex"), 0)
	require.NoError(err)
	require.Equal([]byte("chain"), node1.(*leafNode).value)
	node1, err = node.Search(cli, keyType("ioabc123"), 0)
	require.NoError(err)
	require.Equal([]byte("chabc"), node1.(*leafNode).value)
	node1, err = node.Search(cli, keyType("block"), 0)
	require.NoError(err)
	require.Equal([]byte("chain"), node1.(*leafNode).value)
	node1, err = node.Search(cli, keyType("ixy"), 0)
	require.NoError(err)
	require.Equal([]byte("dog"), node1.(*leafNode).value)

	// branch.Delete case2 case *extensionNode -> branch.Delete b.updateChild -> branch
	node, err = node.Delete(cli, keyType("ixy"), 0)
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2) // block, ext
	checkLeaf(require, bnode, keyType("block"), 0, []byte("chain"))
	enode, ok = bnode.children[byte('i')].(*extensionNode)
	require.True(ok)
	require.Equal([]byte("o"), enode.path)
	require.Len(enode.child.(*branchNode).children, 2)
	node1, err = node.Search(cli, keyType("ixy"), 0)
	require.Equal(trie.ErrNotExist, err)

	// branch.Delete case2 case *extensionNode -> extension
	node, err = node.Delete(cli, keyType("block"), 0)
	require.NoError(err)
	enode, ok = node.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("io"), enode.path)
	require.Len(enode2.child.(*branchNode).children, 2)
	node1, err = node.Search(cli, keyType("block"), 0)
	require.Equal(trie.ErrNotExist, err)

	// extension.Delete default -> branch.Delete case2 case *leafNode -> leaf
	node, err = node.Delete(cli, keyType("iotex"), 0)
	require.NoError(err)
	leaf, ok = node.(*leafNode)
	require.True(ok)
	require.Equal(keyType("ioabc123"), leaf.key)
	require.Equal([]byte("chabc"), leaf.value)
	node1, err = node.Search(cli, keyType("iotex"), 0)
	require.Equal(trie.ErrNotExist, err)

	// leaf.Delete -> nil
	node, err = node.Delete(cli, keyType("ioabc123"), 0)
	require.NoError(err)
	require.Nil(node)

	// key is nil
	node, err = newLeafNode(cli, keyType{}, []byte("0"))
	require.NoError(err)
	node1, err = node.Search(cli, keyType{}, 0)
	require.NoError(err)
	require.Equal([]byte("0"), node1.(*leafNode).value)
	_, err = node.Delete(cli, keyType("123"), 0)
	require.Equal(trie.ErrNotExist, err)
	node2, err = node.Delete(cli, keyType{}, 0)
	require.NoError(err)
	require.Nil(node2)
}
