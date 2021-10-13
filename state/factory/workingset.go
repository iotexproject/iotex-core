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
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
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
		height        uint64
		finalized     bool
		dock          protocol.Dock
		receipts      []*action.Receipt
		commitFunc    func(uint64) error
		readviewFunc  func(name string) (interface{}, error)
		writeviewFunc func(name string, v interface{}) error
		dbFunc        func() db.KVStore
		delStateFunc  func(string, []byte) error
		statesFunc    func(opts ...protocol.StateOption) (uint64, state.Iterator, error)
		digestFunc    func() hash.Hash256
		finalizeFunc  func(uint64) error
		getStateFunc  func(string, []byte, interface{}) error
		putStateFunc  func(string, []byte, interface{}) error
		revertFunc    func(int) error
		snapshotFunc  func() int
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

func (ws *workingSet) Receipts() ([]*action.Receipt, error) {
	if !ws.finalized {
		return nil, errors.New("workingset has not been finalized yet")
	}
	return ws.receipts, nil
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
		ctx, err := withActionCtx(ctx, elp)
		if err != nil {
			return nil, err
		}
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

func withActionCtx(ctx context.Context, selp action.SealedEnvelope) (context.Context, error) {
	var actionCtx protocol.ActionCtx
	var err error
	caller := selp.SrcPubkey().Address()
	if caller == nil {
		return nil, errors.New("failed to get address")
	}
	actionCtx.Caller = caller
	actionCtx.ActionHash, err = selp.Hash()
	if err != nil {
		return nil, err
	}
	actionCtx.GasPrice = selp.GasPrice()
	intrinsicGas, err := selp.IntrinsicGas()
	if err != nil {
		return nil, err
	}
	actionCtx.IntrinsicGas = intrinsicGas
	actionCtx.Nonce = selp.Nonce()

	return protocol.WithActionCtx(ctx, actionCtx), nil
}

func (ws *workingSet) runAction(
	ctx context.Context,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	if protocol.MustGetBlockCtx(ctx).GasLimit < protocol.MustGetActionCtx(ctx).IntrinsicGas {
		return nil, errors.Wrap(action.ErrHitGasLimit, "block gas limit exceeded")
	}
	// Reject execution of chainID not equal the node's chainID
	blkChainCtx := protocol.MustGetBlockchainCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	if g.IsKamchatka(ws.height) {
		if elp.ChainID() != blkChainCtx.ChainID {
			return nil, errors.Wrapf(action.ErrChainID, "expecting %d, got %d", blkChainCtx.ChainID, elp.ChainID())
		}
	}

	// Handle action
	reg, ok := protocol.GetRegistry(ctx)
	if !ok {
		return nil, nil
	}
	for _, actionHandler := range reg.All() {
		receipt, err := actionHandler.Handle(ctx, elp.Action(), ws)
		elpHash, err1 := elp.Hash()
		if err1 != nil {
			return nil, errors.Wrapf(err1, "Failed to get hash")
		}
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"error when action %x mutates states",
				elpHash,
			)
		}
		if receipt != nil {
			return receipt, nil
		}
	}
	// TODO (zhi): return error
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
func (ws *workingSet) Commit(ctx context.Context) error {
	if err := ws.commitFunc(ws.height); err != nil {
		return err
	}
	if err := protocolCommit(ctx, ws); err != nil {
		return err
	}
	ws.Reset()
	return nil
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
	return ws.statesFunc(opts...)
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

// ReadView reads the view
func (ws *workingSet) ReadView(name string) (interface{}, error) {
	return ws.readviewFunc(name)
}

// WriteView writeback the view to factory
func (ws *workingSet) WriteView(name string, v interface{}) error {
	return ws.writeviewFunc(name, v)
}

func (ws *workingSet) ProtocolDirty(name string) bool {
	return ws.dock.ProtocolDirty(name)
}

func (ws *workingSet) Load(name, key string, v interface{}) error {
	return ws.dock.Load(name, key, v)
}

func (ws *workingSet) Unload(name, key string, v interface{}) error {
	return ws.dock.Unload(name, key, v)
}

func (ws *workingSet) Reset() {
	ws.dock.Reset()
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
		caller := selp.SrcPubkey().Address()
		if caller == nil {
			return errors.New("failed to get address")
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

func (ws *workingSet) Process(ctx context.Context, actions []action.SealedEnvelope) error {
	return ws.process(ctx, actions)
}

func (ws *workingSet) process(ctx context.Context, actions []action.SealedEnvelope) error {
	var err error
	reg := protocol.MustGetRegistry(ctx)
	for _, act := range actions {
		if ctx, err = withActionCtx(ctx, act); err != nil {
			return err
		}
		for _, p := range reg.All() {
			if validator, ok := p.(protocol.ActionValidator); ok {
				if err := validator.Validate(ctx, act.Action(), ws); err != nil {
					return err
				}
			}
		}
	}
	for _, p := range protocol.MustGetRegistry(ctx).All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return err
			}
		}
	}
	// TODO: verify whether the post system actions are appended tail

	receipts, err := ws.runActions(ctx, actions)
	if err != nil {
		return err
	}
	ws.receipts = receipts
	return ws.finalize()
}

func (ws *workingSet) pickAndRunActions(
	ctx context.Context,
	ap actpool.ActPool,
	postSystemActions []action.SealedEnvelope,
	allowedBlockGasResidue uint64,
) ([]action.SealedEnvelope, error) {
	err := ws.validate(ctx)
	if err != nil {
		return nil, err
	}
	receipts := make([]*action.Receipt, 0)
	executedActions := make([]action.SealedEnvelope, 0)
	reg := protocol.MustGetRegistry(ctx)

	for _, p := range reg.All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return nil, err
			}
		}
	}

	// initial action iterator
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if ap != nil {
		actionIterator := actioniterator.NewActionIterator(ap.PendingActionMap())
		// To prevent loop all actions in act_pool, we stop processing action when remaining gas is below
		// than certain threshold
		for blkCtx.GasLimit >= allowedBlockGasResidue {
			nextAction, ok := actionIterator.Next()
			if !ok {
				break
			}
			if nextAction.GasLimit() > blkCtx.GasLimit {
				actionIterator.PopAccount()
				continue
			}
			if ctx, err = withActionCtx(ctx, nextAction); err == nil {
				for _, p := range reg.All() {
					if validator, ok := p.(protocol.ActionValidator); ok {
						if err = validator.Validate(ctx, nextAction.Action(), ws); err != nil {
							break
						}
					}
				}
			}
			if err != nil {
				caller := nextAction.SrcPubkey().Address()
				if caller == nil {
					return nil, errors.New("failed to get address")
				}
				ap.DeleteAction(caller)
				actionIterator.PopAccount()
				continue
			}
			receipt, err := ws.runAction(ctx, nextAction)
			switch errors.Cause(err) {
			case nil:
				// do nothing
			case action.ErrChainID:
				continue
			case action.ErrHitGasLimit:
				actionIterator.PopAccount()
				continue
			default:
				nextActionHash, err := nextAction.Hash()
				if err != nil {
					return nil, errors.Wrapf(err, "Failed to get hash for %x", nextActionHash)
				}
				return nil, errors.Wrapf(err, "Failed to update state changes for selp %x", nextActionHash)
			}
			if receipt != nil {
				blkCtx.GasLimit -= receipt.GasConsumed
				ctx = protocol.WithBlockCtx(ctx, blkCtx)
				receipts = append(receipts, receipt)
			}
			executedActions = append(executedActions, nextAction)
		}
	}

	for _, selp := range postSystemActions {
		if ctx, err = withActionCtx(ctx, selp); err != nil {
			return nil, err
		}
		receipt, err := ws.runAction(ctx, selp)
		if err != nil {
			return nil, err
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, selp)
	}
	ws.receipts = receipts

	return executedActions, ws.finalize()
}

func (ws *workingSet) ValidateBlock(ctx context.Context, blk *block.Block) error {
	if err := ws.validateNonce(blk); err != nil {
		return errors.Wrap(err, "failed to validate nonce")
	}
	if err := ws.process(ctx, blk.RunnableActions().Actions()); err != nil {
		log.L().Error("Failed to update state.", zap.Uint64("height", ws.height), zap.Error(err))
		return err
	}

	digest, err := ws.digest()
	if err != nil {
		return err
	}
	if err = blk.VerifyDeltaStateDigest(digest); err != nil {
		return errors.Wrap(err, "failed to verify delta state digest")
	}
	if err = blk.VerifyReceiptRoot(calculateReceiptRoot(ws.receipts)); err != nil {
		return errors.Wrap(err, "Failed to verify receipt root")
	}

	return nil
}

func (ws *workingSet) CreateBuilder(
	ctx context.Context,
	ap actpool.ActPool,
	postSystemActions []action.SealedEnvelope,
	allowedBlockGasResidue uint64,
) (*block.Builder, error) {
	actions, err := ws.pickAndRunActions(ctx, ap, postSystemActions, allowedBlockGasResidue)
	if err != nil {
		return nil, err
	}

	ra := block.NewRunnableActionsBuilder().
		AddActions(actions...).
		Build()

	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	prevBlkHash := bcCtx.Tip.Hash
	digest, err := ws.digest()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get digest")
	}

	blkBuilder := block.NewBuilder(ra).
		SetHeight(blkCtx.BlockHeight).
		SetTimestamp(blkCtx.BlockTimeStamp).
		SetPrevBlockHash(prevBlkHash).
		SetDeltaStateDigest(digest).
		SetReceipts(ws.receipts).
		SetReceiptRoot(calculateReceiptRoot(ws.receipts)).
		SetLogsBloom(calculateLogsBloom(ctx, ws.receipts))
	return blkBuilder, nil
}
