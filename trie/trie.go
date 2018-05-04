// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	cp "github.com/iotexproject/iotex-core-internal/crypto"
	"github.com/iotexproject/iotex-core-internal/db"
)

type (
	leaf struct {
		blob []byte // content of the node
	}
)

type (
	// Trie is the interface of Merkle Patricia Trie
	Trie interface {
		Insert(key, value []byte) error // insert a new entry
		Get(key []byte) ([]byte, error) // retrieve an existing entry
		Update(key, value []byte) error // update an existing entry
		Delete(key []byte) error        // delete an entry

		// Hash returns the root hash of the trie. It does not write to the
		// database and can be used even if the trie doesn't have one
		RootHash() cp.Hash32B
	}

	// trie implements the Trie interface
	trie struct {
		storeDb db.KVStore
	}
)

// NewTrie creates a trie with
func NewTrie() (*trie, error) {
	t := trie{db.NewBoltDB("trie.db", nil)}
	return &t, nil
}

// Insert a new entry
func (t *trie) Insert(key, value []byte) error {
	return nil
}

// Get an existing entry
func (t *trie) Get(key []byte) ([]byte, error) {
	return []byte{}, nil
}

// Update an existing entry
func (t *trie) Update(key, value []byte) error {
	return nil
}

// Delete an entry
func (t *trie) Delete(key []byte) error {
	return nil
}

// private functions
