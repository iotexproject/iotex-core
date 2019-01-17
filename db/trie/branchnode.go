// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

const radix = 256

type branchNode struct {
	hashes map[byte][]byte
	ser    []byte
}

func newEmptyBranchNode() *branchNode {
	return &branchNode{hashes: map[byte][]byte{}}
}

func newBranchNodeAndPutIntoDB(
	tr Trie,
	children map[byte]Node,
) (*branchNode, error) {
	bnode := newEmptyBranchNode()
	for i, n := range children {
		if n == nil {
			continue
		}
		bnode.hashes[i] = tr.nodeHash(n)
	}
	if err := tr.putNodeIntoDB(bnode); err != nil {
		return nil, err
	}
	return bnode, nil
}

func newBranchNodeFromProtoPb(pb *triepb.BranchPb) *branchNode {
	b := newEmptyBranchNode()
	for _, n := range pb.Branches {
		b.hashes[byte(n.Index)] = n.Path
	}
	return b
}

func (b *branchNode) Type() NodeType {
	return BRANCH
}

func (b *branchNode) Key() []byte {
	return nil
}

func (b *branchNode) Value() []byte {
	return nil
}

func (b *branchNode) children(tr Trie) ([]Node, error) {
	trieMtc.WithLabelValues("branchNode", "children").Inc()
	children := []Node{}
	for i := range b.hashes {
		if c, err := b.child(tr, i); err != nil {
			return nil, err
		} else if c != nil {
			children = append(children, c)
		}
	}

	return children, nil
}

func (b *branchNode) delete(tr Trie, key keyType, offset uint8) (Node, error) {
	trieMtc.WithLabelValues("branchNode", "delete").Inc()
	offsetKey := key[offset]
	child, err := b.child(tr, offsetKey)
	if err != nil {
		return nil, err
	}
	newChild, err := child.delete(tr, key, offset+1)
	if err != nil {
		return nil, err
	}
	if newChild != nil {
		return b.updateChild(tr, offsetKey, newChild)
	}
	switch len(b.hashes) {
	case 1:
		panic("branch shouldn't have 0 child after deleting")
	case 2:
		if err := tr.deleteNodeFromDB(b); err != nil {
			return nil, err
		}
		var orphan Node
		var orphanKey byte
		for i, h := range b.hashes {
			if i != offsetKey {
				orphanKey = i
				if orphan, err = tr.loadNodeFromDB(h); err != nil {
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
				tr,
				append([]byte{orphanKey}, node.path...),
			)
		case *leafNode:
			return node, nil
		default:
			return newExtensionNodeAndPutIntoDB(tr, []byte{orphanKey}, node)
		}
	default:
		return b.updateChild(tr, offsetKey, newChild)
	}
}

func (b *branchNode) upsert(tr Trie, key keyType, offset uint8, value []byte) (Node, error) {
	trieMtc.WithLabelValues("branchNode", "upsert").Inc()
	var newChild Node
	offsetKey := key[offset]
	child, err := b.child(tr, offsetKey)
	switch errors.Cause(err) {
	case nil:
		newChild, err = child.upsert(tr, key, offset+1, value)
	case ErrNotExist:
		newChild, err = newLeafNodeAndPutIntoDB(tr, key, value)
	}
	if err != nil {
		return nil, err
	}

	return b.updateChild(tr, offsetKey, newChild)
}

func (b *branchNode) search(tr Trie, key keyType, offset uint8) Node {
	trieMtc.WithLabelValues("branchNode", "search").Inc()
	child, err := b.child(tr, key[offset])
	if errors.Cause(err) == ErrNotExist {
		return nil
	}
	return child.search(tr, key, offset+1)
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

func (b *branchNode) child(tr Trie, key byte) (Node, error) {
	h, ok := b.hashes[key]
	if !ok {
		return nil, ErrNotExist
	}
	child, err := tr.loadNodeFromDB(h)
	if err != nil {
		return nil, errors.Errorf("failed to fetch node for key %x", h)
	}
	return child, nil
}

func (b *branchNode) updateChild(tr Trie, key byte, child Node) (*branchNode, error) {
	if err := tr.deleteNodeFromDB(b); err != nil {
		return nil, err
	}
	b.ser = nil
	if child == nil {
		delete(b.hashes, key)
	} else {
		b.hashes[key] = tr.nodeHash(child)
	}
	if err := tr.putNodeIntoDB(b); err != nil {
		return nil, err
	}
	return b, nil
}
