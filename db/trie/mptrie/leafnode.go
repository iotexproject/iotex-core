// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"bytes"

	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

type leafNode struct {
	cacheNode
	key   keyType
	value []byte
}

func newLeafNode(
	mpt *merklePatriciaTrie,
	key keyType,
	value []byte,
) (node, error) {
	l := &leafNode{
		cacheNode: cacheNode{
			mpt:   mpt,
			dirty: true,
		},
		key:   key,
		value: value,
	}
	l.cacheNode.serializable = l
	if !mpt.async {
		return l.store()
	}
	return l, nil
}

func newLeafNodeFromProtoPb(mpt *merklePatriciaTrie, pb *triepb.LeafPb) *leafNode {
	l := &leafNode{
		cacheNode: cacheNode{
			mpt: mpt,
		},
		key:   pb.Path,
		value: pb.Value,
	}
	l.cacheNode.serializable = l
	return l
}

func (l *leafNode) Key() []byte {
	return l.key
}

func (l *leafNode) Value() []byte {
	return l.value
}

func (l *leafNode) Delete(key keyType, offset uint8) (node, error) {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, trie.ErrNotExist
	}
	if err := l.delete(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (l *leafNode) Upsert(key keyType, offset uint8, value []byte) (node, error) {
	trieMtc.WithLabelValues("leafNode", "upsert").Inc()
	matched := commonPrefixLength(l.key[offset:], key[offset:])
	if offset+matched == uint8(len(key)) {
		return l.updateValue(value)
	}
	// split into another leaf node and create branch/extension node
	newl, err := newLeafNode(l.mpt, key, value)
	if err != nil {
		return nil, err
	}
	var oldLeaf node
	if !l.mpt.async {
		oldLeaf, err = l.store()
		if err != nil {
			return nil, err
		}
	} else {
		oldLeaf = l
	}
	bnode, err := newBranchNode(
		l.mpt,
		map[byte]node{
			key[offset+matched]:   newl,
			l.key[offset+matched]: oldLeaf,
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

func (l *leafNode) Search(key keyType, offset uint8) (node, error) {
	trieMtc.WithLabelValues("leafNode", "search").Inc()
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, trie.ErrNotExist
	}

	return l, nil
}

func (l *leafNode) proto(_ bool) (proto.Message, error) {
	trieMtc.WithLabelValues("leafNode", "proto").Inc()
	return &triepb.NodePb{
		Node: &triepb.NodePb_Leaf{
			Leaf: &triepb.LeafPb{
				Path:  l.key[:],
				Value: l.value,
			},
		},
	}, nil
}

func (l *leafNode) Flush() error {
	_, err := l.store()
	return err
}

func (l *leafNode) updateValue(value []byte) (node, error) {
	if err := l.delete(); err != nil {
		return nil, err
	}
	l.value = value
	l.dirty = true
	if !l.mpt.async {
		return l.store()
	}
	return l, nil
}
