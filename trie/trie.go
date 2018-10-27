// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
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
		bucket    string // bucket name to store the nodes
		numEntry  uint64 // number of entries added to the trie
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
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.rootHash != EmptyRoot {
		var err error
		t.root, err = getPatricia(t.rootHash[:], t.dao, t.bucket, t.cb)
		return err
	}
	// initial empty trie
	t.root = &branch{}
	if _, err := putPatricia(t.root, t.bucket, t.cb); err != nil {
		return err
	}
	return t.dao.Commit(t.cb)
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

	_, err := t.root.upsert(key, value, 0, t.dao, t.bucket, t.cb)
	// update root hash
	t.rootHash = t.root.hash()
	return err
}

// Get an existing entry
func (t *trie) Get(key []byte) ([]byte, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.root.get(key, 0, t.dao, t.bucket, t.cb)
}

// Delete an entry
func (t *trie) Delete(key []byte) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	_, err := t.root.delete(key, 0, t.dao, t.bucket, t.cb)
	// update root hash
	t.rootHash = t.root.hash()
	return err
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
	t.mutex.Lock()
	defer t.mutex.Unlock()

	var root patricia
	if root, err = getPatricia(rootHash[:], t.dao, t.bucket, t.cb); err != nil {
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
		bucket:    name,
		numEntry:  1,
		numBranch: 1}
	t.lifecycle.Add(dao)
	return t
}
