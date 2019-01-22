// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"
	"context"
	"sync"

	"github.com/iotexproject/iotex-core/db"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/db/trie/triepb"
	"github.com/pkg/errors"
)

type (
	// HashFunc defines a function to generate the hash which will be used as key in db
	HashFunc       func([]byte) []byte
	branchRootTrie struct {
		mutex                sync.RWMutex
		keyLength            int
		kvStore              KVStore
		hashFunc             HashFunc
		root                 *branchNode
		rootHash             []byte
		rootKey              string
		preCalcEmptyRootHash []byte
	}
)

func (tr *branchRootTrie) Start(ctx context.Context) error {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()
	emptyRoot := newEmptyBranchNode()
	emptyRootHash, err := emptyRoot.hash(tr, false)
	if err != nil {
		return err
	}
	tr.preCalcEmptyRootHash = emptyRootHash
	if tr.rootKey != "" {
		switch root, err := tr.kvStore.Get([]byte(tr.rootKey)); errors.Cause(err) {
		case nil:
			tr.rootHash = root
		case db.ErrNotExist:
			tr.resetRoot(emptyRoot)
			return nil
		default:
			return err
		}
	}

	return tr.SetRootHash(tr.rootHash)
}

func (tr *branchRootTrie) Stop(_ context.Context) error {
	return nil
}

func (tr *branchRootTrie) RootNode() Node {
	return tr.root
}

func (tr *branchRootTrie) SetRootNode(root Node) error {
	bn, ok := root.(*branchNode)
	if !ok {
		return errors.Wrapf(ErrInvalidTrie, "root should be a branch")
	}
	tr.resetRoot(bn)

	return nil
}

func (tr *branchRootTrie) RootHash() []byte {
	h, err := tr.root.hash(tr, false)
	if err != nil {
		panic("failed to get hash of trie")
	}
	return h
}

func (tr *branchRootTrie) Commit() ([]byte, error) {
	h, err := tr.root.hash(tr, true)
	if err == nil {
		tr.rootHash = make([]byte, len(h))
		copy(tr.rootHash, h)
	}
	return h, err
}

func (tr *branchRootTrie) Clone() Trie {
	rootHash := make([]byte, len(tr.rootHash))
	copy(rootHash, tr.rootHash)

	return &branchRootTrie{
		hashFunc:  tr.hashFunc,
		keyLength: tr.keyLength,
		kvStore:   tr.kvStore,
		root:      tr.root,
		rootHash:  rootHash,
		rootKey:   tr.rootKey,
	}
}

func (tr *branchRootTrie) SetRootHash(rootHash []byte) error {
	if len(rootHash) == 0 {
		rootHash = tr.emptyRootHash()
	}
	node, err := tr.LoadNodeFromDB(rootHash)
	if err != nil {
		return err
	}
	root, ok := node.(*branchNode)
	if !ok {
		return errors.Wrapf(ErrInvalidTrie, "root should be a branch")
	}
	tr.rootHash = make([]byte, len(rootHash))
	copy(tr.rootHash, rootHash)
	tr.resetRoot(root)

	return nil
}

func (tr *branchRootTrie) Get(key []byte) ([]byte, error) {
	trieMtc.WithLabelValues("root", "Get").Inc()
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return nil, err
	}
	t, err := tr.root.search(tr, kt, 0)
	if err != nil {
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
	child, ok := tr.root.children[kt[0]]
	if !ok {
		return errors.Wrapf(ErrNotExist, "key %x does not exist", kt)
	}
	newChild, err := child.delete(tr, kt, 1)
	if err != nil {
		return err
	}
	newRoot := tr.root.updateChild(kt[0], newChild)
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

func (tr *branchRootTrie) deleteNodeFromDB(node Node) error {
	hn, ok := node.(*hashNode)
	if !ok {
		return errors.New("invalid node type")
	}
	return tr.kvStore.Delete(hn.ha)
}

func (tr *branchRootTrie) putNodeIntoDB(node Node) error {
	hn, ok := node.(*hashNode)
	if !ok {
		return errors.New("invalid node type")
	}
	if tr.isEmptyRootHash(hn.ha) {
		return nil
	}
	if hn.sa == nil {
		return errors.New("hash node's serialize array is nil")
	}
	return tr.kvStore.Put(hn.ha, hn.sa)
}

func (tr *branchRootTrie) LoadNodeFromDB(hash []byte) (Node, error) {
	if tr.isEmptyRootHash(hash) {
		return newEmptyBranchNode(), nil
	}
	s, err := tr.kvStore.Get(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read from db %x", hash)
	}
	pb := triepb.NodePb{}
	if err := proto.Unmarshal(s, &pb); err != nil {
		return nil, err
	}
	if pbBranch := pb.GetBranch(); pbBranch != nil {
		node := newEmptyBranchNode()
		for _, child := range pbBranch.Branches {
			idx := child.GetIndex()
			if idx > radix {
				return nil, errors.Errorf("invalid branch node index %d", idx)
			}
			node.children[byte(idx)] = newHashNode(child.GetChildHash())
		}
		return node, nil
	}
	if pbLeaf := pb.GetLeaf(); pbLeaf != nil {
		if len(pbLeaf.GetKey()) != tr.keyLength {
			return nil, errors.New("the key length is invalid")
		}
		return &leafNode{key: pbLeaf.GetKey(), value: pbLeaf.GetValue()}, nil
	}
	if pbExtend := pb.GetExtend(); pbExtend != nil {
		return &extensionNode{
			path:  pbExtend.GetPath(),
			child: newHashNode(pbExtend.GetChildHash()),
		}, nil
	}
	return nil, errors.New("invalid node type")
}

func (tr *branchRootTrie) isEmptyRoot(tn Node) bool {
	if bn, ok := tn.(*branchNode); ok {
		return len(bn.Children()) == 0
	}
	return false
}

func (tr *branchRootTrie) isEmptyRootHash(h []byte) bool {
	return bytes.Equal(h, tr.emptyRootHash())
}

func (tr *branchRootTrie) emptyRootHash() []byte {
	return tr.preCalcEmptyRootHash
}

func (tr *branchRootTrie) hash(data []byte) []byte {
	return tr.hashFunc(data)
}

func (tr *branchRootTrie) resetRoot(newRoot *branchNode) {
	tr.root = newRoot
}

func (tr *branchRootTrie) checkKeyType(key []byte) (keyType, error) {
	if len(key) != tr.keyLength {
		return nil, errors.Errorf("invalid key length %d", len(key))
	}
	kt := make([]byte, tr.keyLength)
	copy(kt, key)

	return kt, nil
}
