// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

type hashNode struct {
	node
	hashVal []byte
}

func newHashNode(ha []byte) *hashNode {
	return &hashNode{hashVal: ha}
}

func (h *hashNode) Flush(_ client) error {
	return nil
}

func (h *hashNode) Delete(cli client, key keyType, offset uint8) (node, error) {
	n, err := h.loadNode(cli)
	if err != nil {
		return nil, err
	}

	return n.Delete(cli, key, offset)
}

func (h *hashNode) Upsert(cli client, key keyType, offset uint8, value []byte) (node, error) {
	n, err := h.loadNode(cli)
	if err != nil {
		return nil, err
	}

	return n.Upsert(cli, key, offset, value)
}

func (h *hashNode) Search(cli client, key keyType, offset uint8) (node, error) {
	node, err := h.loadNode(cli)
	if err != nil {
		return nil, err
	}

	return node.Search(cli, key, offset)
}

func (h *hashNode) LoadNode(cli client) (node, error) {
	return h.loadNode(cli)
}

func (h *hashNode) loadNode(cli client) (node, error) {
	return cli.loadNode(h.hashVal)
}

func (h *hashNode) Hash(_ client) ([]byte, error) {
	return h.hashVal, nil
}
