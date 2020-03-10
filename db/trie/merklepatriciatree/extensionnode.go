// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package merklepatriciatree

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

// extensionNode defines a node with a path and point to a child node
type extensionNode struct {
	cacheNode
	path  []byte
	child node
}

func newExtensionNode(
	mpt *merklePatriciaTree,
	path []byte,
	child node,
) (*extensionNode, error) {
	e := &extensionNode{cacheNode: cacheNode{mpt: mpt}, path: path, child: child}
	e.cacheNode.node = e
	if err := mpt.putNode(e); err != nil {
		return nil, err
	}
	return e, nil
}

func newExtensionNodeFromProtoPb(mpt *merklePatriciaTree, pb *triepb.ExtendPb) *extensionNode {
	e := &extensionNode{cacheNode: cacheNode{mpt: mpt}, path: pb.Path, child: newHashNode(mpt, pb.Value)}
	e.cacheNode.node = e
	return e
}

func (e *extensionNode) Delete(key keyType, offset uint8) (node, error) {
	trieMtc.WithLabelValues("extensionNode", "delete").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, trie.ErrNotExist
	}
	newChild, err := e.child.Delete(key, offset+matched)
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

func (e *extensionNode) Upsert(key keyType, offset uint8, value []byte) (node, error) {
	trieMtc.WithLabelValues("extensionNode", "upsert").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched == uint8(len(e.path)) {
		newChild, err := e.child.Upsert(key, offset+matched, value)
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

func (e *extensionNode) Search(key keyType, offset uint8) (node, error) {
	trieMtc.WithLabelValues("extensionNode", "search").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, trie.ErrNotExist
	}

	return e.child.Search(key, offset+matched)
}

func (e *extensionNode) Proto() (proto.Message, error) {
	trieMtc.WithLabelValues("extensionNode", "serialize").Inc()
	h, err := e.child.Hash()
	if err != nil {
		return nil, err
	}
	return &triepb.NodePb{
		Node: &triepb.NodePb_Extend{
			Extend: &triepb.ExtendPb{
				Path:  e.path,
				Value: h,
			},
		},
	}, nil
}

func (e *extensionNode) Child() node {
	trieMtc.WithLabelValues("extensionNode", "child").Inc()
	return e.child
}

func (e *extensionNode) commonPrefixLength(key []byte) uint8 {
	return commonPrefixLength(e.path, key)
}

func (e *extensionNode) updatePath(path []byte) (*extensionNode, error) {
	if err := e.mpt.deleteNode(e); err != nil {
		return nil, err
	}
	e.path = path
	e.reset()
	if err := e.mpt.putNode(e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *extensionNode) updateChild(newChild node) (*extensionNode, error) {
	err := e.mpt.deleteNode(e)
	if err != nil {
		return nil, err
	}
	e.child = newChild
	e.reset()
	if err := e.mpt.putNode(e); err != nil {
		return nil, err
	}
	return e, nil
}
