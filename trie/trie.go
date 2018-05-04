// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/db"
)

type (
	// Trie is the interface of Merkle Patricia Trie
	Trie interface {
		Insert(key, value []byte) error // insert a new entry
		Get(key []byte) ([]byte, error) // retrieve an existing entry
		Update(key, value []byte) error // update an existing entry
		Delete(key []byte) error        // delete an entry
		RootHash() common.Hash32B       // returns trie's root hash
	}

	// trie implements the Trie interface
	trie struct {
		storeDb db.KVStore
		root    patricia
	}
)

// NewTrie creates a trie with
func NewTrie() (*trie, error) {
	t := trie{db.NewBoltDB("trie.db", nil), nil}
	return &t, nil
}

// Insert a new entry
func (t *trie) Insert(key, value []byte) error {
	return nil
}

// Get an existing entry
func (t *trie) Get(key []byte) ([]byte, error) {
	ptr := t.root
	keyLen := len(key)
	err := error(nil)
	// traverse the patricia trie
	for keyLen > 0 {
		ptr, keyLen, err = ptr.descend(key, keyLen)
		if ptr == nil || err != nil {
			return nil, err
		}
	}
	// retrieve the value from terminal patricia node
	return t.getValue(ptr)
}

// Update an existing entry
func (t *trie) Update(key, value []byte) error {
	return nil
}

// Delete an entry
func (t *trie) Delete(key []byte) error {
	return nil
}

// getValue returns the value stored in patricia node
func (t *trie) getValue(p patricia) ([]byte, error) {
	return p.blob()
}
