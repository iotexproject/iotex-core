// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"container/list"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/db"
)

var (
	// ErrKeyNotExist: key does not exist in trie
	ErrNotExist = errors.New("not exist in trie")

	// ErrAlreadyExist: key already exists in trie
	ErrAlreadyExist = errors.New("already exist in trie")
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
		dao    db.KVStore
		root   patricia
		toRoot *list.List
	}
)

// NewTrie creates a trie with
func NewTrie(dao db.KVStore) (Trie, error) {
	t := trie{dao, &branch{}, list.New()}
	return &t, nil
}

// Insert a new entry
func (t *trie) Insert(key, value []byte) error {
	ptr, size, err := t.query(key)
	if err == nil {
		return errors.Wrapf(ErrAlreadyExist, "key = %x", key)
	}
	// insert at the diverging patricia node
	stack := list.New()
	if err := ptr.insert(key[size:], value, stack); err != nil {
		return err
	}
	// update newly added patricia node
	for stack.Len() > 0 {
		n := stack.Back()
		ptr, _ := n.Value.(patricia)
		value, err := ptr.serialize()
		if err != nil {
			return err
		}
		hashn := ptr.hash()
		t.dao.Put("", hashn[:], value)
		stack.Remove(n)
	}
	// update nodes on path to root
	for t.toRoot.Len() > 0 {
		n := t.toRoot.Back()
		t.toRoot.Remove(n)
	}
	return nil
}

// Get an existing entry
func (t *trie) Get(key []byte) ([]byte, error) {
	ptr, size, err := t.query(key)
	if size != len(key) {
		return nil, errors.Wrapf(ErrNotExist, "key = %x", key)
	}
	if err != nil {
		return nil, err
	}
	// retrieve the value from terminal patricia node
	return ptr.blob()
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

//======================================
// private functions
//======================================
// query keeps walking down the trie until path diverges
// it returns the diverging patricia node, and length of matching path in bytes
func (t *trie) query(key []byte) (patricia, int, error) {
	ptr := t.root
	size := 0
	for len(key) > 0 {
		t.toRoot.PushBack(ptr)
		hashn, match, err := ptr.descend(key)
		if err != nil {
			return ptr, size, err
		}
		if match == len(key) {
			return ptr, size + match, err
		}
		node, err := t.dao.Get("", hashn)
		// first byte of serialized data is type
		switch node[0] {
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
		key = key[size:]
	}
	return nil, size, nil
}
