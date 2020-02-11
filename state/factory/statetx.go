// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// stateTX implements stateTX interface, tracks pending changes to account/contract in local cache
type stateTX struct {
	flusher     db.KVStoreFlusher // the underlying DB for account/contract storage
	finalized   bool
	blockHeight uint64
}

// newStateTX creates a new state tx
func newStateTX(
	blockHeight uint64,
	kv db.KVStore,
	opts ...db.KVStoreFlusherOption,
) (*stateTX, error) {
	flusher, err := db.NewKVStoreFlusher(kv, batch.NewCachedBatch(), opts...)
	if err != nil {
		return nil, err
	}

	return &stateTX{
		flusher:     flusher,
		blockHeight: blockHeight,
		finalized:   false,
	}, nil
}

// RootHash returns the hash of the root node of the accountTrie
func (stx *stateTX) RootHash() (hash.Hash256, error) {
	if !stx.finalized {
		return hash.ZeroHash256, errors.New("workingset has not been finalized yet")
	}
	return hash.ZeroHash256, nil
}

// Digest returns the delta state digest
func (stx *stateTX) Digest() (hash.Hash256, error) {
	if !stx.finalized {
		return hash.ZeroHash256, errors.New("workingset has not been finalized yet")
	}

	return hash.Hash256b(stx.flusher.SerializeQueue()), nil
}

// Version returns the Version of this working set
func (stx *stateTX) Version() uint64 { return stx.blockHeight }

// Height returns the Height of the block being worked on
func (stx *stateTX) Height() (uint64, error) {
	return stx.blockHeight, nil
}

// RunActions runs actions in the block and track pending changes in working set
func (stx *stateTX) RunActions(
	ctx context.Context,
	elps []action.SealedEnvelope,
) ([]*action.Receipt, error) {
	// Handle actions
	receipts := make([]*action.Receipt, 0)
	for _, elp := range elps {
		receipt, err := stx.runAction(ctx, elp)
		if err != nil {
			return nil, errors.Wrap(err, "error when run action")
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
	}

	return receipts, nil
}

func (stx *stateTX) RunAction(ctx context.Context, elp action.SealedEnvelope) (*action.Receipt, error) {
	return stx.runAction(ctx, elp)
}

func (stx *stateTX) validateBlockHeight(blkCtx protocol.BlockCtx) error {
	if blkCtx.BlockHeight == stx.blockHeight {
		return nil
	}
	return errors.Errorf("invalid block height %d, %d expected", blkCtx.BlockHeight, stx.blockHeight)
}

func (stx *stateTX) runAction(
	ctx context.Context,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	if stx.finalized {
		return nil, errors.Errorf("cannot run action on a finalized working set")
	}

	// Handle action
	var actionCtx protocol.ActionCtx
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if err := stx.validateBlockHeight(blkCtx); err != nil {
		return nil, err
	}
	callerAddr, err := address.FromBytes(elp.SrcPubkey().Hash())
	if err != nil {
		return nil, err
	}
	actionCtx.Caller = callerAddr
	actionCtx.ActionHash = elp.Hash()
	actionCtx.GasPrice = elp.GasPrice()
	intrinsicGas, err := elp.IntrinsicGas()
	if err != nil {
		return nil, err
	}

	actionCtx.IntrinsicGas = intrinsicGas
	actionCtx.Nonce = elp.Nonce()
	if bcCtx.Registry == nil {
		return nil, nil
	}
	ctx = protocol.WithActionCtx(ctx, actionCtx)
	for _, actionHandler := range bcCtx.Registry.All() {
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

// Finalize runs action in the block and track pending changes in working set
func (stx *stateTX) Finalize() error {
	if stx.finalized {
		return errors.New("Cannot finalize a working set twice")
	}
	// Persist current chain Height
	stx.flusher.KVStoreWithBuffer().MustPut(
		AccountKVNamespace,
		[]byte(CurrentHeightKey),
		byteutil.Uint64ToBytes(stx.blockHeight),
	)
	stx.finalized = true

	return nil
}

func (stx *stateTX) Snapshot() int {
	return stx.flusher.KVStoreWithBuffer().Snapshot()
}

func (stx *stateTX) Revert(snapshot int) error {
	return stx.flusher.KVStoreWithBuffer().Revert(snapshot)
}

// Commit persists all changes in RunActions() into the DB
func (stx *stateTX) Commit() error {
	if !stx.finalized {
		return errors.New("cannot commit a working set which has not been finalized")
	}
	// Commit all changes in a batch
	dbBatchSizelMtc.WithLabelValues().Set(float64(stx.flusher.KVStoreWithBuffer().Size()))

	return stx.flusher.Flush()
}

// GetDB returns the underlying DB for account/contract storage
func (stx *stateTX) GetDB() db.KVStore {
	return stx.flusher.KVStoreWithBuffer()
}

// State pulls a state from DB
func (stx *stateTX) State(hash hash.Hash160, s interface{}, opts ...protocol.StateOption) error {
	stateDBMtc.WithLabelValues("get").Inc()
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return err
	}
	if cfg.AtHeight {
		return ErrNotSupported
	}
	ns := AccountKVNamespace
	if cfg.Namespace != "" {
		ns = cfg.Namespace
	}

	mstate, err := stx.flusher.KVStoreWithBuffer().Get(ns, hash[:])
	switch errors.Cause(err) {
	case db.ErrNotExist:
		return errors.Wrapf(state.ErrStateNotExist, "k = %x doesn't exist", hash)
	case nil:
		return state.Deserialize(s, mstate)
	}
	return errors.Wrapf(err, "failed to get account of %x", hash)
}

// PutState puts a state into DB
func (stx *stateTX) PutState(pkHash hash.Hash160, s interface{}, opts ...protocol.StateOption) error {
	stateDBMtc.WithLabelValues("put").Inc()
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return err
	}

	ns := AccountKVNamespace
	if cfg.Namespace != "" {
		ns = cfg.Namespace
	}

	ss, err := state.Serialize(s)
	if err != nil {
		return errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}

	stx.flusher.KVStoreWithBuffer().MustPut(ns, pkHash[:], ss)

	return nil
}

// DelState deletes a state from DB
func (stx *stateTX) DelState(pkHash hash.Hash160, opts ...protocol.StateOption) error {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return err
	}

	ns := AccountKVNamespace
	if cfg.Namespace != "" {
		ns = cfg.Namespace
	}
	stx.flusher.KVStoreWithBuffer().MustDelete(ns, pkHash[:])

	return nil
}
