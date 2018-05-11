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
	"github.com/iotexproject/iotex-core/logger"
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
		dao       db.KVStore
		root      patricia
		toRoot    *list.List // stores the path from root to diverging node
		addNode   *list.List // stored newly added nodes on insert() operation
		numBranch uint64
		numExt    uint64
		numLeaf   uint64
	}
)

// NewTrie creates a trie with
func NewTrie(dao db.KVStore) (Trie, error) {
	t := trie{dao, &branch{}, list.New(), list.New(), 1, 0, 0}
	return &t, nil
}

// Insert a new entry
func (t *trie) Insert(key, value []byte) error {
	div, size, err := t.query(key)
	if err == nil {
		return errors.Wrapf(ErrAlreadyExist, "key = %x", key)
	}
	// insert at the diverging patricia node
	nb, ne, nl := div.increase(key[size:])
	if err := div.insert(key[size:], value, t.addNode); err != nil {
		return err
	}
	// update newly added patricia node into db
	var hashChild common.Hash32B
	for t.addNode.Len() > 0 {
		n := t.addNode.Back()
		ptr, _ := n.Value.(patricia)
		value, err := ptr.serialize()
		if err != nil {
			return err
		}
		hashChild = ptr.hash()
		if err := t.dao.PutIfNotExists("", hashChild[:], value); err != nil {
			return err
		}
		logger.Info().Hex("key", hashChild[:8]).Msg("put")
		t.addNode.Remove(n)
	}
	t.numBranch += uint64(nb)
	t.numExt += uint64(ne)
	t.numLeaf += uint64(nl)
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
	child, size, err := t.query(key)
	if size != len(key) {
		return errors.Wrapf(ErrNotExist, "key = %x", key)
	}
	if err != nil {
		return err
	}
	// this node already on stack, pop it and discard
	_, _ = t.popToRoot()
	// for branch node, entry is stored in one of its leaf nodes
	index := key[len(key)-1]
	br, isBranch := child.(*branch)
	if isBranch {
		// delete the leaf corresponding to matching path
		l, err := t.getPatricia(br.Path[index])
		if err != nil {
			return err
		}
		hashCurr := l.hash()
		if err := t.dao.Delete("", hashCurr[:]); err != nil {
			return err
		}
		logger.Info().Hex("leaf", hashCurr[:8]).Msg("del")
	}
	// update nodes on path ascending to root
	path, _, childClps := child.collapse(index, true)
	for t.toRoot.Len() > 0 {
		curr, index := t.popToRoot()
		hash := curr.hash()
		if err := t.dao.Delete("", hash[:]); err != nil {
			return err
		}
		logger.Info().Hex("key", hash[:8]).Msg("del")
		k, _, currClps := curr.collapse(index, childClps)
		if currClps {
			// current node can also collapse, concatenate the path and keep going
			path = append(k, path...)
		} else {
			if childClps {
				// child is the last node that can collapse
				// TODO: update child's <k, v>
				hash := child.hash()
				// store new child to db
				value, err := child.serialize()
				if err != nil {
					return err
				}
				if err := t.dao.PutIfNotExists("", hash[:], value); err != nil {
					return err
				}
				logger.Info().Hex("key", hash[:8]).Msg("clp")
			}
			// update current with new child
			curr.ascend(hash[:], index)
			hash := child.hash()
			value, err := child.serialize()
			if err != nil {
				return err
			}
			if err := t.dao.PutIfNotExists("", hash[:], value); err != nil {
				return err
			}
			logger.Info().Hex("key", hash[:8]).Msg("put")
		}
		childClps = currClps
		child = curr
	}
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
		if _, b := ptr.(*branch); b {
			// for branch node, need to save first byte of path so branch[key[0]] can be updated later
			t.toRoot.PushBack(key[0])
		}
		t.toRoot.PushBack(ptr)
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
		logger.Info().Hex("key", hashn[:8]).Msg("access")
		if ptr, err = t.getPatricia(hashn); err != nil {
			return nil, 0, err
		}
		size += match
		key = key[match:]
	}
	return nil, size, nil
}

// update rewinds the path back to root and updates nodes along the way
func (t *trie) update(hashChild []byte) error {
	for t.toRoot.Len() > 0 {
		ptr, index := t.popToRoot()
		if ptr == nil {
			return errors.Wrap(ErrInvalidPatricia, "patricia pushed on stack is not valid")
		}
		// delete the current patricia node
		hashCurr := ptr.hash()
		logger.Info().Hex("curr key", hashCurr[:8]).Msg("10-4")
		logger.Info().Hex("child key", hashChild[:8]).Msg("10-4")
		if err := t.dao.Delete("", hashCurr[:]); err != nil {
			return err
		}
		logger.Info().Hex("key", hashCurr[:8]).Msg("del")
		// update the patricia node
		if ptr.ascend(hashChild[:], index) {
			hashCurr = ptr.hash()
			hashChild = hashCurr[:]
		}
		// store back to db (with updated hash)
		value, err := ptr.serialize()
		if err != nil {
			return err
		}
		if err := t.dao.PutIfNotExists("", hashChild, value); err != nil {
			return err
		}
		logger.Info().Hex("key", hashChild[:8]).Msg("put")
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
	case 2:
		ptr = &branch{}
	case 1:
		ptr = &leaf{}
	case 0:
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

// pop the stack
func (t *trie) popToRoot() (patricia, byte) {
	if t.toRoot.Len() > 0 {
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
		}
		return ptr, index
	}
	return nil, 0
}
