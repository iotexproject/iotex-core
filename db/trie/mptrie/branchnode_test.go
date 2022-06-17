// Copyright (c) 2022 IoTeX Foundation
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

func createBranchNode(require *require.Assertions, mpt *merklePatriciaTrie, children map[byte]node) *branchNode {
	node, err := newBranchNode(mpt, children)
	require.NoError(err)
	bnode, ok := node.(*branchNode)
	require.True(ok)
	return bnode
}

func createLeafNode(require *require.Assertions, mpt *merklePatriciaTrie, key keyType, value []byte) *leafNode {
	node, err := newLeafNode(mpt, key, value)
	require.NoError(err)
	lnode, ok := node.(*leafNode)
	require.True(ok)
	return lnode
}

func TestNewEmptyRootBranchNode(t *testing.T) {
	require := require.New(t)
	mpt := &merklePatriciaTrie{
		keyLength: 5,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
	}
	bnode := newEmptyRootBranchNode(mpt)
	require.Equal(mpt, bnode.mpt)
}

func TestNewBranchNodeFromProtoPb(t *testing.T) {
	require := require.New(t)
	mpt := &merklePatriciaTrie{
		keyLength: 5,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
	}
	nodes := []*triepb.BranchNodePb{
		{Index: 12, Path: []byte("testchildren1")},
		{Index: 23, Path: []byte("testchildren2")},
	}
	bnode := newBranchNodeFromProtoPb(&triepb.BranchPb{Branches: nodes}, mpt, []byte("test"))
	require.Equal(mpt, bnode.mpt)
	require.Equal([]byte("test"), bnode.hashVal)
	require.Equal([]byte{12, 23}, bnode.indices.li)
}

func TestBranchNodeChildren(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"chain", "link"},
		}
		expected = []struct{ k, v string }{
			{"block", "chain"},
			{"chain", "link"},
			{"iotex", "coin"},
		}
		mpt = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
			async:     true,
		}
		children       = make(map[byte]node)
		offset   uint8 = 0
	)

	for _, item := range items {
		children[[]byte(item.k)[offset]] = createLeafNode(require, mpt, []byte(item.k), []byte(item.v))
	}

	bnode := createBranchNode(require, mpt, children)
	nodes := bnode.Children()
	for i, node := range nodes {
		ln, ok := node.(*leafNode)
		require.True(ok)
		require.Equal(keyType(expected[i].k), ln.key)
		require.Equal([]byte(expected[i].v), ln.value)
	}
}

func TestBranchNodeDelete(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"chain", "link"},
			{"puppy", "dog"},
			{"night", "knight"},
		}
		expected = []struct{ k, v string }{
			items[len(items)-1],
			items[len(items)-2],
		}
		mpt = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
			async:     true,
		}
		children       = make(map[byte]node)
		offset   uint8 = 0
	)

	for _, item := range items {
		children[[]byte(item.k)[offset]] = createLeafNode(require, mpt, []byte(item.k), []byte(item.v))
	}
	bnode := createBranchNode(require, mpt, children)
	for i, item := range items {
		node, err := bnode.Delete([]byte(item.k), offset)
		require.NoError(err)
		last := len(items) - 2
		if i < last {
			require.Equal(bnode, node)
			_, err = bnode.Search(keyType(item.k), offset)
			require.Equal(trie.ErrNotExist, err)
		} else {
			ln, ok := node.(*leafNode)
			require.True(ok)
			require.Equal(keyType(expected[i-last].k), ln.key)
			require.Equal([]byte(expected[i-last].v), ln.value)
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
		mpt = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
		}
		offset uint8 = 0
	)

	bnode := newEmptyRootBranchNode(mpt)
	for _, item := range items {
		node, err := bnode.Upsert(keyType(item.k), offset, []byte(item.v))
		require.NoError(err)
		require.Equal(bnode, node)

		node, err = bnode.Search(keyType(item.k), offset)
		require.NoError(err)
		ln, ok := node.(*leafNode)
		require.True(ok)
		require.Equal(keyType(item.k), ln.key)
		require.Equal([]byte(item.v), ln.value)
	}
}

func TestBranchNodeFlush(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"chain", "link"},
		}
		mpt = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
		}
		offset uint8 = 0
	)

	bnode := newEmptyRootBranchNode(mpt)
	hashVal, err := bnode.Hash()
	require.NoError(err)
	for _, item := range items {
		bnode.children[[]byte(item.k)[offset]] = &leafNode{
			cacheNode: cacheNode{
				mpt:     mpt,
				dirty:   true,
				hashVal: hashVal,
			},
			key:   []byte(item.k),
			value: []byte(item.v),
		}
	}
	bnode.indices = NewSortedList(bnode.children)
	require.NoError(bnode.Flush())
}

func TestBranchNodeUpdateChild(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"bhain", "link"},
			{"cuppy", "dog"},
		}
		mpt = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
			async:     true,
		}
		children       = make(map[byte]node)
		offset   uint8 = 0
		child    node  = nil
	)

	for i, item := range items {
		ln := createLeafNode(require, mpt, keyType(item.k), []byte(item.v))
		if i < len(items)-1 {
			children[[]byte(item.k)[offset]] = ln
		} else {
			node, err := ln.store()
			require.NoError(err)
			child = node
		}
	}

	bnode := createBranchNode(require, mpt, children)
	for i := 1; i < len(items); i++ {
		item := items[i]

		node, err := bnode.updateChild([]byte(item.k)[offset], nil, false)
		require.NoError(err)
		require.Equal(bnode, node)
		_, err = bnode.Search(keyType(item.k), offset)
		require.Equal(trie.ErrNotExist, err)

		node, err = bnode.updateChild([]byte(item.k)[offset], nil, true)
		require.NoError(err)
		require.Equal(bnode, node)
		_, err = bnode.Search(keyType(item.k), offset)
		require.Equal(trie.ErrNotExist, err)

		node, err = bnode.updateChild([]byte(item.k)[offset], child, false)
		require.NoError(err)
		require.Equal(bnode, node)
		node, err = bnode.child(keyType(item.k)[offset])
		require.NoError(err)
		hn, ok := node.(*hashNode)
		require.True(ok)
		require.Equal(child, hn)
	}

	for i := 1; i < len(items); i++ {
		item := items[i]
		mpt := &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
			async:     true,
		}
		bnode = createBranchNode(require, mpt, children)
		bnode.mpt.async = false
		node, err := bnode.updateChild([]byte(item.k)[offset], child, true)
		require.NoError(err)
		hn, ok := node.(*hashNode)
		require.True(ok)
		require.Equal(bnode.hashVal, hn.hashVal)
	}
}
