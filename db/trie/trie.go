// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	trieMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_trie",
			Help: "IoTeX Trie",
		},
		[]string{"node", "type"},
	)
)

func init() {
	prometheus.MustRegister(trieMtc)
}

var (
	// ErrInvalidTrie indicates something wrong causing invalid operation
	ErrInvalidTrie = errors.New("invalid trie operation")

	// ErrNotExist indicates entry does not exist
	ErrNotExist = errors.New("not exist in trie")
)

// DefaultHashFunc implements a default hash function
func DefaultHashFunc(data []byte) []byte {
	h := hash.Hash256b(data)
	return h[:]
}

// Trie is the interface of Merkle Patricia Trie
type Trie interface {
	// Start starts the trie and the corresponding dependencies
	Start(context.Context) error
	// Stop stops the trie
	Stop(context.Context) error
	// Upsert inserts a new entry
	Upsert([]byte, []byte) error
	// Get retrieves an existing entry
	Get([]byte) ([]byte, error)
	// Delete deletes an entry
	Delete([]byte) error
	// RootHash returns trie's root hash
	RootHash() []byte
	// SetRootHash sets a new root to trie
	SetRootHash([]byte) error
	// DB returns the KVStore storing the node data
	DB() KVStore
	// deleteNodeFromDB deletes the data of node from db
	deleteNodeFromDB(tn Node) error
	// putNodeIntoDB puts the data of a node into db
	putNodeIntoDB(tn Node) error
	// loadNodeFromDB loads a node from db
	loadNodeFromDB([]byte) (Node, error)
	// isEmptyRootHash returns whether this is an empty root hash
	isEmptyRootHash([]byte) bool
	// emptyRootHash returns hash of an empty root
	emptyRootHash() []byte
	// nodeHash returns the hash of a node
	nodeHash(tn Node) []byte
}

// Option sets parameters for SameKeyLenTrieContext construction parameter
type Option func(Trie) error

// KeyLengthOption sets the length of the keys saved in trie
func KeyLengthOption(len int) Option {
	return func(tr Trie) error {
		if len <= 0 || len > 128 {
			return errors.New("invalid key length")
		}
		switch t := tr.(type) {
		case *branchRootTrie:
			t.keyLength = len
		default:
			return errors.New("invalid trie type")
		}
		return nil
	}
}

// RootHashOption sets the root hash for the trie
func RootHashOption(h []byte) Option {
	return func(tr Trie) error {
		switch t := tr.(type) {
		case *branchRootTrie:
			t.rootHash = make([]byte, len(h))
			copy(t.rootHash, h)
		default:
			return errors.New("invalid trie type")
		}
		return nil
	}
}

// RootKeyOption sets the root key for the trie
func RootKeyOption(key string) Option {
	return func(tr Trie) error {
		switch t := tr.(type) {
		case *branchRootTrie:
			t.rootKey = key
		default:
			return errors.New("invalid trie type")
		}
		return nil
	}
}

// HashFuncOption sets the hash func for the trie
func HashFuncOption(hashFunc HashFunc) Option {
	return func(tr Trie) error {
		switch t := tr.(type) {
		case *branchRootTrie:
			t.hashFunc = hashFunc
		default:
			return errors.New("invalid trie type")
		}
		return nil
	}
}

// KVStoreOption sets the kvstore for the trie
func KVStoreOption(kvStore KVStore) Option {
	return func(tr Trie) error {
		switch t := tr.(type) {
		case *branchRootTrie:
			t.kvStore = kvStore
		default:
			return errors.New("invalid trie type")
		}
		return nil
	}
}

// NewTrie creates a trie with DB filename
func NewTrie(options ...Option) (Trie, error) {
	t := &branchRootTrie{
		keyLength: 20,
		hashFunc:  DefaultHashFunc,
	}
	for _, opt := range options {
		if err := opt(t); err != nil {
			return nil, err
		}
	}
	if t.rootHash == nil {
		t.rootHash = t.emptyRootHash()
	}
	if t.kvStore == nil {
		t.kvStore = newInMemKVStore()
	}

	return t, nil
}
