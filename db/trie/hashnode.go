// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

type hashNode struct {
	ha []byte
	sa []byte
}

func newHashNode(ha []byte) *hashNode {
	return &hashNode{ha: ha}
}

func newHashNodeFromSer(tr Trie, sa []byte) *hashNode {
	ha := tr.hash(sa)
	return &hashNode{ha: ha, sa: sa}
}

func (h *hashNode) Type() NodeType {
	return HASH
}

func (h *hashNode) Children() []Node {
	return nil
}

func (h *hashNode) delete(tr Trie, key keyType, offset uint8) (Node, error) {
	node, err := tr.LoadNodeFromDB(h.ha)
	if err != nil {
		return nil, err
	}

	return node.delete(tr, key, offset)
}

func (h *hashNode) upsert(tr Trie, key keyType, offset uint8, value []byte) (Node, error) {
	node, err := tr.LoadNodeFromDB(h.ha)
	if err != nil {
		return nil, err
	}

	return node.upsert(tr, key, offset, value)
}

func (h *hashNode) search(tr Trie, key keyType, offset uint8) (Node, error) {
	node, err := tr.LoadNodeFromDB(h.ha)
	if err != nil {
		return nil, err
	}

	return node.search(tr, key, offset)
}

func (h *hashNode) isDirty() bool {
	return false
}

func (h *hashNode) hash(tr Trie, flush bool) ([]byte, error) {
	if h == nil {
		return nil, nil
	}
	return h.ha, nil
}

func (h *hashNode) getHash() []byte {
	return h.ha
}
