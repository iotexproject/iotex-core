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
	cli client,
	key keyType,
	value []byte,
) (node, error) {
	l := &leafNode{
		cacheNode: cacheNode{
			dirty: true,
		},
		key:   key,
		value: value,
	}
	l.cacheNode.serializable = l
	if !cli.asyncMode() {
		if err := l.store(cli); err != nil {
			return nil, err
		}
	}
	return l, nil
}

func newLeafNodeFromProtoPb(pb *triepb.LeafPb, hashVal []byte) *leafNode {
	l := &leafNode{
		cacheNode: cacheNode{
			hashVal: hashVal,
			dirty:   false,
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

func (l *leafNode) Delete(cli client, key keyType, offset uint8) (node, error) {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, trie.ErrNotExist
	}
	return nil, l.delete(cli)
}

func (l *leafNode) Upsert(cli client, key keyType, offset uint8, value []byte) (node, error) {
	matched := commonPrefixLength(l.key[offset:], key[offset:])
	if offset+matched == uint8(len(key)) {
		if err := l.delete(cli); err != nil {
			return nil, err
		}
		return newLeafNode(cli, key, value)
	}
	// split into another leaf node and create branch/extension node
	newl, err := newLeafNode(cli, key, value)
	if err != nil {
		return nil, err
	}
	oldLeaf := l
	if !cli.asyncMode() {
		if err := l.store(cli); err != nil {
			return nil, err
		}
	}
	bnode, err := newBranchNode(
		cli,
		map[byte]node{
			key[offset+matched]:   newl,
			l.key[offset+matched]: oldLeaf,
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	if matched == 0 {
		return bnode, nil
	}

	return newExtensionNode(cli, l.key[offset:offset+matched], bnode)
}

func (l *leafNode) Search(_ client, key keyType, offset uint8) (node, error) {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, trie.ErrNotExist
	}

	return l, nil
}

func (l *leafNode) proto(_ client, _ bool) (proto.Message, error) {
	return &triepb.NodePb{
		Node: &triepb.NodePb_Leaf{
			Leaf: &triepb.LeafPb{
				Path:  l.key[:],
				Value: l.value,
			},
		},
	}, nil
}

func (l *leafNode) Flush(cli client) error {
	return l.store(cli)
}
