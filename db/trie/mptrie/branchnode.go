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

const radix = 256

type branchNode struct {
	cacheNode
	children *sortedList
	isRoot   bool
}

func newBranchNode(
	mpt *merklePatriciaTrie,
	children map[byte]node,
) (node, error) {
	if len(children) == 0 {
		return nil, errors.New("branch node children cannot be empty")
	}
	bnode := &branchNode{
		cacheNode: cacheNode{
			mpt:   mpt,
			dirty: true,
		},
		children: NewSortList(),
	}
	for k, v := range children {
		bnode.children.Upsert(k, v)
	}
	bnode.cacheNode.serializable = bnode

	if bnode.children.Size() != 0 {
		if !mpt.async {
			return bnode.store()
		}
	}
	return bnode, nil
}

func newEmptyRootBranchNode(mpt *merklePatriciaTrie) *branchNode {
	bnode := &branchNode{
		cacheNode: cacheNode{
			mpt: mpt,
		},
		children: NewSortList(),
		isRoot:   true,
	}
	bnode.cacheNode.serializable = bnode

	return bnode
}

func newBranchNodeFromProtoPb(mpt *merklePatriciaTrie, pb *triepb.BranchPb) *branchNode {
	bnode := &branchNode{
		cacheNode: cacheNode{
			mpt: mpt,
		},
		children: NewSortListFromProtoPb(mpt, pb.Branches),
	}
	bnode.cacheNode.serializable = bnode
	return bnode
}

func (b *branchNode) MarkAsRoot() {
	b.isRoot = true
}

func (b *branchNode) Children() []node {
	trieMtc.WithLabelValues("branchNode", "children").Inc()
	ret := make([]node, 0, b.children.Size())
	for ptr := b.children.Front(); ptr != nil; ptr = ptr.Next() {
		tmp := ptr.Value.(*listElement)
		ret = append(ret, tmp.data)
	}
	return ret
}

func (b *branchNode) Delete(key keyType, offset uint8) (node, error) {
	trieMtc.WithLabelValues("branchNode", "delete").Inc()
	offsetKey := key[offset]
	child, err := b.child(offsetKey)
	if err != nil {
		return nil, err
	}
	newChild, err := child.Delete(key, offset+1)
	if err != nil {
		return nil, err
	}
	if newChild != nil || b.isRoot {
		return b.updateChild(offsetKey, newChild, false)
	}
	switch b.children.Size() {
	case 1:
		panic("branch shouldn't have 0 child after deleting")
	case 2:
		if err := b.delete(); err != nil {
			return nil, err
		}
		var orphan node
		var orphanKey byte
		for ptr := b.children.Front(); ptr != nil; ptr = ptr.Next() {
			val := ptr.Value.(*listElement)
			if val.idx != int16(offsetKey) {
				orphanKey = byte(val.idx)
				orphan = val.data
				break
			}
		}
		if orphan == nil {
			panic("unexpected branch status")
		}
		if hn, ok := orphan.(*hashNode); ok {
			if orphan, err = hn.LoadNode(); err != nil {
				return nil, err
			}
		}
		switch node := orphan.(type) {
		case *extensionNode:
			return node.updatePath(
				append([]byte{orphanKey}, node.path...),
				false,
			)
		case *leafNode:
			return node, nil
		default:
			return newExtensionNode(b.mpt, []byte{orphanKey}, node)
		}
	default:
		return b.updateChild(offsetKey, newChild, false)
	}
}

func (b *branchNode) Upsert(key keyType, offset uint8, value []byte) (node, error) {
	trieMtc.WithLabelValues("branchNode", "upsert").Inc()
	var newChild node
	offsetKey := key[offset]
	child, err := b.child(offsetKey)
	switch errors.Cause(err) {
	case nil:
		newChild, err = child.Upsert(key, offset+1, value) // look for next key offset
	case trie.ErrNotExist:
		newChild, err = newLeafNode(b.mpt, key, value)
	}
	if err != nil {
		return nil, err
	}

	return b.updateChild(offsetKey, newChild, true)
}

func (b *branchNode) Search(key keyType, offset uint8) (node, error) {
	trieMtc.WithLabelValues("branchNode", "search").Inc()
	child, err := b.child(key[offset])
	if err != nil {
		return nil, err
	}
	return child.Search(key, offset+1)
}

func (b *branchNode) proto(flush bool) (proto.Message, error) {
	trieMtc.WithLabelValues("branchNode", "serialize").Inc()
	nodes := []*triepb.BranchNodePb{}
	for ptr := b.children.Front(); ptr != nil; ptr = ptr.Next() {
		val := ptr.Value.(*listElement)
		c := val.data
		if flush {
			if sn, ok := c.(serializable); ok {
				var err error
				c, err = sn.store()
				if err != nil {
					return nil, err
				}
			}
		}
		h, err := c.Hash()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &triepb.BranchNodePb{Index: uint32(val.idx), Path: h})
	}
	return &triepb.NodePb{
		Node: &triepb.NodePb_Branch{
			Branch: &triepb.BranchPb{Branches: nodes},
		},
	}, nil
}

func (b *branchNode) child(key byte) (node, error) {
	c := b.children.Get(key)
	if c == nil {
		return nil, trie.ErrNotExist
	}
	return c, nil
}

func (b *branchNode) Flush() error {
	for ptr := b.children.Front(); ptr != nil; ptr = ptr.Next() {
		c := ptr.Value.(*listElement).data
		if err := c.Flush(); err != nil {
			return err
		}
	}
	_, err := b.store()
	return err
}

func (b *branchNode) updateChild(key byte, child node, hashnode bool) (node, error) {
	if err := b.delete(); err != nil {
		return nil, err
	}
	// update branchnode with new child
	if child == nil {
		if err := b.children.Delete(key); err != nil {
			return nil, err
		}
	} else {
		b.children.Upsert(key, child)
	}
	b.dirty = true
	if b.children.Size() != 0 {
		if !b.mpt.async {
			hn, err := b.store()
			if err != nil {
				return nil, err
			}
			if !b.isRoot && hashnode {
				return hn, nil // return hashnode
			}
		}
	} else {
		if _, err := b.hash(false); err != nil {
			return nil, err
		}
	}
	return b, nil // return branchnode
}
