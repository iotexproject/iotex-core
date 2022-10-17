// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
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
	t.Run("dirty empty root", func(t *testing.T) {
		children := map[byte]node{}
		indices := NewSortedList(children)
		node, err := newRootBranchNode(nil, children, indices, true)
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
		node, err := newRootBranchNode(nil, children, indices, false)
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
		node, err := newBranchNode(&merklePatriciaTrie{async: true}, children, indices)
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
	cli := &merklePatriciaTrie{async: true}
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

func TestBranchNodeDelete(t *testing.T) {
	var (
		require    = require.New(t)
		itemsArray = [][]struct{ k, v string }{
			{
				{"iotex", "coin"},
			},
			{
				{"iotex", "coin"},
				{"block", "chain"},
			},
			{
				{"iotex", "coin"},
				{"block", "chain"},
				{"chain", "link"},
				{"puppy", "dog"},
			},
		}
		cli = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
		}
		children = make(map[byte]node)
		offset   uint8
	)

	for i, items := range itemsArray {
		for _, item := range items {
			lnode, err := newLeafNode(cli, keyType(item.k), []byte(item.v))
			require.NoError(err)
			children[keyType(item.k)[offset]] = lnode
		}
		indices := NewSortedList(children)

		bnode, err := newBranchNode(cli, children, indices)
		require.NoError(err)
		for _, item := range items {
			if i == 0 {
				require.Panics(func() { bnode.Delete(cli, keyType(item.k), offset) })
			} else {
				newnode, err := bnode.Delete(cli, keyType(item.k), offset)
				require.NoError(err)
				err = bnode.Flush(cli)
				require.NoError(err)
				_, err = newnode.Search(cli, keyType(item.k), offset)
				require.Equal(trie.ErrNotExist, err)
			}
		}
	}
}

func TestBranchNodeUpsert(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"chain", "link"},
			{"cuppy", "dog"},
			{"cught", "knight"},
		}
		cli = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
		}
		children = make(map[byte]node)
		offset   uint8
	)

	lnode, err := newLeafNode(cli, keyType("iotex"), []byte("chain"))
	require.NoError(err)
	children[keyType("iotex")[offset]] = lnode

	bnode, err := newRootBranchNode(cli, children, NewSortedList(nil), false)
	require.NoError(err)
	for _, item := range items {
		newnode, err := bnode.Upsert(cli, keyType(item.k), offset, []byte(item.v))
		require.NoError(err)
		err = bnode.Flush(cli)
		require.NoError(err)
		node, err := newnode.Search(cli, keyType(item.k), offset)
		require.NoError(err)
		ln, ok := node.(*leafNode)
		require.True(ok)
		require.Equal(keyType(item.k), ln.key)
		require.Equal([]byte(item.v), ln.value)
	}
}
