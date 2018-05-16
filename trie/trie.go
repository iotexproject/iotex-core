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
		curr      patricia   // current patricia node when updating nodes on path ascending to root
		toRoot    *list.List // stores the path from root to diverging node
		addNode   *list.List // stored newly added nodes on insert() operation
		clpsType  byte       // collapse into which node: 1-extension, 0-leaf
		numBranch uint64
		numExt    uint64
		numLeaf   uint64
	}
)

// NewTrie creates a trie with
func NewTrie(dao db.KVStore) (Trie, error) {
	t := trie{dao: dao, root: &branch{}, toRoot: list.New(), addNode: list.New(), numBranch: 1}
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
	// update newly added patricia node into DB
	var hashChild common.Hash32B
	for t.addNode.Len() > 0 {
		n := t.addNode.Back()
		ptr, ok := n.Value.(patricia)
		if !ok {
			return errors.Wrapf(ErrInvalidPatricia, "node = %v", n.Value)
		}
		hashChild = ptr.hash()
		// hash of new node should NOT exist in DB
		if err := t.putPatriciaNew(ptr); err != nil {
			return err
		}
		t.addNode.Remove(n)
	}
	t.numBranch += uint64(nb)
	t.numExt += uint64(ne)
	t.numLeaf += uint64(nl)
	// update nodes on path ascending to root
	return t.updateInsert(hashChild[:])
}

// Get an existing entry
func (t *trie) Get(key []byte) ([]byte, error) {
	ptr, size, err := t.query(key)
	t.clear()
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
	_, size, err := t.query(key)
	if size != len(key) {
		return errors.Wrapf(ErrNotExist, "key = %x", key)
	}
	if err != nil {
		return err
	}
	var index byte
	var path, value []byte
	var childClps bool
	t.curr, index = t.popToRoot()
	t.clpsType = 0
	// parent of leaf must be either branch or extension
	switch t.curr.(type) {
	case *branch:
		// delete the leaf corresponding to matching path
		l, err := t.getPatricia(t.curr.(*branch).Path[index])
		if err != nil {
			return err
		}
		if err := t.delPatricia(l); err != nil {
			return err
		}
		hash := t.curr.hash()
		// check if the branch can collapse, and if yes get the leaf node value
		if path, value, childClps = t.curr.collapse(path, value, index, true); childClps {
			l, err := t.getPatricia(value)
			if err != nil {
				return err
			}
			// the original branch collapse to its single remaining leaf
			var k []byte
			if k, value, err = l.blob(); err != nil {
				return err
			}
			// remaining leaf path != nil means it is extension node
			if k != nil {
				t.clpsType = 1
			}
			path = append(path, k...)
			t.curr.(*branch).print()
		} else {
			// update the branch itself (after deleting the leaf)
			if err := t.dao.Delete("", hash[:]); err != nil {
				return err
			}
			if err := t.putPatricia(t.curr); err != nil {
				return err
			}
		}
	case *leaf:
		if t.curr.(*leaf).Ext == 1 {
			return errors.Wrap(ErrInvalidPatricia, "extension cannot be terminal node")
		}
		// delete the leaf node
		if err := t.delPatricia(t.curr); err != nil {
			return err
		}
		// deleting a leaf, node on the path must be extension
		path, value, childClps, t.clpsType = nil, nil, true, 1
	}
	// update nodes on path ascending to root
	return t.updateDelete(path, value, childClps)
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
		logger.Debug().Hex("key", hashn).Msg("access")
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
			return ptr, size + match, nil
		}
		if ptr, err = t.getPatricia(hashn); err != nil {
			return nil, 0, err
		}
		size += match
		key = key[match:]
	}
	return nil, size, nil
}

// updateInsert rewinds the path back to root and updates nodes along the way
func (t *trie) updateInsert(hashChild []byte) error {
	// if the diverging node is leaf, it will be replaced and no need to update
	n := t.toRoot.Back()
	if _, ok := n.Value.(patricia).(*leaf); ok {
		logger.Warn().Msg("discard leaf")
		t.toRoot.Remove(n)
	}
	for t.toRoot.Len() > 0 {
		var index byte
		t.curr, index = t.popToRoot()
		if t.curr == nil {
			return errors.Wrap(ErrInvalidPatricia, "patricia pushed on stack is not valid")
		}
		// delete the current node
		hashCurr := t.curr.hash()
		logger.Info().Hex("curr key", hashCurr[:8]).Msg("10-4")
		logger.Info().Hex("child key", hashChild[:8]).Msg("10-4")
		if err := t.delPatricia(t.curr); err != nil {
			return err
		}
		// update the patricia node
		if err := t.curr.ascend(hashChild[:], index); err != nil {
			return err
		}
		hashCurr = t.curr.hash()
		hashChild = hashCurr[:]
		// when adding an entry, hash of nodes along the path changes and is expected NOT to exist in DB
		if err := t.putPatriciaNew(t.curr); err != nil {
			return err
		}
	}
	return nil
}

// updateDelete rewinds the path back to root and updates nodes along the way
func (t *trie) updateDelete(path []byte, value []byte, currClps bool) error {
	contClps := false
	for t.toRoot.Len() > 0 {
		logger.Info().Int("stack size", t.toRoot.Len()).Msg("clps")
		next, index := t.popToRoot()
		if next == nil {
			return errors.Wrap(ErrInvalidPatricia, "patricia pushed on stack is not valid")
		}
		if err := t.delPatricia(next); err != nil {
			return err
		}
		// we attempt to collapse in 2 cases:
		// 1. the current node is not root
		// 2. the current node is root, but <k, v> is nil meaning all entries in the trie are deleted
		isRoot := t.toRoot.Len() == 0
		noEntry := path == nil && value == nil
		var nextClps bool
		path, value, nextClps = next.collapse(path, value, index, currClps && (!isRoot || noEntry))
		logger.Info().Bool("curr", currClps).Msg("clps")
		logger.Info().Bool("next", nextClps).Msg("clps")
		if nextClps {
			// current node can also collapse, concatenate the path and keep going
			contClps = true
			if !isRoot {
				currClps = nextClps
				t.curr = next
				continue
			}
		}
		logger.Info().Bool("cont", contClps).Msg("clps")
		if contClps {
			// if deleting a node collapse all the way back including root, and <k, v> is nil
			// this means no more entry exist and the trie fallback to an empty trie
			if isRoot && noEntry {
				t.root = nil
				t.root = &branch{}
				logger.Warn().Msg("all entries deleted, trie fallback to empty")
				return nil
			}
			// otherwise collapse into a leaf node
			t.curr = &leaf{t.clpsType, path, value}
			logger.Info().Hex("k", path).Hex("v", value).Msg("clps")
			// after collapsing, the trie might rollback to an earlier state in the history (before adding the deleted entry)
			// so 'child' may already exist in DB
			if err := t.putPatricia(t.curr); err != nil {
				return err
			}
		}
		contClps = false
		// update current with new child
		hash := t.curr.hash()
		next.ascend(hash[:], index)
		// for the same reason above, the trie might rollback to an earlier state in the history
		// so 'curr' may already exist in DB
		if err := t.putPatricia(next); err != nil {
			return err
		}
		currClps = nextClps
		t.curr = next
	}
	return nil
}

// getPatricia retrieves the patricia node from DB according to key
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

// putPatricia stores the patricia node into DB
// the node may already exist in DB
func (t *trie) putPatricia(ptr patricia) error {
	value, err := ptr.serialize()
	if err != nil {
		return err
	}
	key := ptr.hash()
	if err := t.dao.Put("", key[:], value); err != nil {
		return errors.Wrapf(err, "key = %x", key[:8])
	}
	logger.Debug().Hex("key", key[:8]).Msg("put")
	return nil
}

// putPatriciaNew stores a new patricia node into DB
// it is expected the node does not exist yet, will return error if already exist
func (t *trie) putPatriciaNew(ptr patricia) error {
	value, err := ptr.serialize()
	if err != nil {
		return err
	}
	key := ptr.hash()
	if err := t.dao.PutIfNotExists("", key[:], value); err != nil {
		return errors.Wrapf(err, "key = %x", key[:8])
	}
	logger.Debug().Hex("key", key[:8]).Msg("putnew")
	return nil
}

// delPatricia deletes the patricia node from DB
func (t *trie) delPatricia(ptr patricia) error {
	key := ptr.hash()
	if err := t.dao.Delete("", key[:]); err != nil {
		return err
	}
	logger.Debug().Hex("key", key[:8]).Msg("del")
	return nil
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
	_, v, e := ptr.blob()
	return v, e
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
