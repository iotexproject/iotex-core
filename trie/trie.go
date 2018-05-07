// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/db"
)

var (
	// ErrKeyNotExist: key does not exist in trie
	ErrNotExist = errors.New("not exist in trie")
)

var (
	// emptyRoot is the root hash of an empty trie
	emptyRoot = common.Hash32B{0xe, 0x57, 0x51, 0xc0, 0x26, 0xe5, 0x43, 0xb2, 0xe8, 0xab, 0x2e, 0xb0, 0x60, 0x99,
		0xda, 0xa1, 0xd1, 0xe5, 0xdf, 0x47, 0x77, 0x8f, 0x77, 0x87, 0xfa, 0xab, 0x45, 0xcd, 0xf1, 0x2f, 0xe3, 0xa8}
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
		dao  db.KVStore
		root patricia
	}
)

// NewTrie creates a trie with
func NewTrie() (Trie, error) {
	t := trie{db.NewBoltDB("trie.db", nil), &branch{}}
	return &t, nil
}

// Insert a new entry
func (t *trie) Insert(key, value []byte) error {
	return nil
}

// Get an existing entry
func (t *trie) Get(key []byte) ([]byte, error) {
	ptr, size, err := t.query(key)
	if size != len(key)<<1 {
		return nil, errors.Wrapf(ErrNotExist, "key = %x", key)
	}
	if err != nil {
		return nil, err
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

// RootHash returns the root hash of merkle patricia trie
func (t *trie) RootHash() common.Hash32B {
	return t.root.hash()
}

// ======================================
// private functions
// ======================================

// query keeps walking down the trie until path diverges
// it returns the diverging patricia node, and length of matching path in nibbles (nibble = 4-bit)
func (t *trie) query(key []byte) (patricia, int, error) {
	ptr := t.root
	size := 0
	for len(key) > 0 {
		stream, match, err := ptr.descend(key, size&1 != 0)
		if err != nil {
			break
		}
		node, err := t.dao.Get("", stream[1:])
		// first byte of serialized data is type
		switch stream[0] {
		case 0:
			ptr = &branch{}
		case 1:
			ptr = &ext{}
		case 2:
			ptr = &leaf{}
		}
		if err := ptr.deserialize(node); err != nil {
			return nil, 0, err
		}
		size += match
		key = key[size>>1:]
	}
	return ptr, size, nil
}

// getValue returns the value stored in patricia node
func (t *trie) getValue(p patricia) ([]byte, error) {
	return p.blob()
}
