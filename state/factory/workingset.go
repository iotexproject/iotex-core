// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
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

func (ws *workingSet) digest() (hash.Hash256, error) {
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

func (ws *workingSet) runActions(
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

func (ws *workingSet) runAction(
	ctx context.Context,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	// Handle action
	var actionCtx protocol.ActionCtx
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
	reg, ok := protocol.GetRegistry(ctx)
	if !ok {
		return nil, nil
	}
	for _, actionHandler := range reg.All() {
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

func (ws *workingSet) finalize() error {
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

// State pulls a state from DB
func (ws *workingSet) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	stateDBMtc.WithLabelValues("get").Inc()
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	return ws.height, ws.getStateFunc(cfg.Namespace, cfg.Key, s)
}

func (ws *workingSet) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	return 0, nil, ErrNotSupported
}

// PutState puts a state into DB
func (ws *workingSet) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	stateDBMtc.WithLabelValues("put").Inc()
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	return ws.height, ws.putStateFunc(cfg.Namespace, cfg.Key, s)
}

// DelState deletes a state from DB
func (ws *workingSet) DelState(opts ...protocol.StateOption) (uint64, error) {
	stateDBMtc.WithLabelValues("delete").Inc()
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	return ws.height, ws.delStateFunc(cfg.Namespace, cfg.Key)
}

// createGenesisStates initialize the genesis states
func (ws *workingSet) CreateGenesisStates(ctx context.Context) error {
	if reg, ok := protocol.GetRegistry(ctx); ok {
		for _, p := range reg.All() {
			if gsc, ok := p.(protocol.GenesisStateCreator); ok {
				if err := gsc.CreateGenesisStates(ctx, ws); err != nil {
					return errors.Wrap(err, "failed to create genesis states for protocol")
				}
			}
		}
	}

	return ws.finalize()
}

func (ws *workingSet) validateNonce(blk *block.Block) error {
	accountNonceMap := make(map[string][]uint64)
	for _, selp := range blk.Actions {
		caller, err := address.FromBytes(selp.SrcPubkey().Hash())
		if err != nil {
			return err
		}
		appendActionIndex(accountNonceMap, caller.String(), selp.Nonce())
	}

	// Special handling for genesis block
	if blk.Height() == 0 {
		return nil
	}
	// Verify each account's Nonce
	for srcAddr, receivedNonces := range accountNonceMap {
		confirmedState, err := accountutil.AccountState(ws, srcAddr)
		if err != nil {
			return errors.Wrapf(err, "failed to get the confirmed nonce of address %s", srcAddr)
		}
		receivedNonces := receivedNonces
		sort.Slice(receivedNonces, func(i, j int) bool { return receivedNonces[i] < receivedNonces[j] })
		for i, nonce := range receivedNonces {
			if nonce != confirmedState.Nonce+uint64(i+1) {
				return errors.Wrapf(
					action.ErrNonce,
					"the %d nonce %d of address %s (confirmed nonce %d) is not continuously increasing",
					i,
					nonce,
					srcAddr,
					confirmedState.Nonce,
				)
			}
		}
	}
	return nil
}

func (ws *workingSet) Process(ctx context.Context, actions []action.SealedEnvelope) ([]*action.Receipt, error) {
	return ws.process(ctx, actions)
}

func (ws *workingSet) process(ctx context.Context, actions []action.SealedEnvelope) ([]*action.Receipt, error) {
	for _, p := range protocol.MustGetRegistry(ctx).All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return nil, err
			}
		}
	}
	// TODO: verify whether the post system actions are appended tail

	receipts, err := ws.runActions(ctx, actions)
	if err != nil {
		return nil, err
	}
	return receipts, ws.finalize()
}

func (ws *workingSet) pickAndRunActions(
	ctx context.Context,
	actionMap map[string][]action.SealedEnvelope,
	postSystemActions []action.SealedEnvelope,
	allowedBlockGasResidue uint64,
) ([]*action.Receipt, []action.SealedEnvelope, error) {
	if err := ws.validate(ctx); err != nil {
		return nil, nil, err
	}
	receipts := make([]*action.Receipt, 0)
	executedActions := make([]action.SealedEnvelope, 0)

	blkCtx := protocol.MustGetBlockCtx(ctx)
	for _, p := range protocol.MustGetRegistry(ctx).All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return nil, nil, err
			}
		}
	}

	// initial action iterator
	actionIterator := actioniterator.NewActionIterator(actionMap)
	for {
		nextAction, ok := actionIterator.Next()
		if !ok {
			break
		}
		receipt, err := ws.runAction(ctx, nextAction)
		if err != nil {
			if errors.Cause(err) == action.ErrHitGasLimit {
				// hit block gas limit, we should not process actions belong to this user anymore since we
				// need monotonically increasing nonce. But we can continue processing other actions
				// that belong other users
				actionIterator.PopAccount()
				continue
			}
			return nil, nil, errors.Wrapf(err, "Failed to update state changes for selp %x", nextAction.Hash())
		}
		if receipt != nil {
			blkCtx.GasLimit -= receipt.GasConsumed
			ctx = protocol.WithBlockCtx(ctx, blkCtx)
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, nextAction)

		// To prevent loop all actions in act_pool, we stop processing action when remaining gas is below
		// than certain threshold
		if blkCtx.GasLimit < allowedBlockGasResidue {
			break
		}
	}
	for _, selp := range postSystemActions {
		receipt, err := ws.runAction(ctx, selp)
		if err != nil {
			return nil, nil, err
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, selp)
	}

	return receipts, executedActions, ws.finalize()
}

func (ws *workingSet) ValidateBlock(ctx context.Context, blk *block.Block) error {
	if err := ws.validateNonce(blk); err != nil {
		return errors.Wrap(err, "failed to validate nonce")
	}
	receipts, err := ws.process(ctx, blk.RunnableActions().Actions())
	if err != nil {
		log.L().Panic("Failed to update state.", zap.Uint64("height", ws.height), zap.Error(err))
	}

	digest, err := ws.digest()
	if err != nil {
		return err
	}
	if err = blk.VerifyDeltaStateDigest(digest); err != nil {
		return errors.Wrap(err, "failed to verify delta state digest")
	}
	if err = blk.VerifyReceiptRoot(calculateReceiptRoot(receipts)); err != nil {
		return errors.Wrap(err, "Failed to verify receipt root")
	}

	blk.Receipts = receipts
	return nil
}

func (ws *workingSet) CreateBuilder(
	ctx context.Context,
	actionMap map[string][]action.SealedEnvelope,
	postSystemActions []action.SealedEnvelope,
	allowedBlockGasResidue uint64,
) (*block.Builder, error) {
	rc, actions, err := ws.pickAndRunActions(ctx, actionMap, postSystemActions, allowedBlockGasResidue)
	if err != nil {
		return nil, err
	}

	ra := block.NewRunnableActionsBuilder().
		AddActions(actions...).
		Build()

	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	prevBlkHash := bcCtx.Tip.Hash
	// The first block's previous block hash is pointing to the digest of genesis config. This is to guarantee all nodes
	// could verify that they start from the same genesis
	if blkCtx.BlockHeight == 1 {
		prevBlkHash = bcCtx.Genesis.Hash()
	}
	digest, err := ws.digest()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get digest")
	}

	blkBuilder := block.NewBuilder(ra).
		SetHeight(blkCtx.BlockHeight).
		SetTimestamp(blkCtx.BlockTimeStamp).
		SetPrevBlockHash(prevBlkHash).
		SetDeltaStateDigest(digest).
		SetReceipts(rc).
		SetReceiptRoot(calculateReceiptRoot(rc)).
		SetLogsBloom(calculateLogsBloom(ctx, rc))
	return blkBuilder, nil
}
