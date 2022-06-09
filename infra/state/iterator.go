// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"github.com/pkg/errors"
)

// ErrOutOfBoundary defines an error when the index in the iterator is out of boundary
var ErrOutOfBoundary = errors.New("index is out of boundary")

// ErrNilValue is an error when value is nil
var ErrNilValue = errors.New("value is nil")

// Iterator defines an interator to read a set of states
type Iterator interface {
	// Size returns the size of the iterator
	Size() int
	// Next deserializes the next state in the iterator
	Next(interface{}) error
}

type iterator struct {
	states [][]byte
	index  int
}

// NewIterator returns an interator given a list of serialized states
func NewIterator(states [][]byte) Iterator {
	return &iterator{index: 0, states: states}
}

func (it *iterator) Size() int {
	return len(it.states)
}

func (it *iterator) Next(s interface{}) error {
	i := it.index
	if i >= len(it.states) {
		return ErrOutOfBoundary
	}
	it.index = i + 1
	if it.states[i] == nil {
		return ErrNilValue
	}
	return Deserialize(s, it.states[i])
}
