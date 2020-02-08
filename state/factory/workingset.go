// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"fmt"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
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
		protocol.StateManager
		// states and actions
		RunAction(context.Context, action.SealedEnvelope) (*action.Receipt, error)
		RunActions(context.Context, []action.SealedEnvelope) ([]*action.Receipt, error)
		Finalize() error
		Commit() error
		RootHash() (hash.Hash256, error)
		Digest() (hash.Hash256, error)
		Version() uint64
		History() bool
	}

	// workingSet implements WorkingSet interface, tracks pending changes to account/contract in local cache
	workingSet struct {
		finalized   bool
		blockHeight uint64
		saveHistory bool
		accountTrie trie.Trie            // global account state trie
		trieRoots   map[int]hash.Hash256 // root of trie at time of snapshot
		cb          batch.CachedBatch    // cached batch for pending writes
		dao         db.KVStore           // the underlying DB for account/contract storage
	}
)

// newWorkingSet creates a new working set
func newWorkingSet(
	height uint64,
	kv db.KVStore,
	root hash.Hash256,
	saveHistory bool,
) (WorkingSet, error) {
	ws := &workingSet{
		finalized:   false,
		blockHeight: height,
		saveHistory: saveHistory,
		trieRoots:   make(map[int]hash.Hash256),
		cb:          batch.NewCachedBatch(),
		dao:         kv,
	}
	dbForTrie, err := db.NewKVStoreForTrie(protocol.AccountNameSpace, evm.PruneKVNameSpace, ws.dao, db.CachedBatchOption(ws.cb))
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
func (ws *workingSet) RootHash() (hash.Hash256, error) {
	if !ws.finalized {
		return hash.ZeroHash256, errors.Errorf("working set has not been finalized")
	}
	return hash.BytesToHash256(ws.accountTrie.RootHash()), nil
}

// Digest returns the delta state digest
func (ws *workingSet) Digest() (hash.Hash256, error) {
	return hash.ZeroHash256, nil
}

// Version returns the Version of this working set
func (ws *workingSet) Version() uint64 {
	return ws.blockHeight
}

// Height returns the Height of the block being worked on
func (ws *workingSet) Height() (uint64, error) {
	return ws.blockHeight, nil
}

func (ws *workingSet) History() bool {
	return ws.saveHistory
}

// RunActions runs actions in the block and track pending changes in working set
func (ws *workingSet) RunActions(
	ctx context.Context,
	elps []action.SealedEnvelope,
) ([]*action.Receipt, error) {
	// Handle actions
	receipts := make([]*action.Receipt, 0)
	for _, elp := range elps {
		receipt, err := ws.runAction(ctx, elp)
		if err != nil {
			return nil, errors.Wrap(err, "error when run action")
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
	}

	return receipts, nil
}

func (ws *workingSet) RunAction(
	ctx context.Context,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	return ws.runAction(ctx, elp)
}

func (ws *workingSet) runAction(
	ctx context.Context,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	if ws.finalized {
		return nil, errors.Errorf("cannot run action on a finalized working set")
	}
	// Handle action
	var actionCtx protocol.ActionCtx
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	if blkCtx.BlockHeight != ws.blockHeight {
		return nil, errors.Errorf(
			"invalid block height %d, %d expected",
			blkCtx.BlockHeight,
			ws.blockHeight,
		)
	}
	caller, err := address.FromBytes(elp.SrcPubkey().Hash())
	if err != nil {
		return nil, err
	}
	actionCtx.Caller = caller
	actionCtx.ActionHash = elp.Hash()
	actionCtx.GasPrice = elp.GasPrice()
	intrinsicGas, err := elp.IntrinsicGas()
	if err != nil {
		return nil, err
	}
	actionCtx.IntrinsicGas = intrinsicGas
	actionCtx.Nonce = elp.Nonce()

	ctx = protocol.WithActionCtx(ctx, actionCtx)
	if bcCtx.Registry == nil {
		return nil, nil
	}
	for _, actionHandler := range bcCtx.Registry.All() {
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

// Finalize runs action in the block and track pending changes in working set
func (ws *workingSet) Finalize() error {
	if ws.finalized {
		return errors.New("Cannot finalize a working set twice")
	}
	ws.finalized = true
	// Persist accountTrie's root hash
	rootHash := ws.accountTrie.RootHash()
	ws.cb.Put(protocol.AccountNameSpace, []byte(AccountTrieRootKey), rootHash, "failed to store accountTrie's root hash")
	// Persist current chain Height
	h := byteutil.Uint64ToBytes(ws.blockHeight)
	ws.cb.Put(protocol.AccountNameSpace, []byte(CurrentHeightKey), h, "failed to store accountTrie's current Height")
	// Persist the historical accountTrie's root hash
	ws.cb.Put(
		protocol.AccountNameSpace,
		[]byte(fmt.Sprintf("%s-%d", AccountTrieRootKey, ws.blockHeight)),
		rootHash,
		"failed to store accountTrie's root hash",
	)
	return nil
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
	var cb batch.KVStoreBatch
	if ws.saveHistory {
		// exclude trie deletion
		cb = ws.cb.ExcludeEntries("", batch.Delete)
	} else {
		cb = ws.cb
	}
	if err := ws.dao.WriteBatch(cb); err != nil {
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
func (ws *workingSet) GetCachedBatch() batch.CachedBatch {
	return ws.cb
}

// State pulls a state from DB
func (ws *workingSet) State(hash hash.Hash160, s interface{}, opts ...protocol.StateOption) error {
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

// StateAtHeight pulls a state from DB
func (ws *workingSet) StateAtHeight(height uint64, hash hash.Hash160, s interface{}) error {
	if !ws.saveHistory {
		return ErrNoArchiveData
	}
	// get root through height
	rootHash, err := ws.dao.Get(protocol.AccountNameSpace, []byte(fmt.Sprintf("%s-%d", AccountTrieRootKey, height)))
	if err != nil {
		return errors.Wrap(err, "failed to get root hash through height")
	}
	dbForTrie, err := db.NewKVStoreForTrie(protocol.AccountNameSpace, evm.PruneKVNameSpace, ws.dao, db.CachedBatchOption(batch.NewCachedBatch()))
	if err != nil {
		return errors.Wrap(err, "failed to generate state tire db")
	}
	tr, err := trie.NewTrie(trie.KVStoreOption(dbForTrie), trie.RootHashOption(rootHash))
	if err != nil {
		return errors.Wrap(err, "failed to generate state trie from config")
	}
	err = tr.Start(context.Background())
	if err != nil {
		return err
	}
	defer tr.Stop(context.Background())
	mstate, err := tr.Get(hash[:])
	if errors.Cause(err) == trie.ErrNotExist {
		return errors.Wrapf(state.ErrStateNotExist, "addrHash = %x", hash[:])
	}
	if err != nil {
		return errors.Wrapf(err, "failed to get account of %x", hash)
	}
	return state.Deserialize(s, mstate)
}

// PutState puts a state into DB
func (ws *workingSet) PutState(pkHash hash.Hash160, s interface{}, opts ...protocol.StateOption) error {
	stateDBMtc.WithLabelValues("put").Inc()
	ss, err := state.Serialize(s)
	if err != nil {
		return errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}
	return ws.accountTrie.Upsert(pkHash[:], ss)
}

// DelState deletes a state from DB
func (ws *workingSet) DelState(pkHash hash.Hash160, opts ...protocol.StateOption) error {
	return ws.accountTrie.Delete(pkHash[:])
}

// clearCache removes all local changes after committing to trie
func (ws *workingSet) clear() {
	ws.trieRoots = nil
	ws.trieRoots = make(map[int]hash.Hash256)
}
