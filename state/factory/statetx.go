// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// CheckHistoryDeleteInterval 30 block heights to check if history needs to delete
	CheckHistoryDeleteInterval = 30
)

// stateTX implements stateTX interface, tracks pending changes to account/contract in local cache
type stateTX struct {
	ver            uint64
	blkHeight      uint64
	cb             db.CachedBatch // cached batch for pending writes
	dao            db.KVStore     // the underlying DB for account/contract storage
	actionHandlers []protocol.ActionHandler
	deleting       chan struct{} // make sure there's only one goroutine deleting history state
	cfg            config.DB
}

// newStateTX creates a new state tx
func newStateTX(
	version uint64,
	kv db.KVStore,
	actionHandlers []protocol.ActionHandler,
	cfg config.DB,
) *stateTX {
	return &stateTX{
		ver:            version,
		cb:             db.NewCachedBatch(),
		dao:            kv,
		actionHandlers: actionHandlers,
		deleting:       make(chan struct{}, 1),
		cfg:            cfg,
	}
}

// RootHash returns the hash of the root node of the accountTrie
func (stx *stateTX) RootHash() hash.Hash256 { return hash.ZeroHash256 }

// Digest returns the delta state digest
func (stx *stateTX) Digest() hash.Hash256 { return stx.GetCachedBatch().Digest() }

// Version returns the Version of this working set
func (stx *stateTX) Version() uint64 { return stx.ver }

// Height returns the Height of the block being worked on
func (stx *stateTX) Height() uint64 { return stx.blkHeight }

// RunActions runs actions in the block and track pending changes in working set
func (stx *stateTX) RunActions(
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
		receipt, err := stx.RunAction(raCtx, elp)
		if err != nil {
			return nil, errors.Wrap(err, "error when run action")
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
	}
	stx.UpdateBlockLevelInfo(blockHeight)
	return receipts, nil
}

// RunAction runs action in the block and track pending changes in working set
func (stx *stateTX) RunAction(
	raCtx protocol.RunActionsCtx,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	// Handle action
	// Add caller address into the run action context
	callerAddr, err := address.FromBytes(elp.SrcPubkey().Hash())
	if err != nil {
		return nil, err
	}
	raCtx.Caller = callerAddr
	raCtx.ActionHash = elp.Hash()
	raCtx.GasPrice = elp.GasPrice()
	intrinsicGas, err := elp.IntrinsicGas()
	if err != nil {
		return nil, err
	}
	raCtx.IntrinsicGas = intrinsicGas
	raCtx.Nonce = elp.Nonce()
	ctx := protocol.WithRunActionsCtx(context.Background(), raCtx)
	for _, actionHandler := range stx.actionHandlers {
		receipt, err := actionHandler.Handle(ctx, elp.Action(), stx)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"error when action %x (nonce: %d) from %s mutates states",
				elp.Hash(),
				elp.Nonce(),
				callerAddr.String(),
			)
		}
		if receipt != nil {
			return receipt, nil
		}
	}
	return nil, nil
}

// UpdateBlockLevelInfo runs action in the block and track pending changes in working set
func (stx *stateTX) UpdateBlockLevelInfo(blockHeight uint64) hash.Hash256 {
	if stx.cfg.EnableHistoryState && blockHeight%CheckHistoryDeleteInterval == 0 {
		stx.deleteHistory()
	}
	stx.blkHeight = blockHeight
	// Persist current chain Height
	h := byteutil.Uint64ToBytes(blockHeight)
	stx.cb.Put(AccountKVNameSpace, []byte(CurrentHeightKey), h, "failed to store accountTrie's current Height")
	return hash.ZeroHash256
}

func (stx *stateTX) Snapshot() int { return stx.cb.Snapshot() }

func (stx *stateTX) Revert(snapshot int) error { return stx.cb.Revert(snapshot) }

// Commit persists all changes in RunActions() into the DB
func (stx *stateTX) Commit() error {
	// Commit all changes in a batch
	dbBatchSizelMtc.WithLabelValues().Set(float64(stx.cb.Size()))
	if err := stx.dao.Commit(stx.cb); err != nil {
		return errors.Wrap(err, "failed to Commit all changes to underlying DB in a batch")
	}
	return nil
}

// GetDB returns the underlying DB for account/contract storage
func (stx *stateTX) GetDB() db.KVStore {
	return stx.dao
}

// GetCachedBatch returns the cached batch for pending writes
func (stx *stateTX) GetCachedBatch() db.CachedBatch {
	return stx.cb
}

// State pulls a state from DB
func (stx *stateTX) State(hash hash.Hash160, s interface{}) error {
	stateDBMtc.WithLabelValues("get").Inc()
	mstate, err := stx.cb.Get(AccountKVNameSpace, hash[:])
	if errors.Cause(err) == db.ErrNotExist {
		if mstate, err = stx.dao.Get(AccountKVNameSpace, hash[:]); errors.Cause(err) == db.ErrNotExist {
			return errors.Wrapf(state.ErrStateNotExist, "k = %x doesn't exist", hash)
		}
	}
	if errors.Cause(err) == db.ErrAlreadyDeleted {
		return errors.Wrapf(state.ErrStateNotExist, "k = %x doesn't exist", hash)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to get account of %x", hash)
	}
	return state.Deserialize(s, mstate)
}

// PutState puts a state into DB
func (stx *stateTX) PutState(pkHash hash.Hash160, s interface{}) error {
	stateDBMtc.WithLabelValues("put").Inc()
	ss, err := state.Serialize(s)
	if err != nil {
		return errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}
	stx.cb.Put(AccountKVNameSpace, pkHash[:], ss, "error when putting k = %x", pkHash)
	if !stx.cfg.EnableHistoryState {
		return nil
	}
	return stx.putIndex(pkHash, ss)
}

func (stx *stateTX) getMaxVersion(pkHash hash.Hash160) (uint64, error) {
	indexKey := append(AccountMaxVersionPrefix, pkHash[:]...)
	value, err := stx.dao.Get(AccountKVNameSpace, indexKey)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}

func (stx *stateTX) putIndex(pkHash hash.Hash160, ss []byte) error {
	version := stx.ver + 1
	maxVersion, _ := stx.getMaxVersion(pkHash)
	if (maxVersion != 0) && (maxVersion != 1) && (maxVersion > version) {
		return nil
	}

	currentVersion := make([]byte, 8)
	binary.BigEndian.PutUint64(currentVersion, version)
	indexKey := append(AccountMaxVersionPrefix, pkHash[:]...)
	err := stx.dao.Put(AccountKVNameSpace, indexKey, currentVersion)
	if err != nil {
		return err
	}
	stateKey := append(pkHash[:], currentVersion...)
	return stx.dao.Put(AccountKVNameSpace, stateKey, ss)
}

// deleteAccountHistory delete history of account
func (stx *stateTX) deleteAccountHistory(pkHash hash.Hash160, deleteHeight uint64) error {
	db := stx.dao.DB()
	boltdb, ok := db.(*bolt.DB)
	if !ok {
		log.L().Error("convert to bolt db error")
		return nil
	}
	prefix := pkHash[:]
	err := boltdb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(AccountKVNameSpace))
		if b == nil {
			return errors.New("bucket is nil")
		}
		c := b.Cursor()
		for k, _ := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			if len(k) <= len(pkHash) || len(k) > len(pkHash)+8 {
				continue
			}
			kHeight := binary.BigEndian.Uint64(k[20:])
			if kHeight < deleteHeight {
				b.Delete(k)
			}
		}
		return nil
	})
	return err
}

// delete history asynchronous,this will find all account
func (stx *stateTX) deleteHistory() error {
	currentHeight := stx.ver + 1
	if currentHeight < stx.cfg.HistoryStateHeight {
		return nil
	}
	deleteHeight := currentHeight - stx.cfg.HistoryStateHeight
	go func() {
		stx.deleting <- struct{}{}
		db := stx.dao.DB()
		boltdb, ok := db.(*bolt.DB)
		if !ok {
			log.L().Error("convert to bolt db error")
			return
		}
		allPk := make([]hash.Hash160, 0)
		boltdb.View(func(tx *bolt.Tx) error {
			c := tx.Bucket([]byte(AccountKVNameSpace)).Cursor()
			for k, _ := c.Seek(AccountMaxVersionPrefix); bytes.HasPrefix(k, AccountMaxVersionPrefix); k, _ = c.Next() {
				addrHash := k[len(AccountMaxVersionPrefix):]
				h := hash.BytesToHash160(addrHash)
				allPk = append(allPk, h)
			}
			return nil
		})
		for _, h := range allPk {
			stx.deleteAccountHistory(h, deleteHeight)
		}
		<-stx.deleting
	}()
	return nil
}

// DelState deletes a state from DB
func (stx *stateTX) DelState(pkHash hash.Hash160) error {
	stx.cb.Delete(AccountKVNameSpace, pkHash[:], "error when deleting k = %x", pkHash)
	return nil
}
