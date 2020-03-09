// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
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
)

func init() {
	prometheus.MustRegister(stateDBMtc)
}

type (
	workingSet struct {
		height       uint64
		finalized    bool
		commitFunc   func(uint64) error
		dbFunc       func() db.KVStore
		delStateFunc func(string, []byte) error
		digestFunc   func() hash.Hash256
		finalizeFunc func(uint64) error
		getStateFunc func(string, []byte, interface{}) error
		putStateFunc func(string, []byte, interface{}) error
		revertFunc   func(int) error
		snapshotFunc func() int
	}

	workingSetCreator interface {
		newWorkingSet(context.Context, uint64) (*workingSet, error)
	}
)

// Digest returns the delta state digest
func (ws *workingSet) Digest() (hash.Hash256, error) {
	if !ws.finalized {
		return hash.ZeroHash256, errors.New("workingset has not been finalized yet")
	}
	return ws.digestFunc(), nil
}

// Version returns the Version of this working set
func (ws *workingSet) Version() uint64 {
	return ws.height
}

// Height returns the Height of the block being worked on
func (ws *workingSet) Height() (uint64, error) {
	return ws.height, nil
}

func (ws *workingSet) validate(ctx context.Context) error {
	if ws.finalized {
		return errors.Errorf("cannot run action on a finalized working set")
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if blkCtx.BlockHeight != ws.height {
		return errors.Errorf(
			"invalid block height %d, %d expected",
			blkCtx.BlockHeight,
			ws.height,
		)
	}
	return nil
}

// RunActions runs actions in the block and track pending changes in working set
func (ws *workingSet) RunActions(
	ctx context.Context,
	elps []action.SealedEnvelope,
) ([]*action.Receipt, error) {
	if err := ws.validate(ctx); err != nil {
		return nil, err
	}
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
	if err := ws.validate(ctx); err != nil {
		return nil, err
	}
	return ws.runAction(ctx, elp)
}

func (ws *workingSet) runAction(
	ctx context.Context,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	// Handle action
	var actionCtx protocol.ActionCtx
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
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
	if err := ws.finalizeFunc(ws.height); err != nil {
		return err
	}
	ws.finalized = true

	return nil
}

func (ws *workingSet) Snapshot() int {
	return ws.snapshotFunc()
}

func (ws *workingSet) Revert(snapshot int) error {
	return ws.revertFunc(snapshot)
}

// Commit persists all changes in RunActions() into the DB
func (ws *workingSet) Commit() error {
	return ws.commitFunc(ws.height)
}

// GetDB returns the underlying DB for account/contract storage
func (ws *workingSet) GetDB() db.KVStore {
	return ws.dbFunc()
}

func (ws *workingSet) processNonArchiveOptions(opts ...protocol.StateOption) (string, []byte, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return "", nil, err
	}
	if cfg.AtHeight && cfg.Height != ws.height {
		return cfg.Namespace, cfg.Key, ErrNotSupported
	}
	return cfg.Namespace, cfg.Key, nil
}

// State pulls a state from DB
func (ws *workingSet) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	stateDBMtc.WithLabelValues("get").Inc()
	ns, key, err := ws.processNonArchiveOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	return ws.height, ws.getStateFunc(ns, key, s)
}

func (ws *workingSet) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	return 0, nil, ErrNotSupported
}

// PutState puts a state into DB
func (ws *workingSet) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	stateDBMtc.WithLabelValues("put").Inc()
	ns, key, err := ws.processNonArchiveOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	return ws.height, ws.putStateFunc(ns, key, s)
}

// DelState deletes a state from DB
func (ws *workingSet) DelState(opts ...protocol.StateOption) (uint64, error) {
	stateDBMtc.WithLabelValues("delete").Inc()
	ns, key, err := ws.processNonArchiveOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	return ws.height, ws.delStateFunc(ns, key)
}
