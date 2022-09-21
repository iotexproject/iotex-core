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
		equals(bn, cbn)
	})
	t.Run("clean empty root", func(t *testing.T) {
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
		equals(bn, cbn)
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
		equals(bn, cbn)
	})
}
