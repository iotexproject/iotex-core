// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/trie"
)

// LeafIterator defines an iterator to go through all the leaves under given node
type LeafIterator struct {
	mpt   *merklePatriciaTrie
	stack []node
}

// NewLeafIterator returns a new leaf iterator
func NewLeafIterator(tr trie.Trie) (trie.Iterator, error) {
	mpt, ok := tr.(*merklePatriciaTrie)
	if !ok {
		return nil, errors.New("trie is not supported type")
	}
	stack := []node{mpt.root}

	return &LeafIterator{mpt: mpt, stack: stack}, nil
}

// Next moves iterator to next node
func (li *LeafIterator) Next() ([]byte, []byte, error) {
	for len(li.stack) > 0 {
		size := len(li.stack)
		node := li.stack[size-1]
		li.stack = li.stack[:size-1]
		if ln, ok := node.(leaf); ok {
			key := ln.Key()
			value := ln.Value()

			return append(key[:0:0], key...), append(value, value...), nil
		}
		if bn, ok := node.(branch); ok {
			children, err := bn.children()
			if err != nil {
				return nil, nil, err
			}
			li.stack = append(li.stack, children...)
			continue
		}
		if en, ok := node.(extension); ok {
			child, err := en.child()
			if err != nil {
				return nil, nil, err
			}
			li.stack = append(li.stack, child)
			continue
		}
		return nil, nil, errors.New("unexpected node type")
	}

	return nil, nil, trie.ErrEndOfIterator
}
