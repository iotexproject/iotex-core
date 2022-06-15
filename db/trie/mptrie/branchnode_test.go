// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"context"
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

func creatrBranchNode(require *require.Assertions) *branchNode {
	mpt := &merklePatriciaTrie{
		keyLength: 20,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
	}
	children := make(map[byte]node)
	children[1] = &branchNode{
		cacheNode: cacheNode{mpt: mpt, hashVal: []byte("test1")},
	}
	children[2] = &branchNode{
		cacheNode: cacheNode{mpt: mpt, hashVal: []byte("test2")},
	}
	node, err := newBranchNode(mpt, children)
	require.NoError(err)
	bnode, ok := node.(*branchNode)
	require.True(ok)
	return bnode
}

func TestNewBranchNode(t *testing.T) {
	require := require.New(t)
	mpt := &merklePatriciaTrie{
		keyLength: 20,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
	}
	children := make(map[byte]node)
	children[byte(1)] = &branchNode{
		cacheNode: cacheNode{mpt: mpt, hashVal: []byte("test1")},
	}
	children[byte(2)] = &branchNode{
		cacheNode: cacheNode{mpt: mpt, hashVal: []byte("test2")},
	}
	node, err := newBranchNode(mpt, children)
	require.NoError(err)
	bnode, ok := node.(*branchNode)
	require.True(ok)
	// bnode := createBranchNode(require, mpt, children)
	require.Equal(mpt, bnode.mpt)
	require.Equal(children, bnode.children)
}

func TestNewEmptyRootBranchNode(t *testing.T) {
	require := require.New(t)
	mpt := &merklePatriciaTrie{
		keyLength: 20,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
	}
	bnode := newEmptyRootBranchNode(mpt)
	require.Equal(mpt, bnode.mpt)
}

func TestNewBranchNodeFromProtoPb(t *testing.T) {
	require := require.New(t)
	mpt := &merklePatriciaTrie{
		keyLength: 20,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
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
	require := require.New(t)
	mpt := &merklePatriciaTrie{
		keyLength: 20,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
	}
	expected := []*branchNode{
		{
			cacheNode: cacheNode{mpt: mpt, hashVal: []byte("test1")},
		},
		{
			cacheNode: cacheNode{mpt: mpt, hashVal: []byte("test2")},
		},
	}
	children := make(map[byte]node)
	children[1] = expected[0]
	children[2] = expected[1]
	node, err := newBranchNode(mpt, children)
	require.NoError(err)
	bnode, ok := node.(*branchNode)
	require.True(ok)
	// bnode := createBranchNode(require, mpt, children)
	nodes := bnode.Children()
	for _, node := range nodes {
		ret, ok := node.(*branchNode)
		require.True(ok)
		require.Contains(expected, ret)
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
	)

	mpt := &merklePatriciaTrie{
		keyLength: 5,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
	}
	err := mpt.Start(context.Background())
	require.NoError(err)
	for _, item := range items {
		err = mpt.putNode([]byte(item.k), []byte(item.v))
		require.NoError(err)
	}

	expected := []node{
		createLeafNode(require, mpt, []byte("iotex"), []byte("coin")),
		createLeafNode(require, mpt, []byte("block"), []byte("chain")),
		createLeafNode(require, mpt, []byte("chain"), []byte("dog")),
		createLeafNode(require, mpt, []byte("puppy"), []byte("knight")),

		// newHashNode(mpt, []byte("iotex")),
		// newHashNode(mpt, []byte("block")),

		&branchNode{
			cacheNode: cacheNode{mpt: mpt, hashVal: []byte("chain")},
		},
		&branchNode{
			cacheNode: cacheNode{mpt: mpt, hashVal: []byte("puppy")},
		},
	}
	children := make(map[byte]node)
	children[11] = expected[0]
	children[22] = expected[1]
	xxx := createBranchNode(require, mpt, children)
	t.Logf("xxx=%+v", xxx.indices)

	children2 := make(map[byte]node)
	children2[33] = expected[2]
	children2[44] = expected[3]
	xxx1 := createBranchNode(require, mpt, children2)
	t.Logf("xxx1=%+v", xxx1.indices)

	children3 := make(map[byte]node)
	children3[55] = xxx
	children3[66] = xxx1
	bnode := createBranchNode(require, mpt, children3)
	t.Logf("bnode=%+v", bnode.indices)

	node, err := bnode.Delete(keyType{55, 11, 22, 66, 33, 44}, 0)
	require.NoError(err)
	t.Logf("%+v", node)
	// ret, ok := node.(*branchNode)
	// require.True(ok)
	// childNodes := bnode.Children()

	// require.NotContains(childNodes, node)
}

func TestBranchNodeUpsert(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"chain", "link"},
			{"puppy", "dog"},
			{"night", "knight"},
		}
	)

	mpt := &merklePatriciaTrie{
		keyLength: 5,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
	}
	err := mpt.Start(context.Background())
	require.NoError(err)
	for _, item := range items {
		err = mpt.putNode([]byte(item.k), []byte(item.v))
		require.NoError(err)
	}

	expected := []node{
		createLeafNode(require, mpt, []byte("iotex"), []byte("coin")),
		createLeafNode(require, mpt, []byte("block"), []byte("chain")),
		createLeafNode(require, mpt, []byte("chain"), []byte("dog")),
		createLeafNode(require, mpt, []byte("puppy"), []byte("knight")),
	}
	children := make(map[byte]node)
	children[11] = expected[0]
	children[22] = expected[1]
	xxx := createBranchNode(require, mpt, children)
	t.Logf("xxx=%+v", xxx.indices)

	children2 := make(map[byte]node)
	children2[33] = expected[2]
	children2[44] = expected[3]
	xxx1 := createBranchNode(require, mpt, children2)
	t.Logf("xxx1=%+v", xxx1.indices)

	children3 := make(map[byte]node)
	children3[55] = xxx
	children3[66] = xxx1
	bnode := createBranchNode(require, mpt, children3)
	t.Logf("bnode=%+v", bnode.indices)

	node, err := bnode.Delete(keyType{55, 11, 22, 66, 33, 44}, 0)
	require.NoError(err)
	t.Logf("%+v", node)
	// ret, ok := node.(*branchNode)
	// require.True(ok)
	// childNodes := bnode.Children()

	// require.NotContains(childNodes, node)
}

func TestBranchNodeSearch(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"chain", "link"},
			{"puppy", "dog"},
			{"night", "knight"},
		}
	)

	mpt := &merklePatriciaTrie{
		keyLength: 5,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
	}
	err := mpt.Start(context.Background())
	require.NoError(err)
	for _, item := range items {
		err = mpt.putNode([]byte(item.k), []byte(item.v))
		require.NoError(err)
	}

	expected := []node{
		createLeafNode(require, mpt, []byte("iotex"), []byte("coin")),
		createLeafNode(require, mpt, []byte("block"), []byte("chain")),
		createLeafNode(require, mpt, []byte("chain"), []byte("dog")),
		createLeafNode(require, mpt, []byte("puppy"), []byte("knight")),

		// newHashNode(mpt, []byte("iotex")),
		// newHashNode(mpt, []byte("block")),

		&branchNode{
			cacheNode: cacheNode{mpt: mpt, hashVal: []byte("chain")},
		},
		&branchNode{
			cacheNode: cacheNode{mpt: mpt, hashVal: []byte("puppy")},
		},
	}
	children := make(map[byte]node)
	children[11] = expected[0]
	children[22] = expected[1]
	xxx := createBranchNode(require, mpt, children)
	t.Logf("xxx=%+v", xxx.indices)

	children2 := make(map[byte]node)
	children2[33] = expected[2]
	children2[44] = expected[3]
	xxx1 := createBranchNode(require, mpt, children2)
	t.Logf("xxx1=%+v", xxx1.indices)

	children3 := make(map[byte]node)
	children3[55] = xxx
	children3[66] = xxx1
	bnode := createBranchNode(require, mpt, children3)
	t.Logf("bnode=%+v", bnode.indices)

	node, err := bnode.Delete(keyType{55, 11, 22, 66, 33, 44}, 0)
	require.NoError(err)
	t.Logf("%+v", node)
	// ret, ok := node.(*branchNode)
	// require.True(ok)
	// childNodes := bnode.Children()

	// require.NotContains(childNodes, node)
}

func TestBranchNodeFlush(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"chain", "link"},
			{"puppy", "dog"},
			{"night", "knight"},
		}
	)

	mpt := &merklePatriciaTrie{
		keyLength: 5,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
		async:     true,
	}
	err := mpt.Start(context.Background())
	require.NoError(err)
	for _, item := range items {
		err = mpt.putNode([]byte(item.k), []byte(item.v))
		require.NoError(err)
	}

	expected := []node{
		createLeafNode(require, mpt, []byte("iotex"), []byte("coin")),
		createLeafNode(require, mpt, []byte("block"), []byte("chain")),
		createLeafNode(require, mpt, []byte("chain"), []byte("dog")),
		createLeafNode(require, mpt, []byte("puppy"), []byte("knight")),

		// newHashNode(mpt, []byte("iotex")),
		// newHashNode(mpt, []byte("block")),

		&branchNode{
			cacheNode: cacheNode{mpt: mpt, hashVal: []byte("chain")},
		},
		&branchNode{
			cacheNode: cacheNode{mpt: mpt, hashVal: []byte("puppy")},
		},
	}
	children := make(map[byte]node)
	children[11] = expected[0]
	children[22] = expected[1]
	xxx := createBranchNode(require, mpt, children)
	t.Logf("xxx=%+v", xxx.indices)

	children2 := make(map[byte]node)
	children2[33] = expected[2]
	children2[44] = expected[3]
	xxx1 := createBranchNode(require, mpt, children2)
	t.Logf("xxx1=%+v", xxx1.indices)

	children3 := make(map[byte]node)
	children3[55] = xxx
	children3[66] = xxx1
	bnode := createBranchNode(require, mpt, children3)
	t.Logf("bnode=%+v", bnode.indices)

	node, err := bnode.Delete(keyType{55, 11, 22, 66, 33, 44}, 0)
	require.NoError(err)
	t.Logf("%+v", node)
	// ret, ok := node.(*branchNode)
	// require.True(ok)
	// childNodes := bnode.Children()

	// require.NotContains(childNodes, node)
}
