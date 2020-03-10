// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package merklepatriciatree

import "github.com/golang/protobuf/proto"

type hashNode struct {
	cacheNode
}

func newHashNode(mpt *merklePatriciaTree, ha []byte) *hashNode {
	hn := &hashNode{cacheNode{mpt: mpt, ha: ha}}
	hn.cacheNode.node = hn

	return hn
}

func newHashNodeFromSer(mpt *merklePatriciaTree, ser []byte) *hashNode {
	hn := &hashNode{cacheNode{mpt: mpt, ser: ser, ha: mpt.hashFunc(ser)}}
	hn.cacheNode.node = hn

	return hn
}

func (h *hashNode) Delete(key keyType, offset uint8) (node, error) {
	node, err := h.mpt.loadNode(h.ha)
	if err != nil {
		return nil, err
	}

	return node.Delete(key, offset)
}

func (h *hashNode) Upsert(key keyType, offset uint8, value []byte) (node, error) {
	node, err := h.mpt.loadNode(h.ha)
	if err != nil {
		return nil, err
	}

	return node.Upsert(key, offset, value)
}

func (h *hashNode) Search(key keyType, offset uint8) (node, error) {
	node, err := h.mpt.loadNode(h.ha)
	if err != nil {
		return nil, err
	}

	return node.Search(key, offset)
}

func (h *hashNode) Proto() (proto.Message, error) {
	node, err := h.mpt.loadNode(h.ha)
	if err != nil {
		return nil, err
	}
	return node.Proto()
}

func (h *hashNode) Hash() ([]byte, error) {
	return h.ha, nil
}
