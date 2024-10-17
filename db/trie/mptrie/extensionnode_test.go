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

func TestExtensionOperation(t *testing.T) {
	var (
		require = require.New(t)
		cli     = &merklePatriciaTrie{
			hashFunc: DefaultHashFunc,
			kvStore:  trie.NewMemKVStore(),
		}
	)

	// create
	child, err := newLeafNode(cli, keyType("iotex"), []byte("coin"))
	require.NoError(err)
	node, err := newExtensionNode(cli, keyType("iotex"), child)
	require.NoError(err)

	// extension.Upsert updateChild -> extension
	node, err = node.Upsert(cli, keyType("iotex"), 0, []byte("chain"))
	require.NoError(err)
	enode, ok := node.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("iotex"), enode.path)
	ln, ok := enode.child.(*leafNode)
	require.True(ok)
	require.Equal([]byte("chain"), ln.value)

	// extension.Upsert newExtensionNode -> extension
	node, err = node.Upsert(cli, keyType("ioabc123"), 0, []byte("chabc"))
	require.NoError(err)
	enode, ok = node.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("io"), enode.path)
	bnode, ok := enode.child.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	vn, ok := bnode.children[byte('a')]
	require.True(ok)
	ln, ok = vn.(*leafNode)
	require.True(ok)
	require.Equal([]byte("chabc"), ln.value)

	// extension.Upsert newBranchNode -> branch
	node, err = node.Upsert(cli, keyType("block"), 0, []byte("chain"))
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2) // (block, ext)
	checkLeaf(require, bnode, keyType("block"), 0, []byte("chain"))
	vn, ok = bnode.children[byte('i')] // ext
	require.True(ok)
	ex1, ok := vn.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("o"), ex1.path)
	bn1, ok := ex1.child.(*branchNode)
	require.True(ok)
	require.Len(bn1.children, 2)

	// branch.Upsert child.Upsert -> branch
	node, err = node.Upsert(cli, keyType("ixy"), 0, []byte("dog"))
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2) // (block, (ixy, ext))
	vn, ok = bnode.children[byte('i')]
	require.True(ok)
	bn1, ok = vn.(*branchNode)
	require.True(ok)
	require.Len(bn1.children, 2) // ixy, ext
	checkLeaf(require, bn1, keyType("ixy"), 1, []byte("dog"))

	// branch.Upsert newLeafNode -> branch
	node, err = node.Upsert(cli, keyType("idef"), 0, []byte("cat"))
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2) // (block, (idef, ixy, ext))
	bn1, ok = bnode.children[byte('i')].(*branchNode)
	require.True(ok)
	require.Len(bn1.children, 3) // idef, ixy, ext

	// insert wrong key
	for _, key := range []keyType{
		keyType("b"), keyType("io"), keyType("ioabc12"), keyType("ixy123"),
	} {
		require.Panics(func() { node.Upsert(cli, key, 0, []byte("ch")) }, "index out of range in commonPrefixLength.")
	}

	// check key
	node1, err := node.Search(cli, keyType("iotex"), 0)
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
	node1, err = node.Search(cli, keyType("idef"), 0)
	require.NoError(err)
	require.Equal([]byte("cat"), node1.(*leafNode).value)

	// branch.Delete case2 default newExtensionNode -> extension
	node, err = node.Delete(cli, keyType("block"), 0)
	require.NoError(err)
	enode, ok = node.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("i"), enode.path)
	bnode, ok = enode.child.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 3) // ext, leaf
	vn, ok = bnode.children[byte('o')]
	require.True(ok)
	ex1, ok = vn.(*extensionNode)
	require.True(ok)
	bn1, ok = ex1.child.(*branchNode)
	require.True(ok)
	require.Len(bn1.children, 2) // (iotex, ioabc123)
	checkLeaf(require, bnode, keyType("ixy"), 1, []byte("dog"))
	node1, err = node.Search(cli, keyType("block"), 0)
	require.Equal(trie.ErrNotExist, err)

	// extension.Delete case *branchNode -> branch.Delete default b.updateChild -> extension
	node, err = node.Delete(cli, keyType("idef"), 0)
	require.NoError(err)
	enode, ok = node.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("i"), enode.path)
	bnode, ok = enode.child.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2) // ext, leaf
	node1, err = node.Search(cli, keyType("idef"), 0)
	require.Equal(trie.ErrNotExist, err)

	// extension.Delete case *extensionNode -> branch.Delete case2 case *extensionNode -> extension
	node, err = node.Delete(cli, keyType("ixy"), 0)
	require.NoError(err)
	enode, ok = node.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("io"), enode.path)
	bnode, ok = enode.child.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	checkLeaf(require, bnode, keyType("ioabc123"), 2, []byte("chabc"))
	vn, ok = bnode.children[byte('t')]
	require.True(ok)
	ex1, ok = vn.(*extensionNode)
	require.True(ok)
	node1, err = node.Search(cli, keyType("ixy"), 0)
	require.Equal(trie.ErrNotExist, err)

	// extension.Delete default -> branch.Delete case2 case *leafNode -> leaf
	node, err = node.Delete(cli, keyType("iotex"), 0)
	require.NoError(err)
	ln, ok = node.(*leafNode)
	require.True(ok)
	node1, err = node.Search(cli, keyType("iotex"), 0)
	require.Equal(trie.ErrNotExist, err)

	// delete last -> nil
	node, err = node.Delete(cli, keyType("ioabc123"), 0)
	require.NoError(err)
	require.Nil(node)

	// child is nil
	child1, _ := newLeafNode(cli, nil, nil)
	node, err = newExtensionNode(cli, keyType{}, child1)
	require.NoError(err)
	node, err = node.Search(cli, keyType{}, 0)
	require.NoError(err)
	require.Equal(child1, node)
	_, err = node.Delete(cli, keyType("xyz123"), 0)
	require.Equal(trie.ErrNotExist, err)
	node, err = node.Delete(cli, keyType{}, 0)
	require.NoError(err)
	require.Nil(node)
}
