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
		offset uint8
	)

	// childs := []node{
	// 	&branchNode{
	// 		children: map[byte]node{
	// 			'i': &hashNode{hashVal: []byte("i")},
	// 			'c': &hashNode{hashVal: []byte("c")},
	// 		},
	// 	},
	// 	&extensionNode{
	// 		path:  []byte("i"),
	// 		child: &hashNode{hashVal: []byte("d")},
	// 	},
	// 	&hashNode{hashVal: []byte("f")},
	// }

	// for _, child := range childs {
	children := make(map[byte]node)
	// lnode, err := newLeafNode(cli, []byte("iotex"), []byte("chain"))
	// require.NoError(err)
	// lnode, err := newLeafNode(cli, []byte("iotex"), []byte("chain"))
	// require.NoError(err)

	children[[]byte("a")[offset]] = &leafNode{key: keyType("a"), value: []byte("a")}
	children[[]byte("b")[offset]] = &leafNode{key: keyType("b"), value: []byte("b")}
	children[[]byte("b")[offset]] = &leafNode{key: keyType("c"), value: []byte("c")}

	// children[[]byte("a")[offset]] = &hashNode{hashVal: []byte("a")}
	// children[[]byte("b")[offset]] = &hashNode{hashVal: []byte("b")}
	// children[[]byte("c")[offset]] = &hashNode{hashVal: []byte("c")}
	child, _ := newBranchNode(cli, children, nil)

	exnode, err := newExtensionNode(cli, []byte(""), child)
	require.NoError(err)

	n, err := exnode.Delete(cli, keyType("a"), offset)
	require.NoError(err)
	t.Logf("%+v", n)

	// }

	// err = exnode.Flush(cli)
	// require.NoError(err)
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
			{keyType("iotex"), []byte("chain")},
			{keyType("block"), []byte("chain")},
			{keyType("ioabc"), []byte("chabc")},
		}
		offset uint8
		bnode  *branchNode
		ok     bool
	)

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
		case 1:
			bnode, ok = node.(*branchNode)
			require.True(ok)
		case 2:
			enode, ok := node.(*extensionNode)
			require.True(ok)
			require.Equal([]byte("io"), enode.path)
			bnode, ok = enode.child.(*branchNode)
			require.True(ok)
		}

		if i == 0 {
			break
		}

		require.Len(bnode.children, 2)
		child, ok := bnode.children[keyType("iotex")[0]]
		require.True(ok)
		ln1, ok := child.(*leafNode)
		require.True(ok)
		require.Equal([]byte("coin"), ln1.value)

		child, ok = bnode.children[keyType("block")[0]]
		require.True(ok)
		ln1, ok = child.(*leafNode)
		require.True(ok)
		require.Equal([]byte("chain"), ln1.value)
	}
}
