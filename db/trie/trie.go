// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"

	"github.com/pkg/errors"
)

var (
	// ErrInvalidTrie indicates something wrong causing invalid operation
	ErrInvalidTrie = errors.New("invalid trie operation")

	// ErrNotExist indicates entry does not exist
	ErrNotExist = errors.New("not exist in trie")

	// ErrEndOfIterator defines an error which will be returned
	ErrEndOfIterator = errors.New("hit the end of the iterator, no more item")
)

type (
	// Iterator iterates a trie
	Iterator interface {
		Next() ([]byte, []byte, error)
	}

	// Trie is the interface of Merkle Patricia Trie
	Trie interface {
		// Start starts the trie and the corresponding dependencies
		Start(context.Context) error
		// Stop stops the trie
		Stop(context.Context) error
		// Upsert inserts a new entry
		Upsert([]byte, []byte) error
		// Get retrieves an existing entry
		Get([]byte) ([]byte, error)
		// Delete deletes an entry
		Delete([]byte) error
		// RootHash returns trie's root hash
		RootHash() ([]byte, error)
		// SetRootHash sets a new root to trie
		SetRootHash([]byte) error
		// IsEmpty returns true is this is an empty trie
		IsEmpty() bool
		// Clone clones a trie with a new kvstore
		Clone(KVStore) (Trie, error)
	}
	// TwoLayerTrie is a trie data structure with two layers
	TwoLayerTrie interface {
		// Start starts the layer one trie
		Start(context.Context) error
		// Stop stops the layer one trie
		Stop(context.Context) error
		// RootHash returns the layer one trie root
		RootHash() ([]byte, error)
		// SetRootHash sets root hash for layer one trie
		SetRootHash([]byte) error
		// Get returns the value in layer two
		Get([]byte, []byte) ([]byte, error)
		// Upsert upserts an item in layer two
		Upsert([]byte, []byte, []byte) error
		// Delete deletes an item in layer two
		Delete([]byte, []byte) error
	}
)
