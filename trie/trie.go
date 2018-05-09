// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"container/list"

	"github.com/golang/glog"
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
		dao      db.KVStore
		root     patricia
		toRoot   *list.List // stores the path from root to diverging node
		addNode  *list.List // stored newly added nodes on insert() operation
		divIndex byte       // value of new path when diverging at branch node
	}
)

// NewTrie creates a trie with
func NewTrie(dao db.KVStore) (Trie, error) {
	t := trie{dao, &branch{}, list.New(), list.New(), 0}
	return &t, nil
}

// Insert a new entry
func (t *trie) Insert(key, value []byte) error {
	div, size, err := t.query(key)
	if err == nil {
		return errors.Wrapf(ErrAlreadyExist, "key = %x", key)
	}
	// save the index if diverge at branch
	_, isBranch := div.(*branch)
	if isBranch {
		t.divIndex = key[0]
	}
	// insert at the diverging patricia node
	if err := div.insert(key[size:], value, t.addNode); err != nil {
		return err
	}
	// delete the original diverging node
	hashCurr := div.hash()
	if err := t.dao.Delete("", hashCurr[:]); err != nil {
		return err
	}
	glog.Warningf("delete key = %x", hashCurr[:8])
	hashChild := t.addNode.Front().Value.(patricia).hash()
	if isBranch {
		// update the new path into branch node
		div.ascend(hashChild[:], t.divIndex)
		hashChild = div.hash()
		value, err := div.serialize()
		if err != nil {
			return err
		}
		if err := t.dao.PutIfNotExists("", hashChild[:], value); err != nil {
			return err
		}
		glog.Warningf("put branch = %x", hashChild[:8])
	}
	// update newly added patricia node into db
	for t.addNode.Len() > 0 {
		n := t.addNode.Back()
		ptr, _ := n.Value.(patricia)
		value, err := ptr.serialize()
		if err != nil {
			return err
		}
		hashn := ptr.hash()
		if err := t.dao.PutIfNotExists("", hashn[:], value); err != nil {
			return err
		}
		glog.Warningf("put key = %x", hashn[:8])
		t.addNode.Remove(n)
	}
	// update nodes on path ascending to root
	return t.update(hashChild[:])
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
	size = len(key)
	return t.getValue(ptr, key[size-1])
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
// query returns the diverging patricia node, and length of matching path in bytes
func (t *trie) query(key []byte) (patricia, int, error) {
	ptr := t.root
	size := 0
	for len(key) > 0 {
		// keep descending the trie
		hashn, match, err := ptr.descend(key)
		// path diverges, return the diverging node
		if err != nil {
			// patricia.insert() will be called later to insert <key, value> pair into trie
			return ptr, size, err
		}
		// path matching entire key, return ptr that holds the value
		if match == len(key) {
			// clear the stack before return
			t.clear()
			return ptr, size + match, nil
		}
		if _, b := ptr.(*branch); b {
			// for branch node, need to save first byte of path so branch[key[0]] can be updated later
			t.toRoot.PushBack(key[0])
		}
		t.toRoot.PushBack(ptr)
		glog.Warningf("access key = %x", hashn[:8])
		if ptr, err = t.getPatricia(hashn); err != nil {
			return nil, 0, err
		}
		size += match
		key = key[match:]
	}
	return nil, size, nil
}

func (t *trie) update(hashChild []byte) error {
	for t.toRoot.Len() > 0 {
		n := t.toRoot.Back()
		ptr, _ := n.Value.(patricia)
		t.toRoot.Remove(n)
		var index byte
		_, isBranch := ptr.(*branch)
		if isBranch {
			// for branch node, the index is pushed onto stack in query()
			n := t.toRoot.Back()
			index, _ = n.Value.(byte)
			t.toRoot.Remove(n)
			glog.Warningf("curr is branch")
		}
		hashCurr := ptr.hash()
		glog.Warningf("curr key = %x", hashCurr[:8])
		glog.Warningf("child key = %x", hashChild[:8])
		if err := ptr.ascend(hashChild[:], index); err != nil {
			return err
		}
		// delete the current patricia node
		if err := t.dao.Delete("", hashCurr[:]); err != nil {
			return err
		}
		glog.Warningf("delete key = %x", hashCurr[:8])
		// store the new patricia node (with updated child hash)
		hashCurr = ptr.hash()
		value, err := ptr.serialize()
		if err != nil {
			return err
		}
		if err := t.dao.PutIfNotExists("", hashCurr[:], value); err != nil {
			return err
		}
		glog.Warningf("put key = %x", hashCurr[:8])
		hashChild = hashCurr[:]
	}
	return nil
}

// getPatricia retrieves the patricia node from db according to key
func (t *trie) getPatricia(key []byte) (patricia, error) {
	node, err := t.dao.Get("", key)
	if err != nil {
		return nil, errors.Wrapf(ErrNotExist, "key %x", key)
	}
	var ptr patricia
	// first byte of serialized data is type
	switch node[0] {
	case 0:
		ptr = &branch{}
	case 1:
		ptr = &ext{}
	case 2:
		ptr = &leaf{}
	default:
		return nil, errors.Wrapf(ErrInvalidPatricia, "invalid type = %v", node[0])
	}
	if err := ptr.deserialize(node); err != nil {
		return nil, err
	}
	return ptr, nil
}

// getValue returns the actual value stored in patricia node
func (t *trie) getValue(ptr patricia, index byte) ([]byte, error) {
	br, isBranch := ptr.(*branch)
	var err error
	if isBranch {
		if ptr, err = t.getPatricia(br.Path[index]); err != nil {
			return nil, err
		}
	}
	return ptr.blob()
}

// clear the stack
func (t *trie) clear() {
	for t.toRoot.Len() > 0 {
		n := t.toRoot.Back()
		t.toRoot.Remove(n)
	}
}
