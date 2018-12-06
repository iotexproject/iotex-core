// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var (
	// ErrInvalidTrie indicates something wrong causing invalid operation
	ErrInvalidTrie = errors.New("invalid trie operation")

	// ErrNotExist indicates entry does not exist
	ErrNotExist = errors.New("not exist in trie")
)

// Trie is the interface of Merkle Patricia Trie
type Trie interface {
	lifecycle.StartStopper
	Root() Node
	ChildrenOf(Node) ([]Node, error)
	TrieDB() db.KVStore          // return the underlying DB instance
	Upsert([]byte, []byte) error // insert a new entry
	Get([]byte) ([]byte, error)  // retrieve an existing entry
	Delete([]byte) error         // delete an entry
	Commit() error               // commit the state changes in a batch
	RootHash() hash.Hash32B      // returns trie's root hash
	SetRoot(hash.Hash32B) error  // set a new root to trie
}

// Option sets parameters for SameKeyLenTrieContext construction parameter
type Option func(*SameKeyLenTrieContext) error

// CachedBatchOption defines an option to set the cached batch
func CachedBatchOption(batch db.CachedBatch) Option {
	return func(tr *SameKeyLenTrieContext) error {
		if batch == nil {
			return errors.Wrapf(ErrInvalidTrie, "batch option cannot be nil")
		}
		tr.CB = batch
		return nil
	}
}

// KeyLengthOption sets the length of the keys saved in trie
func KeyLengthOption(len int) Option {
	return func(tr *SameKeyLenTrieContext) error {
		if len <= 0 || len > 128 {
			return errors.New("Invalid key length")
		}
		tr.KeyLength = len
		return nil
	}
}

// RootHashOption sets the root hash for the trie
func RootHashOption(h hash.Hash32B) Option {
	return func(tr *SameKeyLenTrieContext) error {
		tr.InitRootHash = h
		return nil
	}
}

// NewTrie creates a trie with DB filename
func NewTrie(kvStore db.KVStore, name string, options ...Option) (Trie, error) {
	if kvStore == nil {
		return nil, errors.Wrapf(ErrInvalidTrie, "try to create trie with empty KV store")
	}
	tc := SameKeyLenTrieContext{
		DB:           kvStore,
		Bucket:       name,
		KeyLength:    20,
		InitRootHash: EmptyBranchNodeHash,
	}
	for _, opt := range options {
		if err := opt(&tc); err != nil {
			return nil, err
		}
	}
	if tc.CB == nil {
		tc.CB = db.NewCachedBatch()
	}
	t := newBranchRootTrie(tc)
	t.lc.Add(kvStore)

	return t, nil
}

// NewTrieWithKey creates a trie with DB and root key
func NewTrieWithKey(kvStore db.KVStore, name string, key string, options ...Option) (Trie, error) {
	if kvStore == nil {
		return nil, errors.Wrapf(ErrInvalidTrie, "trie to create trie with empty KV store")
	}
	tc := SameKeyLenTrieContext{
		DB:           kvStore,
		Bucket:       name,
		KeyLength:    20,
		InitRootHash: EmptyBranchNodeHash,
		RootKey:      key,
	}
	for _, opt := range options {
		if err := opt(&tc); err != nil {
			return nil, err
		}
	}
	if tc.CB == nil {
		tc.CB = db.NewCachedBatch()
	}
	t := newBranchRootTrie(tc)
	t.lc.Add(kvStore)
	return t, nil
}
