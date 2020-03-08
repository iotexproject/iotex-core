// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package merklepatriciatree

import (
	"bytes"
	"context"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
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

// DefaultHashFunc implements a default hash function
func DefaultHashFunc(data []byte) []byte {
	h := hash.Hash160b(data)
	return h[:]
}

// Option sets parameters for SameKeyLenTrieContext construction parameter
type Option func(*merklePatriciaTree) error

// KeyLengthOption sets the length of the keys saved in trie
func KeyLengthOption(len int) Option {
	return func(mpt *merklePatriciaTree) error {
		if len <= 0 || len > 128 {
			return errors.New("invalid key length")
		}
		mpt.keyLength = len
		return nil
	}
}

// RootHashOption sets the root hash for the trie
func RootHashOption(h []byte) Option {
	return func(mpt *merklePatriciaTree) error {
		mpt.rootHash = make([]byte, len(h))
		copy(mpt.rootHash, h)
		return nil
	}
}

// RootKeyOption sets the root key for the trie
func RootKeyOption(key string) Option {
	return func(mpt *merklePatriciaTree) error {
		mpt.rootKey = key
		return nil
	}
}

// HashFuncOption sets the hash func for the trie
func HashFuncOption(hashFunc HashFunc) Option {
	return func(mpt *merklePatriciaTree) error {
		mpt.hashFunc = hashFunc
		return nil
	}
}

// KVStoreOption sets the kvStore for the trie
func KVStoreOption(kvStore trie.KVStore) Option {
	return func(mpt *merklePatriciaTree) error {
		mpt.kvStore = kvStore
		return nil
	}
}

// New creates a trie with DB filename
func New(options ...Option) (trie.Trie, error) {
	t := &merklePatriciaTree{
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
		t.kvStore = trie.NewMemKVStore()
	}

	return t, nil
}

type (
	// HashFunc defines a function to generate the hash which will be used as key in db
	HashFunc           func([]byte) []byte
	merklePatriciaTree struct {
		mutex     sync.RWMutex
		keyLength int
		kvStore   trie.KVStore
		hashFunc  HashFunc
		root      *branchNode
		rootHash  []byte
		rootKey   string
	}
)

func (tr *merklePatriciaTree) Start(ctx context.Context) error {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()
	if tr.rootKey != "" {
		switch root, err := tr.kvStore.Get([]byte(tr.rootKey)); errors.Cause(err) {
		case nil:
			tr.rootHash = root
		case trie.ErrNotExist:
			tr.rootHash = tr.emptyRootHash()
		default:
			return err
		}
	}

	return tr.SetRootHash(tr.rootHash)
}

func (tr *merklePatriciaTree) Stop(_ context.Context) error {
	return nil
}

func (tr *merklePatriciaTree) RootHash() []byte {
	return tr.rootHash
}

func (tr *merklePatriciaTree) SetRootHash(rootHash []byte) error {
	if len(rootHash) == 0 {
		rootHash = tr.emptyRootHash()
	}
	node, err := tr.loadNodeFromDB(rootHash)
	if err != nil {
		return err
	}
	root, ok := node.(*branchNode)
	if !ok {
		return errors.Wrapf(trie.ErrInvalidTrie, "root should be a branch")
	}
	tr.resetRoot(root)

	return nil
}

func (tr *merklePatriciaTree) IsEmpty() bool {
	return tr.isEmptyRootHash(tr.rootHash)
}

func (tr *merklePatriciaTree) Get(key []byte) ([]byte, error) {
	trieMtc.WithLabelValues("root", "Get").Inc()
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return nil, err
	}
	t := tr.root.search(kt, 0)
	if t == nil {
		return nil, trie.ErrNotExist
	}
	if l, ok := t.(*leafNode); ok {
		return l.Value(), nil
	}
	return nil, trie.ErrInvalidTrie
}

func (tr *merklePatriciaTree) Delete(key []byte) error {
	trieMtc.WithLabelValues("root", "Delete").Inc()
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return err
	}
	child, err := tr.root.child(kt[0])
	if err != nil {
		return errors.Wrapf(trie.ErrNotExist, "key %x does not exist", kt)
	}
	newChild, err := child.delete(kt, 1)
	if err != nil {
		return err
	}
	newRoot, err := tr.root.updateChild(kt[0], newChild)
	if err != nil {
		return err
	}
	tr.resetRoot(newRoot)

	return nil
}

func (tr *merklePatriciaTree) Upsert(key []byte, value []byte) error {
	trieMtc.WithLabelValues("root", "Upsert").Inc()
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return err
	}
	newRoot, err := tr.root.upsert(kt, 0, value)
	if err != nil {
		return err
	}
	bn, ok := newRoot.(*branchNode)
	if !ok {
		panic("unexpected new root")
	}
	tr.resetRoot(bn)

	return nil
}

func (tr *merklePatriciaTree) deleteNodeFromDB(tn node) error {
	return tr.kvStore.Delete(tr.nodeHash(tn))
}

func (tr *merklePatriciaTree) putNodeIntoDB(tn node) error {
	h := tr.nodeHash(tn)
	if tr.isEmptyRootHash(h) {
		return nil
	}
	s := tn.serialize()
	return tr.kvStore.Put(h, s)
}

func (tr *merklePatriciaTree) loadNodeFromDB(key []byte) (node, error) {
	if tr.isEmptyRootHash(key) {
		return newEmptyBranchNode(tr), nil
	}
	s, err := tr.kvStore.Get(key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key %x", key)
	}
	pb := triepb.NodePb{}
	if err := proto.Unmarshal(s, &pb); err != nil {
		return nil, err
	}
	if pbBranch := pb.GetBranch(); pbBranch != nil {
		return newBranchNodeFromProtoPb(tr, pbBranch), nil
	}
	if pbLeaf := pb.GetLeaf(); pbLeaf != nil {
		return newLeafNodeFromProtoPb(tr, pbLeaf), nil
	}
	if pbExtend := pb.GetExtend(); pbExtend != nil {
		return newExtensionNodeFromProtoPb(tr, pbExtend), nil
	}
	return nil, errors.New("invalid node type")
}

func (tr *merklePatriciaTree) isEmptyRootHash(h []byte) bool {
	return bytes.Equal(h, tr.emptyRootHash())
}

func (tr *merklePatriciaTree) emptyRootHash() []byte {
	return tr.nodeHash(newEmptyBranchNode(tr))
}

func (tr *merklePatriciaTree) nodeHash(tn node) []byte {
	if tn == nil {
		panic("unexpected nil node to hash")
	}
	return tr.hashFunc(tn.serialize())
}

func (tr *merklePatriciaTree) resetRoot(newRoot *branchNode) {
	tr.root = newRoot
	h := tr.nodeHash(newRoot)
	tr.rootHash = make([]byte, len(h))
	copy(tr.rootHash, h)
}

func (tr *merklePatriciaTree) checkKeyType(key []byte) (keyType, error) {
	if len(key) != tr.keyLength {
		return nil, errors.Errorf("invalid key length %d", len(key))
	}
	kt := make([]byte, tr.keyLength)
	copy(kt, key)

	return kt, nil
}
