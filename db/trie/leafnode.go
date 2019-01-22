// Copyright (c) 2019 IoTeX
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
	cache nodeCache
}

func newLeafNode(key keyType, value []byte) *leafNode {
	return &leafNode{key: key, value: value, cache: nodeCache{dirty: true}}
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

func (l *leafNode) Children() []Node {
	trieMtc.WithLabelValues("leafNode", "children").Inc()
	return nil
}

func (l *leafNode) delete(_ Trie, key keyType, offset uint8) (Node, error) {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, ErrNotExist
	}

	return nil, nil
}

func (l *leafNode) upsert(tr Trie, key keyType, offset uint8, value []byte) (Node, error) {
	trieMtc.WithLabelValues("leafNode", "upsert").Inc()
	matched := commonPrefixLength(l.key[offset:], key[offset:])
	if offset+matched == uint8(len(key)) {
		return l.updateValue(value), nil
	}
	newl := newLeafNode(key, value)
	bnode := newBranchNode(
		map[byte]Node{
			key[offset+matched]:   newl,
			l.key[offset+matched]: l,
		},
	)
	if matched == 0 {
		return bnode, nil
	}

	return newExtensionNode(l.key[offset:offset+matched], bnode), nil
}

func (l *leafNode) search(_ Trie, key keyType, offset uint8) (Node, error) {
	trieMtc.WithLabelValues("leafNode", "search").Inc()
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, ErrNotExist
	}

	return l, nil
}

func (l *leafNode) isDirty() bool {
	return l.cache.dirty
}

func (l *leafNode) hash(tr Trie, flush bool) ([]byte, error) {
	if l.cache.hn == nil {
		s, err := l.serialize()
		if err != nil {
			return nil, err
		}
		l.cache.hn = newHashNodeFromSer(tr, s)
	}
	if l.isDirty() && flush {
		if err := tr.putNodeIntoDB(l.cache.hn); err != nil {
			return nil, err
		}
	}
	return l.cache.hn.getHash(), nil
}

func (l *leafNode) serialize() ([]byte, error) {
	trieMtc.WithLabelValues("leafNode", "serialize").Inc()
	return proto.Marshal(&triepb.NodePb{
		Node: &triepb.NodePb_Leaf{
			Leaf: &triepb.LeafNodePb{
				Key:   l.key[:],
				Value: l.value,
			},
		},
	})
}

func (l *leafNode) updateValue(value []byte) *leafNode {
	if bytes.Equal(value, l.value) {
		return l
	}
	return newLeafNode(l.key, value)
}
