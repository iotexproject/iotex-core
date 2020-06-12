// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

const radix = 256

type branchNode struct {
	mpt    *merklePatriciaTrie
	hashes map[byte][]byte
	isRoot bool
	ser    []byte
}

func newBranchNode(
	mpt *merklePatriciaTrie,
	children map[byte]node,
) (*branchNode, error) {
	bnode := &branchNode{mpt: mpt, hashes: map[byte][]byte{}}
	for i, n := range children {
		if n == nil {
			continue
		}
		bnode.hashes[i] = mpt.nodeHash(n)
	}
	if len(bnode.hashes) != 0 {
		if err := mpt.putNode(bnode); err != nil {
			return nil, err
		}
	}
	return bnode, nil
}

func newBranchNodeFromProtoPb(mpt *merklePatriciaTrie, pb *triepb.BranchPb) *branchNode {
	b, err := newBranchNode(mpt, nil)
	if err != nil {
		panic(err)
	}
	for _, n := range pb.Branches {
		b.hashes[byte(n.Index)] = n.Path
	}
	return b
}

func (b *branchNode) markAsRoot() {
	b.isRoot = true
}

func (b *branchNode) children() ([]node, error) {
	trieMtc.WithLabelValues("branchNode", "children").Inc()
	children := []node{}
	for i := range b.hashes {
		if c, err := b.child(i); err != nil {
			return nil, err
		} else if c != nil {
			children = append(children, c)
		}
	}

	return children, nil
}

func (b *branchNode) delete(key keyType, offset uint8) (node, error) {
	trieMtc.WithLabelValues("branchNode", "delete").Inc()
	offsetKey := key[offset]
	child, err := b.child(offsetKey)
	if err != nil {
		return nil, err
	}
	newChild, err := child.delete(key, offset+1)
	if err != nil {
		return nil, err
	}
	if newChild != nil || b.isRoot {
		return b.updateChild(offsetKey, newChild)
	}
	switch len(b.hashes) {
	case 1:
		panic("branch shouldn't have 0 child after deleting")
	case 2:
		if err := b.mpt.deleteNode(b); err != nil {
			return nil, err
		}
		var orphan node
		var orphanKey byte
		for i, h := range b.hashes {
			if i != offsetKey {
				orphanKey = i
				if orphan, err = b.mpt.loadNode(h); err != nil {
					return nil, err
				}
				break
			}
		}
		if orphan == nil {
			panic("unexpected branch status")
		}
		switch node := orphan.(type) {
		case *extensionNode:
			return node.updatePath(
				append([]byte{orphanKey}, node.path...),
			)
		case *leafNode:
			return node, nil
		default:
			return newExtensionNode(b.mpt, []byte{orphanKey}, node)
		}
	default:
		return b.updateChild(offsetKey, newChild)
	}
}

func (b *branchNode) upsert(key keyType, offset uint8, value []byte) (node, error) {
	trieMtc.WithLabelValues("branchNode", "upsert").Inc()
	var newChild node
	offsetKey := key[offset]
	child, err := b.child(offsetKey)
	switch errors.Cause(err) {
	case nil:
		newChild, err = child.upsert(key, offset+1, value)
	case trie.ErrNotExist:
		newChild, err = newLeafNode(b.mpt, key, value)
	}
	if err != nil {
		return nil, err
	}

	return b.updateChild(offsetKey, newChild)
}

func (b *branchNode) search(key keyType, offset uint8) node {
	trieMtc.WithLabelValues("branchNode", "search").Inc()
	child, err := b.child(key[offset])
	if errors.Cause(err) == trie.ErrNotExist {
		return nil
	}
	return child.search(key, offset+1)
}

func (b *branchNode) serialize() []byte {
	trieMtc.WithLabelValues("branchNode", "serialize").Inc()
	if b.ser != nil {
		return b.ser
	}
	nodes := []*triepb.BranchNodePb{}
	for index := 0; index < radix; index++ {
		if h, ok := b.hashes[byte(index)]; ok {
			nodes = append(nodes, &triepb.BranchNodePb{Index: uint32(index), Path: h})
		}
	}
	pb := &triepb.NodePb{
		Node: &triepb.NodePb_Branch{
			Branch: &triepb.BranchPb{Branches: nodes},
		},
	}
	ser, err := proto.Marshal(pb)
	if err != nil {
		panic("failed to marshal a branch node")
	}
	b.ser = ser

	return b.ser
}

func (b *branchNode) child(key byte) (node, error) {
	h, ok := b.hashes[key]
	if !ok {
		return nil, trie.ErrNotExist
	}
	child, err := b.mpt.loadNode(h)
	if err != nil {
		return nil, errors.Errorf("failed to fetch node for key %x", h)
	}
	return child, nil
}

func (b *branchNode) updateChild(key byte, child node) (*branchNode, error) {
	if err := b.mpt.deleteNode(b); err != nil {
		return nil, err
	}
	b.ser = nil
	if child == nil {
		delete(b.hashes, key)
	} else {
		b.hashes[key] = b.mpt.nodeHash(child)
	}
	if !b.isRoot || len(b.hashes) != 0 {
		if err := b.mpt.putNode(b); err != nil {
			return nil, err
		}
	}
	return b, nil
}
