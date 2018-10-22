// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"container/list"
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var (
	// AccountKVNameSpace is the bucket name for account trie
	AccountKVNameSpace = "Account"

	// CodeKVNameSpace is the bucket name for code
	CodeKVNameSpace = "Code"

	// ContractKVNameSpace is the bucket name for contract data storage
	ContractKVNameSpace = "Contract"

	// CandidateKVNameSpace is the bucket name for candidate data storage
	CandidateKVNameSpace = "Candidate"

	// ErrInvalidTrie indicates something wrong causing invalid operation
	ErrInvalidTrie = errors.New("invalid trie operation")

	// ErrNotExist indicates entry does not exist
	ErrNotExist = errors.New("not exist in trie")

	// EmptyRoot is the root hash of an empty trie
	EmptyRoot = hash.Hash32B{0xe, 0x57, 0x51, 0xc0, 0x26, 0xe5, 0x43, 0xb2, 0xe8, 0xab, 0x2e, 0xb0, 0x60, 0x99,
		0xda, 0xa1, 0xd1, 0xe5, 0xdf, 0x47, 0x77, 0x8f, 0x77, 0x87, 0xfa, 0xab, 0x45, 0xcd, 0xf1, 0x2f, 0xe3, 0xa8}
)

type (
	// Trie is the interface of Merkle Patricia Trie
	Trie interface {
		lifecycle.StartStopper
		TrieDB() db.KVStore          // return the underlying DB instance
		Upsert([]byte, []byte) error // insert a new entry
		Get([]byte) ([]byte, error)  // retrieve an existing entry
		Delete([]byte) error         // delete an entry
		Commit() error               // commit the state changes in a batch
		RootHash() hash.Hash32B      // returns trie's root hash
		SetRoot(hash.Hash32B) error  // set a new root to trie
	}

	// trie implements the Trie interface
	trie struct {
		lifecycle lifecycle.Lifecycle
		mutex     sync.RWMutex
		root      patricia
		rootHash  hash.Hash32B
		toRoot    *list.List // stores the path from root to diverging node
		bucket    string     // bucket name to store the nodes
		numEntry  uint64     // number of entries added to the trie
		numBranch uint64
		numExt    uint64
		numLeaf   uint64
		cb        db.CachedBatch // cached batch for pending writes
		dao       db.KVStore     // the underlying storage DB
	}
)

// NewTrie creates a trie with DB filename
func NewTrie(kvStore db.KVStore, name string, root hash.Hash32B) (Trie, error) {
	if kvStore == nil {
		return nil, errors.New("try to create trie with empty KV store")
	}
	return newTrie(kvStore, name, root), nil
}

// NewTrieSharedBatch creates a trie with a shared batch
func NewTrieSharedBatch(kvStore db.KVStore, batch db.CachedBatch, name string, root hash.Hash32B) (Trie, error) {
	if kvStore == nil || batch == nil {
		return nil, errors.New("try to create trie with empty KV store")
	}
	return newTrieSharedBatch(kvStore, batch, name, root), nil
}

func (t *trie) Start(ctx context.Context) error {
	t.lifecycle.OnStart(ctx)
	return t.loadRoot()
}

func (t *trie) Stop(ctx context.Context) error { return t.lifecycle.OnStop(ctx) }

// TrieDB return the underlying DB instance
func (t *trie) TrieDB() db.KVStore {
	return t.dao
}

// Upsert a new entry
func (t *trie) Upsert(key, value []byte) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.upsert(key, value)
}

// Get an existing entry
func (t *trie) Get(key []byte) ([]byte, error) {
	// Use write lock because t.clear() will mutate toRoot
	t.mutex.Lock()
	defer t.mutex.Unlock()

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

// Delete an entry
func (t *trie) Delete(key []byte) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	ptr, size, err := t.query(key)
	if size != len(key) {
		return errors.Wrapf(ErrNotExist, "key = %x not exist", key)
	}
	if err != nil {
		return errors.Wrap(err, "failed to query")
	}
	if isBranch(ptr) {
		// for branch, the entry to delete is the leaf matching last byte of path
		size = len(key)
		index := key[size-1]
		if ptr, err = t.getPatricia(ptr.(*branch).Path[index]); err != nil {
			return errors.Wrap(err, "failed to getPatricia")
		}
	} else {
		ptr, _ = t.popToRoot()
	}
	// delete the entry
	if err := t.delPatricia(ptr); err != nil {
		return errors.Wrap(err, "failed to delete")
	}
	if t.numEntry == 1 {
		return errors.Wrapf(ErrInvalidTrie, "trie has more entries than ever added")
	}
	t.numEntry--
	// update upstream nodes on path ascending to root
	return t.updateDelete()
}

// Commit local cached <k, v> in a batch
func (t *trie) Commit() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.dao.Commit(t.cb)
}

// RootHash returns the root hash of merkle patricia trie
func (t *trie) RootHash() hash.Hash32B {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.rootHash
}

// SetRoot sets the root trie
func (t *trie) SetRoot(rootHash hash.Hash32B) (err error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var root patricia
	if root, err = t.getPatricia(rootHash[:]); err != nil {
		return errors.Wrapf(err, "failed to set root %x", rootHash[:])
	}
	t.root = root
	t.rootHash = rootHash
	return err
}

//======================================
// private functions
//======================================
// newTrie creates a trie
func newTrie(dao db.KVStore, name string, root hash.Hash32B) *trie {
	t := &trie{
		cb:        db.NewCachedBatch(),
		dao:       dao,
		rootHash:  root,
		toRoot:    list.New(),
		bucket:    name,
		numEntry:  1,
		numBranch: 1,
	}
	t.lifecycle.Add(dao)
	return t
}

// newTrieSharedBatch creates a trie with shared DB
func newTrieSharedBatch(dao db.KVStore, batch db.CachedBatch, name string, root hash.Hash32B) *trie {
	t := &trie{
		cb:        batch,
		dao:       dao,
		rootHash:  root,
		toRoot:    list.New(),
		bucket:    name,
		numEntry:  1,
		numBranch: 1}
	t.lifecycle.Add(dao)
	return t
}

// loadRoot loads the root patricia from DB
func (t *trie) loadRoot() error {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if t.rootHash != EmptyRoot {
		var err error
		t.root, err = t.getPatricia(t.rootHash[:])
		return err
	}
	// initial empty trie
	t.root = &branch{}
	return t.putPatricia(t.root)
}

// upsert a new entry
func (t *trie) upsert(key, value []byte) error {
	ptr, size, err := t.query(key)
	if ptr == nil {
		return errors.Wrapf(err, "failed to parse key %x", key)
	}
	if err != nil {
		nb, ne, nl := ptr.increase(key[size:])
		addNode := list.New()
		if err := ptr.insert(key, value, size, addNode); err != nil {
			return errors.Wrapf(err, "failed to insert key = %x", key)
		}
		// update newly added patricia node into DB
		for addNode.Len() > 0 {
			n := addNode.Back()
			ptr, _ = n.Value.(patricia)
			// hash of new node should NOT exist in DB
			if err := t.putPatricia(ptr); err != nil {
				return err
			}
			addNode.Remove(n)
		}
		t.numBranch += uint64(nb)
		t.numExt += uint64(ne)
		t.numLeaf += uint64(nl)
		t.numEntry++
		// if the diverging node is leaf, delete it
		n := t.toRoot.Back()
		if _, ok := n.Value.(patricia).(*leaf); ok {
			logger.Debug().Msg("delete leaf")
			t.toRoot.Remove(n)
		}
	} else {
		// key already exists, update with new value
		if size != len(key) {
			return errors.Wrapf(ErrNotExist, "key = %x not exist", key)
		}
		if err != nil {
			return err
		}
		var index byte
		if isBranch(ptr) {
			// for branch, the entry to delete is the leaf matching last byte of path
			size = len(key)
			index = key[size-1]
			if ptr, err = t.getPatricia(ptr.(*branch).Path[index]); err != nil {
				return err
			}
		} else {
			ptr, index = t.popToRoot()
		}
		// delete the entry
		if err = t.delPatricia(ptr); err != nil {
			return err
		}
		// update with new value
		err := ptr.set(value)
		if err != nil {
			return err
		}
		if err := t.putPatricia(ptr); err != nil {
			return err
		}
	}
	// update upstream nodes on path ascending to root
	return t.updateInsert(ptr)
}

// query returns the diverging patricia node, and length of matching path in bytes
func (t *trie) query(key []byte) (patricia, int, error) {
	ptr := t.root
	if ptr == nil {
		return nil, 0, errors.Wrap(ErrNotExist, "failed to load root")
	}
	size := 0
	for len(key) > 0 {
		// keep descending the trie
		hashn, match, err := ptr.descend(key)
		if isBranch(ptr) {
			// for branch node, need to save first byte of path to traceback to branch[key[0]] later
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
	return ptr, size, nil
}

// updateInsert rewinds the path back to root and updates nodes along the way
func (t *trie) updateInsert(curr patricia) error {
	for t.toRoot.Len() > 0 {
		next, index := t.popToRoot()
		if next == nil || isLeaf(next) {
			return errors.Wrap(ErrInvalidPatricia, "patricia pushed on stack is not valid")
		}
		// update the patricia node
		hash := curr.hash()
		if err := next.ascend(hash[:], index); err != nil {
			return err
		}
		// when adding an entry, hash of nodes along the path changes and is expected NOT to exist in DB
		if err := t.putPatricia(next); err != nil {
			return err
		}
		curr = next
	}
	// update root hash
	t.rootHash = t.root.hash()
	return nil
}

// updateDelete rewinds the path back to root and updates nodes along the way
func (t *trie) updateDelete() error {
	var curr patricia
	for t.toRoot.Len() > 0 {
		logger.Debug().Int("stack size", t.toRoot.Len()).Msg("clps")
		next, index := t.popToRoot()
		if next == nil || isLeaf(next) {
			return errors.Wrap(ErrInvalidPatricia, "patricia pushed on stack is not valid")
		}
		if err := t.delPatricia(next); err != nil {
			return errors.Wrap(err, "failed to delete patricia")
		}
		// update current with new child
		hash := []byte(nil)
		if curr != nil {
			h := curr.hash()
			hash = h[:]
		}
		path, hash, active, err := next.collapse(hash, index)
		if err != nil {
			return err
		}
		if active == 0 {
			// no active branch/extension, 'next' can be deleted
			curr = nil
			continue
		}
		if active == 1 && isBranch(next) {
			// only 1 active branch, the branch can be replaced by an ext or leaf
			child, err := t.getPatricia(hash)
			if err != nil {
				return errors.Wrap(err, "failed to collapse branch")
			}
			if isLeaf(child) {
				l := child.(*leaf)
				next = &leaf{l.Ext - 1, l.Path, l.Value}
			} else {
				next = &leaf{EXTLEAF, path, hash}
			}
		}
		// two ext can combine into one
		if t.toRoot.Len() > 0 {
			n := t.toRoot.Back()
			parent, _ := n.Value.(patricia)
			if isExt(next) && isExt(parent) {
				ep, _ := parent.(*leaf)
				next = &leaf{EXTLEAF, append(ep.Path, path...), hash}
				if err := t.delPatricia(parent); err != nil {
					return errors.Wrap(err, "failed to delete patricia")
				}
				t.toRoot.Remove(n)
			}
		}
		if err := t.putPatricia(next); err != nil {
			return errors.Wrap(err, "failed to put patricia")
		}
		curr = next
	}
	// update root hash
	t.rootHash = t.root.hash()
	return nil
}

//======================================
// helper functions to operate patricia
//======================================
// getPatricia retrieves the patricia node from DB according to key
func (t *trie) getPatricia(key []byte) (patricia, error) {
	// search in cache first
	node, err := t.cb.Get(t.bucket, key)
	if err != nil {
		node, err = t.dao.Get(t.bucket, key)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key %x", key[:8])
	}
	var ptr patricia
	// first byte of serialized data is type
	switch node[0] {
	case BRANCH:
		ptr = &branch{}
	case EXTLEAF:
		ptr = &leaf{}
	default:
		return nil, errors.Wrapf(ErrInvalidPatricia, "invalid node type = %v", node[0])
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
		return errors.Wrapf(err, "failed to encode patricia node")
	}
	key := ptr.hash()
	logger.Debug().Hex("key", key[:8]).Msg("put")
	t.cb.Put(t.bucket, key[:], value, "failed to put key = %x", key)
	return nil
}

// putPatriciaNew stores a new patricia node into DB
// it is expected the node does not exist yet, will return error if already exist
func (t *trie) putPatriciaNew(ptr patricia) error {
	value, err := ptr.serialize()
	if err != nil {
		return errors.Wrap(err, "failed to encode patricia node")
	}
	key := ptr.hash()
	logger.Debug().Hex("key", key[:8]).Msg("putnew")
	return t.cb.PutIfNotExists(t.bucket, key[:], value, "failed to put non-existing key = %x", key)
}

// delPatricia deletes the patricia node from DB
func (t *trie) delPatricia(ptr patricia) error {
	key := ptr.hash()
	logger.Debug().Hex("key", key[:8]).Msg("del")
	t.cb.Delete(t.bucket, key[:], "failed to delete key = %x", key)
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

func isBranch(ptr patricia) bool {
	_, ok := ptr.(*branch)
	return ok
}

func isExt(ptr patricia) bool {
	e, ok := ptr.(*leaf)
	if ok {
		return e.Ext == EXTLEAF
	}
	return false
}

func isLeaf(ptr patricia) bool {
	l, ok := ptr.(*leaf)
	if ok {
		return l.Ext > EXTLEAF
	}
	return false
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
		if isBranch(ptr) {
			// for branch node, the index is pushed onto stack in query()
			n := t.toRoot.Back()
			index, _ = n.Value.(byte)
			t.toRoot.Remove(n)
		}
		return ptr, index
	}
	return nil, 0
}
