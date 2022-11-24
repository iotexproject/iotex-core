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

func TestLeafOperation(t *testing.T) {
	var (
		require = require.New(t)
		cli     = &merklePatriciaTrie{
			keyLength: 5,
			hashFunc:  DefaultHashFunc,
			kvStore:   trie.NewMemKVStore(),
		}
	)
	checkChild := func(bnode *branchNode, key keyType, offset uint8, value []byte) {
		child, ok := bnode.children[key[offset]]
		require.True(ok)
		ln1, ok := child.(*leafNode)
		require.True(ok)
		require.Equal(value, ln1.value)
	}

	// create
	node, err := newLeafNode(cli, keyType("iotex"), []byte("coin"))
	require.NoError(err)

	// insert same key -> leaf
	node, err = node.Upsert(cli, keyType("iotex"), 0, []byte("chain"))
	require.NoError(err)
	leaf, ok := node.(*leafNode)
	require.True(ok)
	require.Equal(keyType("iotex"), leaf.key)
	require.Equal([]byte("chain"), leaf.value)

	// insert second -> extension
	node, err = node.Upsert(cli, keyType("ioabc123"), 0, []byte("chabc"))
	require.NoError(err)
	enode, ok := node.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("io"), enode.path)
	bnode, ok := enode.child.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	checkChild(bnode, keyType("iotex"), 2, []byte("chain"))
	checkChild(bnode, keyType("ioabc123"), 2, []byte("chabc"))

	// insert third -> branch
	node, err = node.Upsert(cli, keyType("block"), 0, []byte("chain"))
	require.NoError(err)
	bnode, ok = node.(*branchNode)
	require.True(ok)
	require.Len(bnode.children, 2)
	checkChild(bnode, keyType("block"), 0, []byte("chain"))
	node1, ok := bnode.children[byte('i')]
	require.True(ok)
	enode1, ok := node1.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("o"), enode1.path)

	// insert wrong key
	require.Panics(func() { node.Upsert(cli, keyType("io"), 0, []byte("ch")) }, "index out of range")
	require.Panics(func() { node.Upsert(cli, keyType("ioab"), 0, []byte("ch")) }, "index out of range")

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

	// delete one -> extension
	node, err = node.Delete(cli, keyType("block"), 0)
	require.NoError(err)
	enode, ok = node.(*extensionNode)
	require.True(ok)
	require.Equal([]byte("io"), enode.path)
	node1, err = node.Search(cli, keyType("block"), 0)
	require.Equal(trie.ErrNotExist, err)

	// delete second -> leaf
	node, err = node.Delete(cli, keyType("iotex"), 0)
	require.NoError(err)
	leaf, ok = node.(*leafNode)
	require.True(ok)
	require.Equal(keyType("ioabc123"), leaf.key)
	require.Equal([]byte("chabc"), leaf.value)
	node1, err = node.Search(cli, keyType("iotex"), 0)
	require.Equal(trie.ErrNotExist, err)

	// delete third
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
	node2, err := node.Delete(cli, keyType{}, 0)
	require.NoError(err)
	require.Nil(node2)
}
