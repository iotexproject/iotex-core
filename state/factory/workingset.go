// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// WorkingSet defines an interface for working set of states changes
	WorkingSet interface {
		// states and actions
		RunActions(context.Context, uint64, []action.SealedEnvelope) (hash.Hash32B, map[hash.Hash32B]*action.Receipt, error)
		Snapshot() int
		Revert(int) error
		Commit() error
		RootHash() hash.Hash32B
		Version() uint64
		Height() uint64
		// General state
		State(hash.PKHash, interface{}) error
		PutState(hash.PKHash, interface{}) error
		DelState(pkHash hash.PKHash) error
		GetDB() db.KVStore
		GetCachedBatch() db.CachedBatch
	}

	// workingSet implements WorkingSet interface, tracks pending changes to account/contract in local cache
	workingSet struct {
		ver            uint64
		blkHeight      uint64
		accountTrie    trie.Trie            // global account state trie
		trieRoots      map[int]hash.Hash32B // root of trie at time of snapshot
		cb             db.CachedBatch       // cached batch for pending writes
		dao            db.KVStore           // the underlying DB for account/contract storage
		actionHandlers []protocol.ActionHandler
	}
)

// NewWorkingSet creates a new working set
func NewWorkingSet(
	version uint64,
	kv db.KVStore,
	root hash.Hash32B,
	actionHandlers []protocol.ActionHandler,
) (WorkingSet, error) {
	ws := &workingSet{
		ver:            version,
		trieRoots:      make(map[int]hash.Hash32B),
		cb:             db.NewCachedBatch(),
		dao:            kv,
		actionHandlers: actionHandlers,
	}
	dbForTrie, err := db.NewKVStoreForTrie(AccountKVNameSpace, ws.dao, db.CachedBatchOption(ws.cb))
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate state tire db")
	}
	tr, err := trie.NewTrie(trie.KVStoreOption(dbForTrie), trie.RootHashOption(root[:]))
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate state trie from config")
	}
	ws.accountTrie = tr
	if err := ws.accountTrie.Start(context.Background()); err != nil {
		return nil, errors.Wrapf(err, "failed to load state trie from root = %x", root)
	}
	return ws, nil
}

// RootHash returns the hash of the root node of the accountTrie
func (ws *workingSet) RootHash() hash.Hash32B {
	return byteutil.BytesTo32B(ws.accountTrie.RootHash())
}

// Version returns the Version of this working set
func (ws *workingSet) Version() uint64 {
	return ws.ver
}

// Height returns the Height of the block being worked on
func (ws *workingSet) Height() uint64 {
	return ws.blkHeight
}

// RunActions runs actions in the block and track pending changes in working set
func (ws *workingSet) RunActions(
	ctx context.Context,
	blockHeight uint64,
	elps []action.SealedEnvelope,
) (hash.Hash32B, map[hash.Hash32B]*action.Receipt, error) {
	ws.blkHeight = blockHeight
	// Handle actions
	receipts := make(map[hash.Hash32B]*action.Receipt)
	for _, elp := range elps {
		for _, actionHandler := range ws.actionHandlers {
			receipt, err := actionHandler.Handle(ctx, elp.Action(), ws)
			if err != nil {
				return hash.ZeroHash32B, nil, errors.Wrapf(
					err,
					"error when action %x (nonce: %d) from %s mutates states",
					elp.Hash(),
					elp.Nonce(),
					elp.SrcAddr(),
				)
			}
			if receipt != nil {
				receipts[elp.Hash()] = receipt
			}
		}
	}

	// Persist accountTrie's root hash
	rootHash := ws.accountTrie.RootHash()
	ws.cb.Put(AccountKVNameSpace, []byte(AccountTrieRootKey), rootHash[:], "failed to store accountTrie's root hash")
	// Persist current chain Height
	h := byteutil.Uint64ToBytes(blockHeight)
	ws.cb.Put(AccountKVNameSpace, []byte(CurrentHeightKey), h, "failed to store accountTrie's current Height")
	// Persis the historical accountTrie's root hash
	ws.cb.Put(
		AccountKVNameSpace,
		[]byte(fmt.Sprintf("%s-%d", AccountTrieRootKey, blockHeight)),
		rootHash[:],
		"failed to store accountTrie's root hash",
	)

	return ws.RootHash(), receipts, nil
}

func (ws *workingSet) Snapshot() int {
	s := ws.cb.Snapshot()
	ws.trieRoots[s] = byteutil.BytesTo32B(ws.accountTrie.RootHash())
	return s
}

func (ws *workingSet) Revert(snapshot int) error {
	if err := ws.cb.Revert(snapshot); err != nil {
		return err
	}
	root, ok := ws.trieRoots[snapshot]
	if !ok {
		// this should not happen, b/c we save the trie root on a successful return of Snapshot(), but check anyway
		return errors.Wrapf(trie.ErrInvalidTrie, "failed to get trie root for snapshot = %d", snapshot)
	}
	return ws.accountTrie.SetRootHash(root[:])
}

// Commit persists all changes in RunActions() into the DB
func (ws *workingSet) Commit() error {
	// Commit all changes in a batch
	if err := ws.dao.Commit(ws.cb); err != nil {
		return errors.Wrap(err, "failed to Commit all changes to underlying DB in a batch")
	}
	ws.clear()
	return nil
}

// GetDB returns the underlying DB for account/contract storage
func (ws *workingSet) GetDB() db.KVStore {
	return ws.dao
}

// GetCachedBatch returns the cached batch for pending writes
func (ws *workingSet) GetCachedBatch() db.CachedBatch {
	return ws.cb
}

// State pulls a state from DB
func (ws *workingSet) State(hash hash.PKHash, s interface{}) error {
	mstate, err := ws.accountTrie.Get(hash[:])
	if errors.Cause(err) == trie.ErrNotExist {
		return errors.Wrapf(state.ErrStateNotExist, "addrHash = %x", hash[:])
	}
	if err != nil {
		return errors.Wrapf(err, "failed to get account of %x", hash)
	}
	return state.Deserialize(s, mstate)
}

// PutState puts a state into DB
func (ws *workingSet) PutState(pkHash hash.PKHash, s interface{}) error {
	ss, err := state.Serialize(s)
	if err != nil {
		return errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}
	return ws.accountTrie.Upsert(pkHash[:], ss)
}

// DelState deletes a state from DB
func (ws *workingSet) DelState(pkHash hash.PKHash) error {
	return ws.accountTrie.Delete(pkHash[:])
}

// clearCache removes all local changes after committing to trie
func (ws *workingSet) clear() {
	ws.trieRoots = nil
	ws.trieRoots = make(map[int]hash.Hash32B)
}
