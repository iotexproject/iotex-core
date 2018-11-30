// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

// extensionNode defines a node with a path and point to a child node
type extensionNode struct {
	path      []byte
	childHash hash.Hash32B
	ser       []byte
}

func newExtensionNodeFromProtoPb(pb *iproto.ExtendPb) *extensionNode {
	return &extensionNode{
		path:      pb.Path,
		childHash: byteutil.BytesTo32B(pb.Value),
	}
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

func (e *extensionNode) children(tc SameKeyLenTrieContext) ([]Node, error) {
	child, err := e.child(tc)
	if err != nil {
		return nil, err
	}

	return []Node{child}, nil
}

func (e *extensionNode) delete(tc SameKeyLenTrieContext, key keyType, offset uint8) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("delete from an extension")
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, ErrNotExist
	}
	child, err := e.child(tc)
	if err != nil {
		return nil, err
	}
	newChild, err := child.delete(tc, key, offset+matched)
	if err != nil {
		return nil, err
	}
	if newChild == nil {
		return nil, tc.DeleteNodeFromDB(e)
	}
	switch node := newChild.(type) {
	case *extensionNode:
		if err := tc.DeleteNodeFromDB(e); err != nil {
			return nil, err
		}
		return node.updatePath(tc, append(e.path, node.path...))
	case *leafNode:
		if err := tc.DeleteNodeFromDB(e); err != nil {
			return nil, err
		}
		return node, nil
	default:
		return e.updateChild(tc, node)
	}
}

func (e *extensionNode) upsert(tc SameKeyLenTrieContext, key keyType, offset uint8, value []byte) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("upsert into an extension")
	matched := e.commonPrefixLength(key[offset:])
	if matched == uint8(len(e.path)) {
		child, err := e.child(tc)
		if err != nil {
			return nil, err
		}
		newChild, err := child.upsert(tc, key, offset+matched, value)
		if err != nil {
			return nil, err
		}
		return e.updateChild(tc, newChild)
	}
	eb := e.path[matched]
	enode, err := e.updatePath(tc, e.path[matched+1:])
	if err != nil {
		return nil, err
	}
	lnode, err := tc.newLeafNodeAndPutIntoDB(key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := tc.newBranchNodeAndPutIntoDB(map[byte]Node{
		eb:                  enode,
		key[offset+matched]: lnode,
	})
	if err != nil {
		return nil, err
	}
	if matched == 0 {
		return bnode, nil
	}
	return tc.newExtensionNodeAndPutIntoDB(key[offset:offset+matched], bnode)
}

func (e *extensionNode) search(tc SameKeyLenTrieContext, key keyType, offset uint8) Node {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("search in an extension")
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil
	}
	child, err := e.child(tc)
	if err != nil {
		return nil
	}

	return child.search(tc, key, offset+matched)
}

func (e *extensionNode) serialize() []byte {
	if e.ser != nil {
		return e.ser
	}
	pb := &iproto.NodePb{
		Node: &iproto.NodePb_Extend{
			Extend: &iproto.ExtendPb{
				Path:  e.path,
				Value: e.childHash[:],
			},
		},
	}
	ser, err := proto.Marshal(pb)
	if err != nil {
		logger.Panic().
			Err(err).
			Hex("path", e.path).
			Hex("value", e.childHash[:]).
			Msg("failed to marshal an extension node")
	}
	e.ser = ser

	return e.ser
}

func (e *extensionNode) child(tc SameKeyLenTrieContext) (Node, error) {
	child, err := tc.LoadNodeFromDB(e.childHash)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to get the child of an extension, something went wrong with db")
	}

	return child, err
}

func (e *extensionNode) commonPrefixLength(key []byte) uint8 {
	return commonPrefixLength(e.path, key)
}

func (e *extensionNode) updatePath(
	tc SameKeyLenTrieContext, path []byte,
) (*extensionNode, error) {
	if err := tc.DeleteNodeFromDB(e); err != nil {
		return nil, err
	}
	e.path = path
	e.ser = nil
	if err := tc.PutNodeIntoDB(e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *extensionNode) updateChild(
	tc SameKeyLenTrieContext, newChild Node,
) (*extensionNode, error) {
	if err := tc.DeleteNodeFromDB(e); err != nil {
		return nil, err
	}
	e.childHash = nodeHash(newChild)
	e.ser = nil
	if err := tc.PutNodeIntoDB(e); err != nil {
		return nil, err
	}
	return e, nil
}
