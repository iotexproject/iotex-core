// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"
	"context"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
)

type (
	// HashFunc defines a function to generate the hash which will be used as key in db
	HashFunc       func([]byte) []byte
	branchRootTrie struct {
		mutex     sync.RWMutex
		keyLength int
		kvStore   KVStore
		hashFunc  HashFunc
		root      *branchNode
		rootHash  []byte
		rootKey   string
	}
)

func (tr *branchRootTrie) Start(ctx context.Context) error {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()
	if tr.rootKey != "" {
		switch root, err := tr.kvStore.Get([]byte(tr.rootKey)); errors.Cause(err) {
		case nil:
			tr.rootHash = root
		case db.ErrNotExist:
			tr.rootHash = tr.emptyRootHash()
		default:
			return err
		}
	}

	return tr.SetRootHash(tr.rootHash)
}

func (tr *branchRootTrie) Stop(_ context.Context) error {
	return nil
}

func (tr *branchRootTrie) RootHash() []byte {
	return tr.rootHash
}

func (tr *branchRootTrie) SetRootHash(rootHash []byte) error {
	if len(rootHash) == 0 {
		rootHash = tr.emptyRootHash()
	}
	node, err := tr.loadNodeFromDB(rootHash)
	if err != nil {
		return err
	}
	root, ok := node.(*branchNode)
	if !ok {
		return errors.Wrapf(ErrInvalidTrie, "root should be a branch")
	}
	tr.resetRoot(root)

	return nil
}

func (tr *branchRootTrie) Get(key []byte) ([]byte, error) {
	trieMtc.WithLabelValues("root", "Get").Inc()
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return nil, err
	}
	t := tr.root.search(tr, kt, 0)
	if t == nil {
		return nil, ErrNotExist
	}
	if l, ok := t.(*leafNode); ok {
		return l.Value(), nil
	}
	return nil, ErrInvalidTrie
}

func (tr *branchRootTrie) Delete(key []byte) error {
	trieMtc.WithLabelValues("root", "Delete").Inc()
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return err
	}
	child, err := tr.root.child(tr, kt[0])
	if err != nil {
		return errors.Wrapf(ErrNotExist, "key %x does not exist", kt)
	}
	newChild, err := child.delete(tr, kt, 1)
	if err != nil {
		return err
	}
	newRoot, err := tr.root.updateChild(tr, kt[0], newChild)
	if err != nil {
		return err
	}
	tr.resetRoot(newRoot)

	return nil
}

func (tr *branchRootTrie) Upsert(key []byte, value []byte) error {
	trieMtc.WithLabelValues("root", "Upsert").Inc()
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return err
	}
	newRoot, err := tr.root.upsert(tr, kt, 0, value)
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

func (tr *branchRootTrie) DB() KVStore {
	return tr.kvStore
}

func (tr *branchRootTrie) deleteNodeFromDB(tn Node) error {
	return tr.kvStore.Delete(tr.nodeHash(tn))
}

func (tr *branchRootTrie) putNodeIntoDB(tn Node) error {
	h := tr.nodeHash(tn)
	if tr.isEmptyRootHash(h) {
		return nil
	}
	s := tn.serialize()
	return tr.kvStore.Put(h, s)
}

func (tr *branchRootTrie) loadNodeFromDB(key []byte) (Node, error) {
	if tr.isEmptyRootHash(key) {
		return newEmptyBranchNode(), nil
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
		return newBranchNodeFromProtoPb(pbBranch), nil
	}
	if pbLeaf := pb.GetLeaf(); pbLeaf != nil {
		return newLeafNodeFromProtoPb(pbLeaf), nil
	}
	if pbExtend := pb.GetExtend(); pbExtend != nil {
		return newExtensionNodeFromProtoPb(pbExtend), nil
	}
	return nil, errors.New("invalid node type")
}

func (tr *branchRootTrie) isEmptyRootHash(h []byte) bool {
	return bytes.Equal(h, tr.emptyRootHash())
}

func (tr *branchRootTrie) emptyRootHash() []byte {
	return tr.nodeHash(newEmptyBranchNode())
}

func (tr *branchRootTrie) nodeHash(tn Node) []byte {
	if tn == nil {
		panic("unexpected nil node to hash")
	}
	return tr.hashFunc(tn.serialize())
}

func (tr *branchRootTrie) resetRoot(newRoot *branchNode) {
	tr.root = newRoot
	h := tr.nodeHash(newRoot)
	tr.rootHash = make([]byte, len(h))
	copy(tr.rootHash, h)
}

func (tr *branchRootTrie) checkKeyType(key []byte) (keyType, error) {
	if len(key) != tr.keyLength {
		return nil, errors.Errorf("invalid key length %d", len(key))
	}
	kt := make([]byte, tr.keyLength)
	copy(kt, key)

	return kt, nil
}
