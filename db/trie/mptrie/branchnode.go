// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

type branchNode struct {
	cacheNode
	children map[byte]node
	indices  *SortedList
	isRoot   bool
}

func newBranchNode(
	cli client,
	children map[byte]node,
	indices *SortedList,
) (node, error) {
	if len(children) == 0 {
		return nil, errors.New("branch node children cannot be empty")
	}
	if indices == nil {
		indices = NewSortedList(children)
	}
	bnode := &branchNode{
		cacheNode: cacheNode{
			dirty: true,
		},
		children: children,
		indices:  indices,
	}
	bnode.cacheNode.serializable = bnode
	if len(bnode.children) != 0 {
		if !cli.asyncMode() {
			if err := bnode.store(cli); err != nil {
				return nil, err
			}
		}
	}
	return bnode, nil
}

func newRootBranchNode(cli client, children map[byte]node, indices *SortedList, dirty bool) (branch, error) {
	if indices == nil {
		indices = NewSortedList(children)
	}
	bnode := &branchNode{
		cacheNode: cacheNode{
			dirty: dirty,
		},
		children: children,
		indices:  indices,
		isRoot:   true,
	}
	bnode.cacheNode.serializable = bnode
	if len(bnode.children) != 0 {
		if !cli.asyncMode() {
			if err := bnode.store(cli); err != nil {
				return nil, err
			}
		}
	}
	return bnode, nil
}

func newBranchNodeFromProtoPb(pb *triepb.BranchPb, hashVal []byte) *branchNode {
	bnode := &branchNode{
		cacheNode: cacheNode{
			hashVal: hashVal,
			dirty:   false,
		},
		children: make(map[byte]node, len(pb.Branches)),
	}
	for _, n := range pb.Branches {
		bnode.children[byte(n.Index)] = newHashNode(n.Path)
	}
	bnode.indices = NewSortedList(bnode.children)
	bnode.cacheNode.serializable = bnode
	return bnode
}

func (b *branchNode) MarkAsRoot() {
	b.isRoot = true
}

func (b *branchNode) Children() []node {
	ret := make([]node, 0, len(b.children))
	for _, idx := range b.indices.List() {
		ret = append(ret, b.children[idx])
	}
	return ret
}

func (b *branchNode) Delete(cli client, key keyType, offset uint8) (node, error) {
	offsetKey := key[offset]
	child, err := b.child(offsetKey)
	if err != nil {
		return nil, err
	}
	newChild, err := child.Delete(cli, key, offset+1)
	if err != nil {
		return nil, err
	}
	if newChild != nil || b.isRoot {
		return b.updateChild(cli, offsetKey, newChild, false)
	}
	switch len(b.children) {
	case 1:
		panic("branch shouldn't have 0 child after deleting")
	case 2:
		if err := b.delete(cli); err != nil {
			return nil, err
		}
		var orphan node
		var orphanKey byte
		for i, n := range b.children {
			if i != offsetKey {
				orphanKey = i
				orphan = n
				break
			}
		}
		if orphan == nil {
			panic("unexpected branch status")
		}
		if hn, ok := orphan.(*hashNode); ok {
			if orphan, err = hn.LoadNode(cli); err != nil {
				return nil, err
			}
		}
		switch node := orphan.(type) {
		case *extensionNode:
			return node.updatePath(
				cli,
				append([]byte{orphanKey}, node.path...),
				false,
			)
		case *leafNode:
			return node, nil
		default:
			return newExtensionNode(cli, []byte{orphanKey}, node)
		}
	default:
		return b.updateChild(cli, offsetKey, newChild, false)
	}
}

func (b *branchNode) Upsert(cli client, key keyType, offset uint8, value []byte) (node, error) {
	var newChild node
	offsetKey := key[offset]
	child, err := b.child(offsetKey)
	switch errors.Cause(err) {
	case nil:
		newChild, err = child.Upsert(cli, key, offset+1, value) // look for next key offset
	case trie.ErrNotExist:
		newChild, err = newLeafNode(cli, key, value)
	}
	if err != nil {
		return nil, err
	}

	return b.updateChild(cli, offsetKey, newChild, true)
}

func (b *branchNode) Search(cli client, key keyType, offset uint8) (node, error) {
	child, err := b.child(key[offset])
	if err != nil {
		return nil, err
	}
	return child.Search(cli, key, offset+1)
}

func (b *branchNode) proto(cli client, flush bool) (proto.Message, error) {
	nodes := []*triepb.BranchNodePb{}
	for _, idx := range b.indices.List() {
		c := b.children[idx]
		if flush {
			if sn, ok := c.(serializable); ok {
				if err := sn.store(cli); err != nil {
					return nil, err
				}
			}
		}
		h, err := c.Hash(cli)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &triepb.BranchNodePb{Index: uint32(idx), Path: h})
	}
	return &triepb.NodePb{
		Node: &triepb.NodePb_Branch{
			Branch: &triepb.BranchPb{Branches: nodes},
		},
	}, nil
}

func (b *branchNode) child(key byte) (node, error) {
	c, ok := b.children[key]
	if !ok {
		return nil, trie.ErrNotExist
	}
	return c, nil
}

func (b *branchNode) Flush(cli client) error {
	if !b.dirty {
		return nil
	}
	for _, idx := range b.indices.List() {
		if err := b.children[idx].Flush(cli); err != nil {
			return err
		}
	}

	return b.store(cli)
}

func (b *branchNode) updateChild(cli client, key byte, child node, hashnode bool) (node, error) {
	if err := b.delete(cli); err != nil {
		return nil, err
	}
	var indices *SortedList
	var children map[byte]node
	// update branchnode with new child
	if child == nil {
		children = make(map[byte]node, len(b.children)-1)
		for k, v := range b.children {
			if k != key {
				children[k] = v
			}
		}
		if b.indices.sorted {
			indices = b.indices.Clone()
			indices.Delete(key)
		}
	} else {
		children = make(map[byte]node, len(b.children))
		for k, v := range b.children {
			children[k] = v
		}
		children[key] = child
		if b.indices.sorted {
			indices = b.indices.Clone()
			indices.Insert(key)
		}
	}

	if b.isRoot {
		bn, err := newRootBranchNode(cli, children, indices, true)
		if err != nil {
			return nil, err
		}
		return bn, nil
	}
	return newBranchNode(cli, children, indices)
}

func (b *branchNode) Clone() (branch, error) {
	children := make(map[byte]node, len(b.children))
	for key, child := range b.children {
		children[key] = child
	}
	hashVal := make([]byte, len(b.hashVal))
	copy(hashVal, b.hashVal)
	ser := make([]byte, len(b.ser))
	copy(ser, b.ser)
	clone := &branchNode{
		cacheNode: cacheNode{
			dirty:   b.dirty,
			hashVal: hashVal,
			ser:     ser,
		},
		children: children,
		indices:  b.indices,
		isRoot:   b.isRoot,
	}
	clone.cacheNode.serializable = clone
	return clone, nil
}
