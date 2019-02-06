// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

// extensionNode defines a node with a path and point to a child node
type extensionNode struct {
	path      []byte
	childHash []byte
	ser       []byte
}

func newExtensionNodeAndPutIntoDB(
	tr Trie,
	path []byte,
	child Node,
) (*extensionNode, error) {
	e := &extensionNode{path: path, childHash: tr.nodeHash(child)}
	if err := tr.putNodeIntoDB(e); err != nil {
		return nil, err
	}
	return e, nil
}

func newExtensionNodeFromProtoPb(pb *triepb.ExtendPb) *extensionNode {
	return &extensionNode{path: pb.Path, childHash: pb.Value}
}

func (e *extensionNode) Type() NodeType {
	return EXTENSION
}

func (e *extensionNode) Key() []byte {
	return nil
}

func (e *extensionNode) Value() []byte {
	return nil
}

func (e *extensionNode) children(tr Trie) ([]Node, error) {
	trieMtc.WithLabelValues("extensionNode", "children").Inc()
	child, err := e.child(tr)
	if err != nil {
		return nil, err
	}

	return []Node{child}, nil
}

func (e *extensionNode) delete(tr Trie, key keyType, offset uint8) (Node, error) {
	trieMtc.WithLabelValues("extensionNode", "delete").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, ErrNotExist
	}
	child, err := e.child(tr)
	if err != nil {
		return nil, err
	}
	newChild, err := child.delete(tr, key, offset+matched)
	if err != nil {
		return nil, err
	}
	if newChild == nil {
		return nil, tr.deleteNodeFromDB(e)
	}
	switch node := newChild.(type) {
	case *extensionNode:
		if err := tr.deleteNodeFromDB(e); err != nil {
			return nil, err
		}
		return node.updatePath(tr, append(e.path, node.path...))
	case *leafNode:
		if err := tr.deleteNodeFromDB(e); err != nil {
			return nil, err
		}
		return node, nil
	default:
		return e.updateChild(tr, node)
	}
}

func (e *extensionNode) upsert(tr Trie, key keyType, offset uint8, value []byte) (Node, error) {
	trieMtc.WithLabelValues("extensionNode", "upsert").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched == uint8(len(e.path)) {
		child, err := e.child(tr)
		if err != nil {
			return nil, err
		}
		newChild, err := child.upsert(tr, key, offset+matched, value)
		if err != nil {
			return nil, err
		}
		return e.updateChild(tr, newChild)
	}
	eb := e.path[matched]
	enode, err := e.updatePath(tr, e.path[matched+1:])
	if err != nil {
		return nil, err
	}
	lnode, err := newLeafNodeAndPutIntoDB(tr, key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := newBranchNodeAndPutIntoDB(
		tr,
		map[byte]Node{
			eb:                  enode,
			key[offset+matched]: lnode,
		},
	)
	if err != nil {
		return nil, err
	}
	if matched == 0 {
		return bnode, nil
	}
	return newExtensionNodeAndPutIntoDB(tr, key[offset:offset+matched], bnode)
}

func (e *extensionNode) search(tr Trie, key keyType, offset uint8) Node {
	trieMtc.WithLabelValues("extensionNode", "search").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil
	}
	child, err := e.child(tr)
	if err != nil {
		return nil
	}

	return child.search(tr, key, offset+matched)
}

func (e *extensionNode) serialize() []byte {
	trieMtc.WithLabelValues("extensionNode", "serialize").Inc()
	if e.ser != nil {
		return e.ser
	}
	pb := &triepb.NodePb{
		Node: &triepb.NodePb_Extend{
			Extend: &triepb.ExtendPb{
				Path:  e.path,
				Value: e.childHash,
			},
		},
	}
	ser, err := proto.Marshal(pb)
	if err != nil {
		panic("failed to marshal an extension node")
	}
	e.ser = ser

	return e.ser
}

func (e *extensionNode) child(tr Trie) (Node, error) {
	return tr.loadNodeFromDB(e.childHash)
}

func (e *extensionNode) commonPrefixLength(key []byte) uint8 {
	return commonPrefixLength(e.path, key)
}

func (e *extensionNode) updatePath(tr Trie, path []byte) (*extensionNode, error) {
	if err := tr.deleteNodeFromDB(e); err != nil {
		return nil, err
	}
	e.path = path
	e.ser = nil
	if err := tr.putNodeIntoDB(e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *extensionNode) updateChild(tr Trie, newChild Node) (*extensionNode, error) {
	if err := tr.deleteNodeFromDB(e); err != nil {
		return nil, err
	}
	e.childHash = tr.nodeHash(newChild)
	e.ser = nil
	if err := tr.putNodeIntoDB(e); err != nil {
		return nil, err
	}
	return e, nil
}
