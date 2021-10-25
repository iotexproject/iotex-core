// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import "fmt"

type hashNode struct {
	node
	mpt     *merklePatriciaTrie
	hashVal []byte
}

func newHashNode(mpt *merklePatriciaTrie, ha []byte) *hashNode {
	return &hashNode{mpt: mpt, hashVal: ha}
}

func (h *hashNode) Flush() error {
	return nil
}

func (h *hashNode) Delete(key keyType, offset uint8) (node, error) {
	n, err := h.loadNode()
	if err != nil {
		return nil, err
	}

	return n.Delete(key, offset)
}

func (h *hashNode) Upsert(key keyType, offset uint8, value []byte) (node, error) {
	n, err := h.loadNode()
	if err != nil {
		return nil, err
	}

	return n.Upsert(key, offset, value)
}

func (h *hashNode) Search(key keyType, offset uint8) (node, error) {
	node, err := h.loadNode()
	if err != nil {
		return nil, err
	}

	return node.Search(key, offset)
}

func (h *hashNode) LoadNode() (node, error) {
	return h.loadNode()
}

func (h *hashNode) Print(indent string) {
	fmt.Printf("%s%x(hash node)\n", h.hashVal)
	n, err := h.loadNode()
	if err != nil {
		panic("failed to load node")
	}
	n.Print(indent)
}

func (h *hashNode) loadNode() (node, error) {
	return h.mpt.loadNode(h.hashVal)
}

func (h *hashNode) Hash() ([]byte, error) {
	return h.hashVal, nil
}
