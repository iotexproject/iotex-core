// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

type leafNode struct {
	key   keyType
	value []byte
	ser   []byte
}

func newLeafNodeAndPutIntoDB(
	tr Trie,
	key keyType,
	value []byte,
) (*leafNode, error) {
	l := &leafNode{key: key, value: value}
	if err := tr.putNodeIntoDB(l); err != nil {
		return nil, err
	}
	return l, nil
}

func newLeafNodeFromProtoPb(pb *triepb.LeafPb) *leafNode {
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

func (l *leafNode) children(Trie) ([]Node, error) {
	trieMtc.WithLabelValues("leafNode", "children").Inc()
	return nil, nil
}

func (l *leafNode) delete(tr Trie, key keyType, offset uint8) (Node, error) {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, ErrNotExist
	}
	if err := tr.deleteNodeFromDB(l); err != nil {
		return nil, err
	}
	return nil, nil
}

func (l *leafNode) upsert(tr Trie, key keyType, offset uint8, value []byte) (Node, error) {
	trieMtc.WithLabelValues("leafNode", "upsert").Inc()
	matched := commonPrefixLength(l.key[offset:], key[offset:])
	if offset+matched == uint8(len(key)) {
		return l.updateValue(tr, value)
	}
	newl, err := newLeafNodeAndPutIntoDB(tr, key, value)
	if err != nil {
		return nil, err
	}
	bnode, err := newBranchNodeAndPutIntoDB(
		tr,
		map[byte]Node{
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

	return newExtensionNodeAndPutIntoDB(tr, l.key[offset:offset+matched], bnode)
}

func (l *leafNode) search(_ Trie, key keyType, offset uint8) Node {
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

func (l *leafNode) updateValue(tr Trie, value []byte) (*leafNode, error) {
	if err := tr.deleteNodeFromDB(l); err != nil {
		return nil, err
	}
	l.value = value
	l.ser = nil
	if err := tr.putNodeIntoDB(l); err != nil {
		return nil, err
	}

	return l, nil
}
