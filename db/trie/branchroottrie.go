// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"
	"reflect"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type branchRootTrie struct {
	lc    lifecycle.Lifecycle
	mutex sync.RWMutex
	tc    SameKeyLenTrieContext
	root  *branchNode
}

func newBranchRootTrie(tc SameKeyLenTrieContext) *branchRootTrie {
	return &branchRootTrie{tc: tc}
}

func (tr *branchRootTrie) Start(ctx context.Context) error {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()
	if err := tr.lc.OnStart(ctx); err != nil {
		return err
	}
	if tr.tc.RootKey != "" {
		switch root, err := tr.tc.DB.Get(tr.tc.Bucket, []byte(tr.tc.RootKey)); errors.Cause(err) {
		case nil:
			tr.tc.InitRootHash = byteutil.BytesTo32B(root)
		case bolt.ErrBucketNotFound:
			tr.tc.InitRootHash = EmptyBranchNodeHash
		case badger.ErrKeyNotFound:
			tr.tc.InitRootHash = EmptyBranchNodeHash
		default:
			return err
		}
	}

	return tr.SetRoot(tr.tc.InitRootHash)
}

func (tr *branchRootTrie) Stop(ctx context.Context) error {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	return tr.lc.OnStop(ctx)
}

func (tr *branchRootTrie) RootHash() hash.Hash32B {
	return tr.tc.InitRootHash
}

func (tr *branchRootTrie) TrieDB() db.KVStore {
	return tr.tc.DB
}

func (tr *branchRootTrie) Commit() error {
	return tr.tc.Commit()
}

func (tr *branchRootTrie) SetRoot(rootHash hash.Hash32B) error {
	logger.Debug().Hex("hash", rootHash[:]).Msg("reset root hash")
	var root *branchNode
	switch rootHash {
	case EmptyBranchNodeHash:
		var err error
		root, err = tr.tc.newBranchNodeAndPutIntoDB(nil)
		if err != nil {
			return err
		}
	default:
		node, err := tr.tc.LoadNodeFromDB(rootHash)
		if err != nil {
			return err
		}
		bn, ok := node.(*branchNode)
		if !ok {
			return errors.Wrapf(ErrInvalidTrie, "root should be a branch")
		}
		root = bn
	}
	tr.resetRoot(root)

	return nil
}

func (tr *branchRootTrie) Get(key []byte) ([]byte, error) {
	logger.Debug().Hex("key", key).Msg("Get value by key")
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return nil, err
	}
	t := tr.root.search(tr.tc, kt, 0)
	if t == nil {
		return nil, ErrNotExist
	}
	if l, ok := t.(*leafNode); ok {
		return l.Value(), nil
	}
	return nil, ErrInvalidTrie
}

func (tr *branchRootTrie) Delete(key []byte) error {
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return err
	}
	child, err := tr.root.child(tr.tc, kt[0])
	if err != nil {
		return errors.Wrapf(ErrNotExist, "key %x does not exist", kt)
	}
	newChild, err := child.delete(tr.tc, kt, 1)
	if err != nil {
		return err
	}
	newRoot, err := tr.root.updateChild(tr.tc, kt[0], newChild)
	if err != nil {
		return err
	}
	tr.resetRoot(newRoot)

	return nil
}

func (tr *branchRootTrie) Upsert(key []byte, value []byte) error {
	logger.Debug().Hex("key", key).Hex("value", value).Msg("Upsert into branch root trie")
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return err
	}
	newRoot, err := tr.root.upsert(tr.tc, kt, 0, value)
	if err != nil {
		return err
	}
	bn, ok := newRoot.(*branchNode)
	if !ok {
		logger.Panic().
			Interface("rootType", reflect.TypeOf(newRoot)).
			Msg("unexpected new root")
	}
	tr.resetRoot(bn)

	return nil
}

func (tr *branchRootTrie) resetRoot(newRoot *branchNode) {
	tr.root = newRoot
	tr.tc.InitRootHash = nodeHash(newRoot)
}

func (tr *branchRootTrie) checkKeyType(key []byte) (keyType, error) {
	if len(key) != tr.tc.KeyLength {
		return nil, errors.Errorf("invalid key length %d", len(key))
	}
	kt := make([]byte, tr.tc.KeyLength)
	copy(kt, key)

	return kt, nil
}
