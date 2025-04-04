// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package state

import (
	"github.com/pkg/errors"
)

// ErrOutOfBoundary defines an error when the index in the iterator is out of boundary
var ErrOutOfBoundary = errors.New("index is out of boundary")

// ErrNilValue is an error when value is nil
var ErrNilValue = errors.New("value is nil")

// ErrInConsistentLength is an error when keys and states have inconsistent length
var ErrInConsistentLength = errors.New("keys and states have inconsistent length")

// Iterator defines an iterator to read a set of states
type Iterator interface {
	// Size returns the size of the iterator
	Size() int
	// Next deserializes the next state in the iterator
	Next(interface{}) ([]byte, error)
}

type iterator struct {
	keys   [][]byte
	states [][]byte
	index  int
}

// NewIterator returns an iterator given a list of serialized states
func NewIterator(keys [][]byte, states [][]byte) (Iterator, error) {
	if len(keys) != len(states) {
		return nil, ErrInConsistentLength
	}
	return &iterator{index: 0, keys: keys, states: states}, nil
}

func (it *iterator) Size() int {
	return len(it.states)
}

func (it *iterator) Next(s interface{}) ([]byte, error) {
	i := it.index
	if i >= len(it.states) {
		return nil, ErrOutOfBoundary
	}
	it.index = i + 1
	if it.states[i] == nil {
		return nil, ErrNilValue
	}
	return it.keys[i], Deserialize(s, it.states[i])
}
