// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"bytes"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

type leafNode struct {
	mpt   *merklePatriciaTrie
	key   keyType
	value []byte
	ser   []byte
}

func newLeafNode(
	mpt *merklePatriciaTrie,
	key keyType,
	value []byte,
) (*leafNode, error) {
	l := &leafNode{mpt: mpt, key: key, value: value}
	if err := mpt.putNode(l); err != nil {
		return nil, err
	}
	return l, nil
}

func newLeafNodeFromProtoPb(mpt *merklePatriciaTrie, pb *triepb.LeafPb) *leafNode {
	return &leafNode{mpt: mpt, key: pb.Path, value: pb.Value}
}

func (l *leafNode) Key() []byte {
	return l.key
}

func (l *leafNode) Value() []byte {
	return l.value
}

func (l *leafNode) delete(key keyType, offset uint8) (node, error) {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, trie.ErrNotExist
	}
	if err := l.mpt.deleteNode(l); err != nil {
		return nil, err
	}
	return nil, nil
}

func (l *leafNode) upsert(key keyType, offset uint8, value []byte) (node, error) {
	trieMtc.WithLabelValues("leafNode", "upsert").Inc()
	matched := commonPrefixLength(l.key[offset:], key[offset:])
	if offset+matched == uint8(len(key)) {
		return l.updateValue(value)
	}
	newl, err := newLeafNode(l.mpt, key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := newBranchNode(
		l.mpt,
		map[byte]node{
			key[offset+matched]:   newl,
			l.key[offset+matched]: l,
		},
	)
	if err != nil {
		return nil, err
	}
	if matched == 0 {
		return bnode, nil
	}

	return newExtensionNode(l.mpt, l.key[offset:offset+matched], bnode)
}

func (l *leafNode) search(key keyType, offset uint8) node {
	trieMtc.WithLabelValues("leafNode", "search").Inc()
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil
	}

	return l
}

func (l *leafNode) serialize() []byte {
	trieMtc.WithLabelValues("leafNode", "serialize").Inc()
	if l.ser != nil {
		return l.ser
	}
	pb := &triepb.NodePb{
		Node: &triepb.NodePb_Leaf{
			Leaf: &triepb.LeafPb{
				Path:  l.key[:],
				Value: l.value,
			},
		},
	}
	ser, err := proto.Marshal(pb)
	if err != nil {
		panic("failed to marshal a leaf node")
	}
	l.ser = ser

	return l.ser
}

func (l *leafNode) updateValue(value []byte) (*leafNode, error) {
	if err := l.mpt.deleteNode(l); err != nil {
		return nil, err
	}
	l.value = value
	l.ser = nil
	if err := l.mpt.putNode(l); err != nil {
		return nil, err
	}

	return l, nil
}
