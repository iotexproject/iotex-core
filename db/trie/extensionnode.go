// Copyright (c) 2019 IoTeX
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
	path  []byte
	child Node
	cache nodeCache
}

func newExtensionNode(path []byte, child Node) *extensionNode {
	return &extensionNode{path: path, child: child, cache: nodeCache{dirty: true}}
}

func (e *extensionNode) Type() NodeType {
	return EXTENSION
}

func (e *extensionNode) Children() []Node {
	trieMtc.WithLabelValues("extensionNode", "children").Inc()
	return []Node{e.child}
}

func (e *extensionNode) delete(tr Trie, key keyType, offset uint8) (Node, error) {
	trieMtc.WithLabelValues("extensionNode", "delete").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, ErrNotExist
	}
	if e.child == nil {
		panic("invalid extension node")
	}
	newChild, err := e.child.delete(tr, key, offset+matched)
	if err != nil {
		return nil, err
	}
	switch node := newChild.(type) {
	case nil:
		return nil, nil
	case *extensionNode:
		return node.updatePath(append(e.path, node.path...)), nil
	case *leafNode:
		return node, nil
	default:
		return e.updateChild(node), nil
	}
}

func (e *extensionNode) upsert(tr Trie, key keyType, offset uint8, value []byte) (Node, error) {
	trieMtc.WithLabelValues("extensionNode", "upsert").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched == uint8(len(e.path)) {
		newChild, err := e.child.upsert(tr, key, offset+matched, value)
		if err != nil {
			return nil, err
		}

		return e.updateChild(newChild), nil
	}
	eb := e.path[matched]
	enode := e.updatePath(e.path[matched+1:])
	lnode := newLeafNode(key, value)
	bnode := newBranchNode(
		map[byte]Node{
			eb:                  enode,
			key[offset+matched]: lnode,
		},
	)
	if matched == 0 {
		return bnode, nil
	}
	return newExtensionNode(key[offset:offset+matched], bnode), nil
}

func (e *extensionNode) search(tr Trie, key keyType, offset uint8) (Node, error) {
	trieMtc.WithLabelValues("extensionNode", "search").Inc()
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, ErrNotExist
	}
	if e.child == nil {
		panic("invalid extension node")
	}

	return e.child.search(tr, key, offset+matched)
}

func (e *extensionNode) isDirty() bool {
	return e.cache.dirty
}

func (e *extensionNode) hash(tr Trie, flush bool) ([]byte, error) {
	if e.cache.hn != nil {
		if e.isDirty() && flush {
			if _, err := e.child.hash(tr, flush); err != nil {
				return nil, err
			}
		}
	} else {
		s, err := e.serialize(tr, flush)
		if err != nil {
			return nil, err
		}
		e.cache.hn = newHashNodeFromSer(tr, s)
	}
	if e.isDirty() && flush {
		if err := tr.putNodeIntoDB(e.cache.hn); err != nil {
			return nil, err
		}
	}
	return e.cache.hn.getHash(), nil
}

func (e *extensionNode) serialize(tr Trie, flush bool) ([]byte, error) {
	trieMtc.WithLabelValues("extensionNode", "serialize").Inc()
	childHash, err := e.child.hash(tr, flush)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(&triepb.NodePb{
		Node: &triepb.NodePb_Extend{
			Extend: &triepb.ExtendNodePb{
				Path:      e.path,
				ChildHash: childHash,
			},
		},
	})
}

func (e *extensionNode) commonPrefixLength(key []byte) uint8 {
	return commonPrefixLength(e.path, key)
}

func (e *extensionNode) updatePath(path []byte) *extensionNode {
	return newExtensionNode(path, e.child)
}

func (e *extensionNode) updateChild(newChild Node) *extensionNode {
	return newExtensionNode(e.path, newChild)
}
