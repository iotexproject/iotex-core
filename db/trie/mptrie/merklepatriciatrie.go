// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"bytes"
	"context"
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

type (
	// HashFunc defines a function to generate the hash which will be used as key in db
	HashFunc func([]byte) []byte

	merklePatriciaTrie struct {
		mutex         sync.RWMutex
		keyLength     int
		root          branch
		rootHash      []byte
		rootKey       string
		kvStore       trie.KVStore
		hashFunc      HashFunc
		async         bool
		emptyRootHash []byte
	}
)

// DefaultHashFunc implements a default hash function
func DefaultHashFunc(data []byte) []byte {
	h := hash.Hash160b(data)
	return h[:]
}

// Option sets parameters for SameKeyLenTrieContext construction parameter
type Option func(*merklePatriciaTrie) error

// KeyLengthOption sets the length of the keys saved in trie
func KeyLengthOption(len int) Option {
	return func(mpt *merklePatriciaTrie) error {
		if len <= 0 || len > 128 {
			return errors.New("invalid key length")
		}
		mpt.keyLength = len
		return nil
	}
}

// RootHashOption sets the root hash for the trie
func RootHashOption(h []byte) Option {
	return func(mpt *merklePatriciaTrie) error {
		mpt.rootHash = make([]byte, len(h))
		copy(mpt.rootHash, h)
		return nil
	}
}

// HashFuncOption sets the hash func for the trie
func HashFuncOption(hashFunc HashFunc) Option {
	return func(mpt *merklePatriciaTrie) error {
		mpt.hashFunc = hashFunc
		return nil
	}
}

// KVStoreOption sets the kvStore for the trie
func KVStoreOption(kvStore trie.KVStore) Option {
	return func(mpt *merklePatriciaTrie) error {
		mpt.kvStore = kvStore
		return nil
	}
}

// AsyncOption enables async commit
func AsyncOption() Option {
	return func(mpt *merklePatriciaTrie) error {
		mpt.async = true
		return nil
	}
}

// New creates a trie with DB filename
func New(options ...Option) (trie.Trie, error) {
	t := &merklePatriciaTrie{
		keyLength: 20,
		hashFunc:  DefaultHashFunc,
		kvStore:   trie.NewMemKVStore(),
	}
	for _, opt := range options {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

func (mpt *merklePatriciaTrie) Start(ctx context.Context) error {
	mpt.mutex.Lock()
	defer mpt.mutex.Unlock()

	emptyRoot, err := newRootBranchNode(mpt, nil, nil, false)
	if err != nil {
		return err
	}
	emptyRootHash, err := emptyRoot.Hash(mpt)
	if err != nil {
		return err
	}
	mpt.emptyRootHash = emptyRootHash
	if mpt.rootHash == nil {
		mpt.rootHash = mpt.emptyRootHash
	}

	return mpt.setRootHash(mpt.rootHash)
}

func (mpt *merklePatriciaTrie) Stop(_ context.Context) error {
	return nil
}

func (mpt *merklePatriciaTrie) RootHash() ([]byte, error) {
	if mpt.async {
		if err := mpt.root.Flush(mpt); err != nil {
			return nil, err
		}
		h, err := mpt.root.Hash(mpt)
		if err != nil {
			return nil, err
		}
		mpt.rootHash = h
	}

	return mpt.rootHash, nil
}

func (mpt *merklePatriciaTrie) SetRootHash(rootHash []byte) error {
	mpt.mutex.Lock()
	defer mpt.mutex.Unlock()

	return mpt.setRootHash(rootHash)
}

func (mpt *merklePatriciaTrie) IsEmpty() bool {
	mpt.mutex.RLock()
	defer mpt.mutex.RUnlock()
	if mpt.async {
		return mpt.root == nil || len(mpt.root.Children()) == 0
	}

	return mpt.isEmptyRootHash(mpt.rootHash)
}

func (mpt *merklePatriciaTrie) Get(key []byte) ([]byte, error) {
	mpt.mutex.RLock()
	defer mpt.mutex.RUnlock()

	kt, err := mpt.checkKeyType(key)
	if err != nil {
		return nil, err
	}
	t, err := mpt.root.Search(mpt, kt, 0)
	if err != nil {
		return nil, err
	}
	if l, ok := t.(leaf); ok {
		return l.Value(), nil
	}

	return nil, trie.ErrInvalidTrie
}

func (mpt *merklePatriciaTrie) Delete(key []byte) error {
	mpt.mutex.Lock()
	defer mpt.mutex.Unlock()

	kt, err := mpt.checkKeyType(key)
	if err != nil {
		return err
	}
	newRoot, err := mpt.root.Delete(mpt, kt, 0)
	if err != nil {
		return errors.Wrapf(trie.ErrNotExist, "key %x does not exist", kt)
	}
	var bn branch
	switch n := newRoot.(type) {
	case branch:
		bn = n
	case *hashNode:
		newRoot, err = n.LoadNode(mpt)
		if err != nil {
			return err
		}
		var ok bool
		bn, ok = newRoot.(branch)
		if !ok {
			panic("unexpected new root")
		}
	default:
		panic("unexpected new root")
	}

	return mpt.resetRoot(bn, nil)
}

func (mpt *merklePatriciaTrie) Upsert(key []byte, value []byte) error {
	mpt.mutex.Lock()
	defer mpt.mutex.Unlock()

	kt, err := mpt.checkKeyType(key)
	if err != nil {
		return err
	}
	newRoot, err := mpt.root.Upsert(mpt, kt, 0, value)
	if err != nil {
		return err
	}
	bn, ok := newRoot.(branch)
	if !ok {
		panic("unexpected new root")
	}

	return mpt.resetRoot(bn, nil)
}

func (mpt *merklePatriciaTrie) isEmptyRootHash(h []byte) bool {
	return bytes.Equal(h, mpt.emptyRootHash)
}

func (mpt *merklePatriciaTrie) setRootHash(rootHash []byte) error {
	if len(rootHash) == 0 || mpt.isEmptyRootHash(rootHash) {
		emptyRoot, err := newRootBranchNode(mpt, nil, nil, false)
		if err != nil {
			return err
		}
		mpt.resetRoot(emptyRoot, mpt.emptyRootHash)
		return nil
	}
	node, err := mpt.loadNode(rootHash)
	if err != nil {
		return err
	}
	root, ok := node.(branch)
	if !ok {
		return errors.Wrapf(trie.ErrInvalidTrie, "root should be a branch")
	}
	root.MarkAsRoot()

	return mpt.resetRoot(root, rootHash)
}

func (mpt *merklePatriciaTrie) resetRoot(newRoot branch, rootHash []byte) error {
	mpt.root = newRoot
	if mpt.async {
		return nil
	}
	if rootHash == nil {
		var err error
		rootHash, err = newRoot.Hash(mpt)
		if err != nil {
			return err
		}
	}
	mpt.rootHash = make([]byte, len(rootHash))
	copy(mpt.rootHash, rootHash)

	return nil
}

func (mpt *merklePatriciaTrie) asyncMode() bool {
	return mpt.async
}

func (mpt *merklePatriciaTrie) checkKeyType(key []byte) (keyType, error) {
	if len(key) != mpt.keyLength {
		return nil, errors.Errorf("invalid key length %d", len(key))
	}
	kt := make([]byte, mpt.keyLength)
	copy(kt, key)

	return kt, nil
}

func (mpt *merklePatriciaTrie) hash(key []byte) []byte {
	return mpt.hashFunc(key)
}

func (mpt *merklePatriciaTrie) deleteNode(key []byte) error {
	return mpt.kvStore.Delete(key)
}

func (mpt *merklePatriciaTrie) putNode(key []byte, value []byte) error {
	return mpt.kvStore.Put(key, value)
}

func (mpt *merklePatriciaTrie) loadNode(key []byte) (node, error) {
	s, err := mpt.kvStore.Get(key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key %x", key)
	}
	pb := triepb.NodePb{}
	if err := proto.Unmarshal(s, &pb); err != nil {
		return nil, err
	}
	if pbBranch := pb.GetBranch(); pbBranch != nil {
		return newBranchNodeFromProtoPb(mpt, pbBranch, key), nil
	}
	if pbLeaf := pb.GetLeaf(); pbLeaf != nil {
		return newLeafNodeFromProtoPb(mpt, pbLeaf, key), nil
	}
	if pbExtend := pb.GetExtend(); pbExtend != nil {
		return newExtensionNodeFromProtoPb(mpt, pbExtend, key), nil
	}

	return nil, errors.New("invalid node type")
}

func (mpt *merklePatriciaTrie) Clone(kvStore trie.KVStore) (trie.Trie, error) {
	mpt.mutex.RLock()
	defer mpt.mutex.RUnlock()
	root, err := mpt.root.Clone()
	if err != nil {
		return nil, err
	}
	rh := make([]byte, len(mpt.rootHash))
	copy(rh, mpt.rootHash)
	erh := make([]byte, len(mpt.emptyRootHash))
	copy(erh, mpt.emptyRootHash)

	return &merklePatriciaTrie{
		keyLength:     mpt.keyLength,
		root:          root,
		rootHash:      rh,
		rootKey:       mpt.rootKey,
		kvStore:       kvStore,
		hashFunc:      mpt.hashFunc,
		async:         mpt.async,
		emptyRootHash: erh,
	}, nil
}
