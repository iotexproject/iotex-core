// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

type extensionNode struct {
	path      []byte
	childHash hash.Hash32B
	ser       []byte
}

func newExtensionNodeAndSave(
	tc SameKeyLenTrieContext, path []byte,
	child Node,
) (Node, error) {
	h, err := nodeHash(child)
	if err != nil {
		return nil, err
	}
	e := &extensionNode{path: path, childHash: h}
	if err := putNodeIntoDB(tc, e); err != nil {
		return nil, err
	}
	return e, nil
}

func newExtensionNodeFromProtoPb(pb *iproto.ExtendPb) *extensionNode {
	return &extensionNode{
		path:      pb.Path,
		childHash: byteutil.BytesTo32B(pb.Value),
	}
}

func (e *extensionNode) Children(ctx context.Context) ([]Node, error) {
	tc, ok := getSameKeyLenTrieContext(ctx)
	if !ok {
		return nil, errors.New("failed to get operation context")
	}

	return []Node{e.child(tc)}, nil
}

func (e *extensionNode) delete(tc SameKeyLenTrieContext, key keyType, offset uint8) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("delete from an extension")
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil, ErrNotExist
	}
	newChild, err := e.child(tc).delete(tc, key, offset+matched)
	if err != nil {
		return nil, err
	}
	if newChild == nil {
		err := deleteNodeFromDB(tc, e)
		return nil, err
	}
	switch node := newChild.(type) {
	case *extensionNode:
		if err := deleteNodeFromDB(tc, e); err != nil {
			return nil, err
		}
		return node.updatePath(tc, append(e.path, node.path...))
	case *leafNode:
		if err := deleteNodeFromDB(tc, e); err != nil {
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
		newChild, err := e.child(tc).upsert(tc, key, offset+matched, value)
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
	lnode, err := newLeafNodeAndSave(tc, key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := newBranchNodeAndSave(tc, map[byte]Node{
		eb:                  enode,
		key[offset+matched]: lnode,
	})
	if err != nil {
		return nil, err
	}
	if matched == 0 {
		return bnode, nil
	}
	return newExtensionNodeAndSave(tc, key[offset:offset+matched], bnode)
}

func (e *extensionNode) search(tc SameKeyLenTrieContext, key keyType, offset uint8) Node {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("search in an extension")
	matched := e.commonPrefixLength(key[offset:])
	if matched != uint8(len(e.path)) {
		return nil
	}

	return e.child(tc).search(tc, key, offset+matched)
}

func (e *extensionNode) serialize() ([]byte, error) {
	if e.ser != nil {
		return e.ser, nil
	}
	pb := &iproto.NodePb{
		Node: &iproto.NodePb_Extend{
			Extend: &iproto.ExtendPb{
				Path:  e.path,
				Value: e.childHash[:],
			},
		},
	}
	var err error
	e.ser, err = proto.Marshal(pb)

	return e.ser, err
}

func (e *extensionNode) child(tc SameKeyLenTrieContext) Node {
	child, err := loadNodeFromDB(tc, e.childHash)
	if err != nil {
		// TODO: handle panic error
		panic(err)
	}
	return child
}

func (e *extensionNode) commonPrefixLength(key []byte) uint8 {
	return commonPrefixLength(e.path, key)
}

func (e *extensionNode) updatePath(
	tc SameKeyLenTrieContext, path []byte,
) (*extensionNode, error) {
	if err := deleteNodeFromDB(tc, e); err != nil {
		return nil, err
	}
	e.path = path
	e.ser = nil
	if err := putNodeIntoDB(tc, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *extensionNode) updateChild(
	tc SameKeyLenTrieContext, newChild Node,
) (*extensionNode, error) {
	childHash, err := nodeHash(newChild)
	if err != nil {
		return nil, err
	}
	if err := deleteNodeFromDB(tc, e); err != nil {
		return nil, err
	}
	e.childHash = childHash
	e.ser = nil
	if err := putNodeIntoDB(tc, e); err != nil {
		return nil, err
	}
	return e, nil
}
