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
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

var (
	stateDBMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_state_db",
			Help: "IoTeX State DB",
		},
		[]string{"type"},
	)
	dbBatchSizelMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_db_batch_size",
			Help: "DB batch size",
		},
		[]string{},
	)
)

func init() {
	prometheus.MustRegister(stateDBMtc)
	prometheus.MustRegister(dbBatchSizelMtc)
}

type (
	// WorkingSet defines an interface for working set of states changes
	WorkingSet interface {
		// states and actions
		//RunActions(context.Context, uint64, []action.SealedEnvelope) (map[hash.Hash32B]*action.Receipt, error)
		RunAction(protocol.RunActionsCtx, action.SealedEnvelope) (*action.Receipt, error)
		UpdateBlockLevelInfo(blockHeight uint64) hash.Hash256
		RunActions(context.Context, uint64, []action.SealedEnvelope) ([]*action.Receipt, error)
		Snapshot() int
		Revert(int) error
		Commit() error
		RootHash() hash.Hash256
		Digest() hash.Hash256
		Version() uint64
		Height() uint64
		// General state
		State(hash.Hash160, interface{}) error
		PutState(hash.Hash160, interface{}) error
		DelState(pkHash hash.Hash160) error
		GetDB() db.KVStore
		GetCachedBatch() db.CachedBatch
	}

	// workingSet implements WorkingSet interface, tracks pending changes to account/contract in local cache
	workingSet struct {
		ver            uint64
		blkHeight      uint64
		accountTrie    trie.Trie            // global account state trie
		trieRoots      map[int]hash.Hash256 // root of trie at time of snapshot
		cb             db.CachedBatch       // cached batch for pending writes
		dao            db.KVStore           // the underlying DB for account/contract storage
		actionHandlers []protocol.ActionHandler
	}
)

// NewWorkingSet creates a new working set
func NewWorkingSet(
	version uint64,
	kv db.KVStore,
	root hash.Hash256,
	actionHandlers []protocol.ActionHandler,
) (WorkingSet, error) {
	ws := &workingSet{
		ver:            version,
		trieRoots:      make(map[int]hash.Hash256),
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
func (ws *workingSet) RootHash() hash.Hash256 {
	return hash.BytesToHash256(ws.accountTrie.RootHash())
}

// Digest returns the delta state digest
func (ws *workingSet) Digest() hash.Hash256 { return hash.ZeroHash256 }

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
) ([]*action.Receipt, error) {
	// Handle actions
	receipts := make([]*action.Receipt, 0)
	var raCtx protocol.RunActionsCtx
	if len(elps) > 0 {
		raCtx = protocol.MustGetRunActionsCtx(ctx)
	}
	for _, elp := range elps {
		receipt, err := ws.RunAction(raCtx, elp)
		if err != nil {
			return nil, errors.Wrap(err, "error when run action")
		}
		if receipt != nil {
			raCtx.GasLimit -= receipt.GasConsumed
			receipts = append(receipts, receipt)
		}
	}
	ws.UpdateBlockLevelInfo(blockHeight)
	return receipts, nil
}

// RunAction runs action in the block and track pending changes in working set
func (ws *workingSet) RunAction(
	raCtx protocol.RunActionsCtx,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	// Handle action
	// Add caller address into the run action context
	caller, err := address.FromBytes(elp.SrcPubkey().Hash())
	if err != nil {
		return nil, err
	}
	raCtx.Caller = caller
	raCtx.ActionHash = elp.Hash()
	raCtx.GasPrice = elp.GasPrice()
	intrinsicGas, err := elp.IntrinsicGas()
	if err != nil {
		return nil, err
	}
	raCtx.IntrinsicGas = intrinsicGas
	raCtx.Nonce = elp.Nonce()
	ctx := protocol.WithRunActionsCtx(context.Background(), raCtx)

	for _, actionHandler := range ws.actionHandlers {
		receipt, err := actionHandler.Handle(ctx, elp.Action(), ws)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"error when action %x (nonce: %d) from %s mutates states",
				elp.Hash(),
				elp.Nonce(),
				caller.String(),
			)
		}
		if receipt != nil {
			return receipt, nil
		}
	}
	return nil, nil
}

// UpdateBlockLevelInfo runs action in the block and track pending changes in working set
func (ws *workingSet) UpdateBlockLevelInfo(blockHeight uint64) hash.Hash256 {
	ws.blkHeight = blockHeight
	// Persist accountTrie's root hash
	rootHash := ws.accountTrie.RootHash()
	ws.cb.Put(AccountKVNameSpace, []byte(AccountTrieRootKey), rootHash, "failed to store accountTrie's root hash")
	// Persist current chain Height
	h := byteutil.Uint64ToBytes(blockHeight)
	ws.cb.Put(AccountKVNameSpace, []byte(CurrentHeightKey), h, "failed to store accountTrie's current Height")
	// Persist the historical accountTrie's root hash
	ws.cb.Put(
		AccountKVNameSpace,
		[]byte(fmt.Sprintf("%s-%d", AccountTrieRootKey, blockHeight)),
		rootHash,
		"failed to store accountTrie's root hash",
	)
	return ws.RootHash()
}

func (ws *workingSet) Snapshot() int {
	s := ws.cb.Snapshot()
	ws.trieRoots[s] = hash.BytesToHash256(ws.accountTrie.RootHash())
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
	dbBatchSizelMtc.WithLabelValues().Set(float64(ws.cb.Size()))
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
func (ws *workingSet) State(hash hash.Hash160, s interface{}) error {
	stateDBMtc.WithLabelValues("get").Inc()
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
func (ws *workingSet) PutState(pkHash hash.Hash160, s interface{}) error {
	stateDBMtc.WithLabelValues("put").Inc()
	ss, err := state.Serialize(s)
	if err != nil {
		return errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}
	return ws.accountTrie.Upsert(pkHash[:], ss)
}

// DelState deletes a state from DB
func (ws *workingSet) DelState(pkHash hash.Hash160) error {
	return ws.accountTrie.Delete(pkHash[:])
}

// clearCache removes all local changes after committing to trie
func (ws *workingSet) clear() {
	ws.trieRoots = nil
	ws.trieRoots = make(map[int]hash.Hash256)
}
