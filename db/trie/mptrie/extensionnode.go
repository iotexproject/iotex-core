// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

// extensionNode defines a node with a path and point to a child node
type extensionNode struct {
	mpt       *merklePatriciaTrie
	path      []byte
	childHash []byte
	ser       []byte
}

func newExtensionNode(
	mpt *merklePatriciaTrie,
	path []byte,
	child node,
) (*extensionNode, error) {
	e := &extensionNode{mpt: mpt, path: path, childHash: mpt.nodeHash(child)}
	if err := mpt.putNode(e); err != nil {
		return nil, err
	}
	return e, nil
}

func newExtensionNodeFromProtoPb(mpt *merklePatriciaTrie, pb *triepb.ExtendPb) *extensionNode {
	return &extensionNode{mpt: mpt, path: pb.Path, childHash: pb.Value}
}

func (e *extensionNode) delete(key keyType, offset uint8) (node, error) {
	trieMtc.WithLabelValues("extensionNode", "delete").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, trie.ErrNotExist
	}
	child, err := e.child()
	if err != nil {
		return nil, err
	}
	newChild, err := child.delete(key, offset+matched)
	if err != nil {
		return nil, err
	}
	if newChild == nil {
		return nil, e.mpt.deleteNode(e)
	}
	switch node := newChild.(type) {
	case *extensionNode:
		if err := e.mpt.deleteNode(e); err != nil {
			return nil, err
		}
		return node.updatePath(append(e.path, node.path...))
	case *leafNode:
		if err := e.mpt.deleteNode(e); err != nil {
			return nil, err
		}
		return node, nil
	default:
		return e.updateChild(node)
	}
}

func (e *extensionNode) upsert(key keyType, offset uint8, value []byte) (node, error) {
	trieMtc.WithLabelValues("extensionNode", "upsert").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched == uint8(len(e.path)) {
		child, err := e.child()
		if err != nil {
			return nil, err
		}
		newChild, err := child.upsert(key, offset+matched, value)
		if err != nil {
			return nil, err
		}
		return e.updateChild(newChild)
	}
	eb := e.path[matched]
	enode, err := e.updatePath(e.path[matched+1:])
	if err != nil {
		return nil, err
	}
	lnode, err := newLeafNode(e.mpt, key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := newBranchNode(
		e.mpt,
		map[byte]node{
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
	return newExtensionNode(e.mpt, key[offset:offset+matched], bnode)
}

func (e *extensionNode) search(key keyType, offset uint8) node {
	trieMtc.WithLabelValues("extensionNode", "search").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil
	}
	child, err := e.child()
	if err != nil {
		return nil
	}

	return child.search(key, offset+matched)
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

func (e *extensionNode) child() (node, error) {
	trieMtc.WithLabelValues("extensionNode", "child").Inc()
	return e.mpt.loadNode(e.childHash)
}

func (e *extensionNode) commonPrefixLength(key []byte) uint8 {
	return commonPrefixLength(e.path, key)
}

func (e *extensionNode) updatePath(path []byte) (*extensionNode, error) {
	if err := e.mpt.deleteNode(e); err != nil {
		return nil, err
	}
	e.path = path
	e.ser = nil
	if err := e.mpt.putNode(e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *extensionNode) updateChild(newChild node) (*extensionNode, error) {
	if err := e.mpt.deleteNode(e); err != nil {
		return nil, err
	}
	e.childHash = e.mpt.nodeHash(newChild)
	e.ser = nil
	if err := e.mpt.putNode(e); err != nil {
		return nil, err
	}
	return e, nil
}
