// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"google.golang.org/protobuf/proto"

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
	cli client,
	path []byte,
	child node,
) (node, error) {
	e := &extensionNode{
		cacheNode: cacheNode{
			dirty: true,
		},
		path:  path,
		child: child,
	}
	e.cacheNode.serializable = e

	if !cli.asyncMode() {
		if err := e.store(cli); err != nil {
			return nil, err
		}
	}
	return e, nil
}

func newExtensionNodeFromProtoPb(cli client, pb *triepb.ExtendPb, hashVal []byte) *extensionNode {
	e := &extensionNode{
		cacheNode: cacheNode{
			hashVal: hashVal,
			dirty:   false,
		},
		path:  pb.Path,
		child: newHashNode(cli, pb.Value),
	}
	e.cacheNode.serializable = e
	return e
}

func (e *extensionNode) Delete(cli client, key keyType, offset uint8) (node, error) {
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, trie.ErrNotExist
	}
	newChild, err := e.child.Delete(cli, key, offset+matched)
	if err != nil {
		return nil, err
	}
	if newChild == nil {
		return nil, e.delete(cli)
	}
	if hn, ok := newChild.(*hashNode); ok {
		if newChild, err = hn.LoadNode(cli); err != nil {
			return nil, err
		}
	}
	switch node := newChild.(type) {
	case *extensionNode:
		return node.updatePath(cli, append(e.path, node.path...), false)
	case *branchNode:
		return e.updateChild(cli, node, false)
	default:
		if err := e.delete(cli); err != nil {
			return nil, err
		}
		return node, nil
	}
}

func (e *extensionNode) Upsert(cli client, key keyType, offset uint8, value []byte) (node, error) {
	matched := e.commonPrefixLength(key[offset:])
	if matched == uint8(len(e.path)) {
		newChild, err := e.child.Upsert(cli, key, offset+matched, value)
		if err != nil {
			return nil, err
		}
		return e.updateChild(cli, newChild, true)
	}
	eb := e.path[matched]
	enode, err := e.updatePath(cli, e.path[matched+1:], true)
	if err != nil {
		return nil, err
	}
	lnode, err := newLeafNode(cli, key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := newBranchNode(
		cli,
		map[byte]node{
			eb:                  enode,
			key[offset+matched]: lnode,
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	if matched == 0 {
		return bnode, nil
	}
	return newExtensionNode(cli, key[offset:offset+matched], bnode)
}

func (e *extensionNode) Search(cli client, key keyType, offset uint8) (node, error) {
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, trie.ErrNotExist
	}

	return e.child.Search(cli, key, offset+matched)
}

func (e *extensionNode) proto(cli client, flush bool) (proto.Message, error) {
	if flush {
		if sn, ok := e.child.(serializable); ok {
			if err := sn.store(cli); err != nil {
				return nil, err
			}
		}
	}
	h, err := e.child.Hash(cli)
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
	return e.child
}

func (e *extensionNode) commonPrefixLength(key []byte) uint8 {
	return commonPrefixLength(e.path, key)
}

func (e *extensionNode) Flush(cli client) error {
	if !e.dirty {
		return nil
	}
	if err := e.child.Flush(cli); err != nil {
		return err
	}

	return e.store(cli)
}

func (e *extensionNode) updatePath(cli client, path []byte, hashnode bool) (node, error) {
	if err := e.delete(cli); err != nil {
		return nil, err
	}
	return newExtensionNode(cli, path, e.child)
}

func (e *extensionNode) updateChild(cli client, newChild node, hashnode bool) (node, error) {
	err := e.delete(cli)
	if err != nil {
		return nil, err
	}
	path := make([]byte, len(e.path))
	copy(path, e.path)
	return newExtensionNode(cli, path, newChild)
}
