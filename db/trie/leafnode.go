// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/proto"
)

type leafNode struct {
	key   keyType
	value []byte
	ser   []byte
}

func newLeafNode(
	key keyType,
	value []byte,
) *leafNode {
	logger.Debug().Hex("key", key).Hex("value", value).Msg("new leaf")
	return &leafNode{key: key, value: value}
}

func newLeafNodeFromProtoPb(pb *iproto.LeafPb) *leafNode {
	return &leafNode{key: pb.Path, value: pb.Value}
}

func (l *leafNode) Type() NodeType {
	return LEAF
}

func (l *leafNode) Key() []byte {
	return l.key
}

func (l *leafNode) Value() []byte {
	return l.value
}

func (l *leafNode) children(tc SameKeyLenTrieContext) ([]Node, error) {
	return nil, nil
}

func (l *leafNode) delete(tc SameKeyLenTrieContext, key keyType, offset uint8) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Hex("leaf", l.key[:]).Msg("delete from a leaf")
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, ErrNotExist
	}
	if err := tc.DeleteNodeFromDB(l); err != nil {
		return nil, err
	}
	return nil, nil
}

func (l *leafNode) upsert(tc SameKeyLenTrieContext, key keyType, offset uint8, value []byte) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Hex("leaf", l.key[:]).Msg("upsert into a leaf")
	matched := commonPrefixLength(l.key[offset:], key[offset:])
	if offset+matched == uint8(len(key)) {
		return l.updateValue(tc, value)
	}
	newl, err := tc.newLeafNodeAndPutIntoDB(key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := tc.newBranchNodeAndPutIntoDB(map[byte]Node{
		key[offset+matched]:   newl,
		l.key[offset+matched]: l,
	})
	if err != nil {
		return nil, err
	}
	if matched == 0 {
		return bnode, nil
	}

	return tc.newExtensionNodeAndPutIntoDB(l.key[offset:offset+matched], bnode)
}

func (l *leafNode) search(_ SameKeyLenTrieContext, key keyType, offset uint8) Node {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Hex("leaf", l.key[:]).Msg("search in a leaf")
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil
	}

	return l
}

func (l *leafNode) serialize() []byte {
	if l.ser != nil {
		return l.ser
	}
	pb := &iproto.NodePb{
		Node: &iproto.NodePb_Leaf{
			Leaf: &iproto.LeafPb{
				Path:  l.key[:],
				Value: l.value,
			},
		},
	}
	ser, err := proto.Marshal(pb)
	if err != nil {
		logger.Panic().
			Err(err).
			Hex("key", l.key).
			Hex("value", l.value).
			Msg("failed to marshal a leaf node")
	}
	l.ser = ser

	return l.ser
}

func (l *leafNode) updateValue(
	tc SameKeyLenTrieContext, value []byte,
) (*leafNode, error) {
	if err := tc.DeleteNodeFromDB(l); err != nil {
		return nil, err
	}
	l.value = value
	l.ser = nil
	if err := tc.PutNodeIntoDB(l); err != nil {
		return nil, err
	}

	return l, nil
}
