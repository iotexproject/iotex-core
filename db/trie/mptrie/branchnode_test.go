// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/triepb"
)

func equals(bn *branchNode, clone *branchNode) bool {
	if bn.isRoot != clone.isRoot {
		return false
	}
	if bn.dirty != clone.dirty {
		return false
	}
	if !bytes.Equal(bn.hashVal, clone.hashVal) || !bytes.Equal(bn.ser, clone.ser) {
		return false
	}
	if len(bn.children) != len(clone.children) {
		return false
	}
	for key, child := range clone.children {
		if bn.children[key] != child {
			return false
		}
	}
	indices := bn.indices.List()
	cloneIndices := clone.indices.List()
	if len(indices) != len(cloneIndices) {
		return false
	}
	for i, value := range cloneIndices {
		if indices[i] != value {
			return false
		}
	}
	return true
}

func TestBranchNodeClone(t *testing.T) {
	require := require.New(t)
	cli := &merklePatriciaTrie{async: true, hashFunc: DefaultHashFunc}
	t.Run("dirty empty root", func(t *testing.T) {
		children := map[byte]node{}
		indices := NewSortedList(children)
		node, err := newRootBranchNode(cli, children, indices, true)
		require.NoError(err)
		bn, ok := node.(*branchNode)
		require.True(ok)
		clone, err := node.Clone()
		require.NoError(err)
		cbn, ok := clone.(*branchNode)
		require.True(ok)
		require.True(equals(bn, cbn))
	})
	t.Run("clean empty root", func(t *testing.T) {
		children := map[byte]node{}
		indices := NewSortedList(children)
		node, err := newRootBranchNode(cli, children, indices, false)
		require.NoError(err)
		bn, ok := node.(*branchNode)
		require.True(ok)
		clone, err := node.Clone()
		require.NoError(err)
		cbn, ok := clone.(*branchNode)
		require.True(ok)
		require.True(equals(bn, cbn))
	})
	t.Run("normal branch node", func(t *testing.T) {
		children := map[byte]node{}
		children['a'] = &hashNode{hashVal: []byte("a")}
		children['b'] = &hashNode{hashVal: []byte("b")}
		children['c'] = &hashNode{hashVal: []byte("c")}
		children['d'] = &hashNode{hashVal: []byte("d")}
		indices := NewSortedList(children)
		node, err := newBranchNode(cli, children, indices)
		require.NoError(err)
		bn, ok := node.(*branchNode)
		require.True(ok)
		clone, err := bn.Clone()
		require.NoError(err)
		cbn, ok := clone.(*branchNode)
		require.True(ok)
		require.True(equals(bn, cbn))
	})
}

func TestBranchNodeProto(t *testing.T) {
	require := require.New(t)
	children := map[byte]node{}
	children['a'] = &hashNode{hashVal: []byte("a")}
	children['b'] = &hashNode{hashVal: []byte("b")}
	children['c'] = &hashNode{hashVal: []byte("c")}
	children['d'] = &hashNode{hashVal: []byte("d")}
	indices := NewSortedList(children)
	bnode := &branchNode{
		children: children,
		indices:  indices,
	}
	cli := &merklePatriciaTrie{async: true, hashFunc: DefaultHashFunc}
	proto, err := bnode.proto(cli, true)
	require.NoError(err)
	nodepb, ok := proto.(*triepb.NodePb)
	require.True(ok)
	branch, ok := nodepb.Node.(*triepb.NodePb_Branch)
	require.True(ok)
	bnode1 := newBranchNodeFromProtoPb(branch.Branch, nil)
	for key, child := range bnode1.children {
		h, err := bnode.children[key].Hash(cli)
		require.NoError(err)
		h1, err := child.Hash(cli)
		require.NoError(err)
		require.Equal(h, h1)
	}
	li := bnode.indices.List()
	for i, value := range bnode1.indices.List() {
		require.Equal(li[i], value)
	}
	require.Equal(bnode.isRoot, bnode1.isRoot)
	require.Equal(bnode.dirty, bnode1.dirty)
}

func TestBranchNodeChildren(t *testing.T) {
	require := require.New(t)
	children := map[byte]node{}
	children['a'] = &hashNode{hashVal: []byte("a")}
	children['b'] = &hashNode{hashVal: []byte("b")}
	children['c'] = &hashNode{hashVal: []byte("c")}
	children['d'] = &hashNode{hashVal: []byte("d")}
	indices := NewSortedList(children)
	bnode := &branchNode{
		children: children,
		indices:  indices,
	}
	childs := bnode.Children()
	li := bnode.indices.List()
	for i, node := range childs {
		require.Equal(bnode.children[li[i]], node)
	}
}

func TestBranchOperation(t *testing.T) {
	var (
		require = require.New(t)
		cli     = &merklePatriciaTrie{
			hashFunc: DefaultHashFunc,
			kvStore:  trie.NewMemKVStore(),
		}
		children = make(map[byte]node)
	)

	// create
	node, err := newLeafNode(cli, keyType("iotex"), []byte("coin"))
	require.NoError(err)
	children[byte('i')] = node
	node, err = newBranchNode(cli, children, nil)
	require.NoError(err)
	require.Panics(func() { node.Delete(cli, keyType("iotex"), 0) }, "branch shouldn't have 0 child after deleting")

	// branch.Upsert child.Upsert -> branch
	node, err = node.Upsert(cli, keyType("ioabc123"), 0, []byte("chabc"))
	require.NoError(err)
	bnode, ok := node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 1) // ext
	vn, ok := bnode.children[byte('i')]
	require.True(ok)
	ex1, ok := vn.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("o"), ex1.path)
	bn1, ok := ex1.child.(*branchNode)
	require.True(ok)
	require.Len(bn1.children, 2)
	checkLeaf(require, bn1, keyType("iotex"), 2, []byte("coin"))
	checkLeaf(require, bn1, keyType("ioabc123"), 2, []byte("chabc"))

	// branch.Upsert newLeafNode -> branch
	node, err = node.Upsert(cli, keyType("block"), 0, []byte("chain"))
	require.NoError(err)
	bnode1, ok := node.(*branchNode)
	require.True(ok)
	require.Len(bnode1.children, 2) // block, ext
	require.Equal(bnode.children[byte('i')], bnode1.children[byte('i')])
	checkLeaf(require, bnode1, keyType("block"), 0, []byte("chain"))

	// branch.Upsert child.Upsert -> branch
	node, err = node.Upsert(cli, keyType("iotex"), 0, []byte("chain"))
	require.NoError(err)
	bnode2, ok := node.(*branchNode)
	require.True(ok)
	require.Len(bnode2.children, 2) // block, ex2
	require.Equal(bnode1.children[byte('b')], bnode2.children[byte('b')])
	require.NotEqual(bnode.children[byte('i')], bnode2.children[byte('i')])
	ex2, ok := bnode2.children[byte('i')].(*extensionNode)
	require.True(ok)
	bn2, ok := ex2.child.(*branchNode)
	require.True(ok)
	checkLeaf(require, bn2, keyType("iotex"), 2, []byte("chain"))

	// branch.Upsert child.Upsert -> branch
	node, err = node.Upsert(cli, keyType("ixy"), 0, []byte("dog"))
	require.NoError(err)
	bnode3, ok := node.(*branchNode)
	require.True(ok)
	require.Len(bnode3.children, 2) // block, bn3
	require.Equal(bnode1.children[byte('b')], bnode3.children[byte('b')])
	require.NotEqual(bnode.children[byte('i')], bnode3.children[byte('i')])
	bn3, ok := bnode3.children[byte('i')].(*branchNode)
	require.True(ok)
	require.Len(bn3.children, 2) // ixy, ext
	checkLeaf(require, bn3, keyType("ixy"), 1, []byte("dog"))

	// branch.Upsert child.Upsert -> branch
	node, err = node.Upsert(cli, keyType("idef"), 0, []byte("cat"))
	require.NoError(err)
	bnode4, ok := node.(*branchNode)
	require.True(ok)
	require.Len(bnode4.children, 2) // block, bn4
	require.Equal(bnode1.children[byte('b')], bnode4.children[byte('b')])
	bn4, ok := bnode4.children[byte('i')].(*branchNode)
	require.True(ok)
	require.Len(bn4.children, 3) // idef, ixy, ext
	checkLeaf(require, bn4, keyType("idef"), 1, []byte("cat"))

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

	// branch.Delete default b.updateChild -> extension
	node, err = node.Delete(cli, keyType("idef"), 0)
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	node1, err = node.Search(cli, keyType("idef"), 0)
	require.Equal(trie.ErrNotExist, err)

	// branch.Delete case2 case *leafNode -> branch.Delete b.updateChild
	node, err = node.Delete(cli, keyType("ioabc123"), 0)
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	node1, err = node.Search(cli, keyType("ioabc123"), 0)
	require.Equal(trie.ErrNotExist, err)

	// branch.Delete case2 default newExtensionNode
	node, err = node.Delete(cli, keyType("block"), 0)
	require.NoError(err)
	enode, ok := node.(*extensionNode)
	require.True(ok)
	bn1, ok = enode.child.(*branchNode)
	require.True(ok)
	require.Len(bn1.children, 2)
	node1, err = node.Search(cli, keyType("block"), 0)
	require.Equal(trie.ErrNotExist, err)

	// extension.Delete default
	node, err = node.Delete(cli, keyType("iotex"), 0)
	require.NoError(err)
	_, ok = node.(*leafNode)
	require.True(ok)
	node1, err = node.Search(cli, keyType("iotex"), 0)
	require.Equal(trie.ErrNotExist, err)

	// leaf.Delete
	node, err = node.Delete(cli, keyType("ixy"), 0)
	require.NoError(err)
	require.Nil(node)

	// children is nil
	node, err = newRootBranchNode(cli, nil, nil, false)
	require.NoError(err)
	node, err = node.Delete(cli, keyType{1}, 0)
	require.Equal(trie.ErrNotExist, err)
	require.Nil(node)

	// children is max
	node, err = newRootBranchNode(cli, nil, nil, false)
	for i := byte(0); i < 255; i++ {
		node, err = node.Upsert(cli, keyType{1, i}, 1, []byte{i})
		require.NoError(err)
	}
	require.Len(node.(*branchNode).children, 255)
	n1, err := node.Search(cli, keyType{1, 2}, 1)
	require.NoError(err)
	require.Equal([]byte{2}, n1.(*leafNode).value)
	node, err = node.Upsert(cli, keyType{2, 3, 4, 5}, 0, []byte{2, 3, 4, 5})
	require.NoError(err)
	n1, err = node.Search(cli, keyType{2, 3, 4, 5}, 0)
	require.NoError(err)
	require.Equal([]byte{2, 3, 4, 5}, n1.(*leafNode).value)
	require.Panics(func() { node.Search(cli, keyType{1, 2}, 1) }, "keyType{1, 2} is not exist")
}

func checkLeaf(require *require.Assertions, bnode *branchNode, key keyType, offset uint8, value []byte) {
	child, ok := bnode.children[key[offset]]
	require.True(ok)
	ln, ok := child.(*leafNode)
	require.True(ok)
	require.Equal(value, ln.value)
}
