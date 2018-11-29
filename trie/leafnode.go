// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/proto"
)

type leafNode struct {
	key   keyType
	value []byte
	ser   []byte
}

func newLeafNodeAndSave(
	tc SameKeyLenTrieContext, key keyType,
	value []byte,
) (*leafNode, error) {
	logger.Debug().Hex("key", key).Hex("value", value).Msg("new leaf")
	l := &leafNode{key: key, value: value}
	if err := putNodeIntoDB(tc, l); err != nil {
		return nil, err
	}
	return l, nil
}

func newLeafNodeFromProtoPb(pb *iproto.LeafPb) *leafNode {
	return &leafNode{key: pb.Path, value: pb.Value}
}

func (l *leafNode) Children(context.Context) ([]Node, error) {
	return nil, errors.New("leaf node has no child")
}

func (l *leafNode) delete(tc SameKeyLenTrieContext, key keyType, offset uint8) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Hex("leaf", l.key[:]).Msg("delete from a leaf")
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, ErrNotExist
	}
	if err := deleteNodeFromDB(tc, l); err != nil {
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
	newl, err := newLeafNodeAndSave(tc, key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := newBranchNodeAndSave(tc, map[byte]Node{
		key[offset+matched]:   newl,
		l.key[offset+matched]: l,
	})
	if err != nil {
		return nil, err
	}
	if matched == 0 {
		return bnode, nil
	}

	return newExtensionNodeAndSave(tc, l.key[offset:offset+matched], bnode)
}

func (l *leafNode) search(tc SameKeyLenTrieContext, key keyType, offset uint8) Node {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Hex("leaf", l.key[:]).Msg("search in a leaf")
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil
	}

	return l
}

func (l *leafNode) serialize() ([]byte, error) {
	if l.ser != nil {
		return l.ser, nil
	}
	pb := &iproto.NodePb{
		Node: &iproto.NodePb_Leaf{
			Leaf: &iproto.LeafPb{
				Ext:   0,
				Path:  l.key[:],
				Value: l.value,
			},
		},
	}
	var err error
	l.ser, err = proto.Marshal(pb)

	return l.ser, err
}

func (l *leafNode) updateValue(
	tc SameKeyLenTrieContext, value []byte,
) (*leafNode, error) {
	if err := deleteNodeFromDB(tc, l); err != nil {
		return nil, err
	}
	l.value = value
	l.ser = nil
	if err := putNodeIntoDB(tc, l); err != nil {
		return nil, err
	}

	return l, nil
}

func (l *leafNode) Value() []byte {
	return l.value
}
