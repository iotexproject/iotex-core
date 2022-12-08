// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package minifactory

import (
	"context"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	workingSet struct {
		height    uint64
		store     workingSetStore
		finalized bool
		dock      protocol.Dock
		receipts  []*action.Receipt
	}
)

func newWorkingSet(height uint64, store workingSetStore) *workingSet {
	return &workingSet{
		height: height,
		store:  store,
		dock:   protocol.NewDock(),
	}
}

func (ws *workingSet) digest() (hash.Hash256, error) {
	if !ws.finalized {
		return hash.ZeroHash256, errors.New("workingset has not been finalized yet")
	}
	return ws.store.Digest(), nil
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
	// Handle actions
	receipts := make([]*action.Receipt, 0)
	for _, elp := range elps {
		ctxWithActionContext, err := withActionCtx(ctx, elp)
		if err != nil {
			return nil, err
		}
		receipt, err := ws.runAction(ctxWithActionContext, elp)
		if err != nil {
			return nil, errors.Wrap(err, "error when run action")
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
	}
	if protocol.MustGetFeatureCtx(ctx).CorrectTxLogIndex {
		updateReceiptIndex(receipts)
	}
	return receipts, nil
}

func withActionCtx(ctx context.Context, selp action.SealedEnvelope) (context.Context, error) {
	var actionCtx protocol.ActionCtx
	var err error
	caller := selp.SenderAddress()
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
		return nil, action.ErrGasLimit
	}
	// Reject execution of chainID not equal the node's chainID
	if err := validateChainID(ctx, elp.ChainID()); err != nil {
		return nil, err
	}
	// Handle action
	reg, ok := protocol.GetRegistry(ctx)
	if !ok {
		return nil, nil
	}
	elpHash, err := elp.Hash()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get hash")
	}
	var receipt *action.Receipt
	for _, actionHandler := range reg.All() {
		receipt, err = actionHandler.Handle(ctx, elp.Action(), ws)
		if err != nil {
			err = errors.Wrapf(
				err,
				"error when action %x mutates states",
				elpHash,
			)
		}
		if receipt != nil || err != nil {
			break
		}
	}
	ws.ResetSnapshots()

	// TODO (zhi): return error if both receipt and err are nil
	return receipt, err
}

func validateChainID(ctx context.Context, chainID uint32) error {
	blkChainCtx := protocol.MustGetBlockchainCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.AllowCorrectDefaultChainID && (chainID != blkChainCtx.ChainID && chainID != 0) {
		return errors.Wrapf(action.ErrChainID, "expecting %d, got %d", blkChainCtx.ChainID, chainID)
	}
	return nil
}

func (ws *workingSet) finalize() error {
	if ws.finalized {
		return errors.New("Cannot finalize a working set twice")
	}
	if err := ws.store.Finalize(ws.height); err != nil {
		return err
	}
	ws.finalized = true

	return nil
}

func (ws *workingSet) Snapshot() int {
	return ws.store.Snapshot()
}

func (ws *workingSet) Revert(snapshot int) error {
	return ws.store.RevertSnapshot(snapshot)
}

func (ws *workingSet) ResetSnapshots() {
	ws.store.ResetSnapshots()
}

// Commit persists all changes in RunActions() into the DB
func (ws *workingSet) Commit(ctx context.Context) error {
	if err := protocolPreCommit(ctx, ws); err != nil {
		return err
	}
	if err := ws.store.Commit(); err != nil {
		return err
	}
	if err := protocolCommit(ctx, ws); err != nil {
		// TODO (zhi): wrap the error and eventually panic it in caller side
		return err
	}
	ws.Reset()
	return nil
}

// State pulls a state from DB
func (ws *workingSet) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	value, err := ws.store.Get(cfg.Namespace, cfg.Key)
	if err != nil {
		return ws.height, err
	}
	return ws.height, state.Deserialize(s, value)
}

func (ws *workingSet) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, nil, err
	}
	if cfg.Key != nil {
		return 0, nil, errors.Wrap(factory.ErrNotSupported, "Read states with key option has not been implemented yet")
	}
	values, err := ws.store.States(cfg.Namespace, cfg.Keys)
	if err != nil {
		return 0, nil, err
	}
	return ws.height, state.NewIterator(values), nil
}

// PutState puts a state into DB
func (ws *workingSet) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	ss, err := state.Serialize(s)
	if err != nil {
		return ws.height, errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}
	return ws.height, ws.store.Put(cfg.Namespace, cfg.Key, ss)
}

// DelState deletes a state from DB
func (ws *workingSet) DelState(opts ...protocol.StateOption) (uint64, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	return ws.height, ws.store.Delete(cfg.Namespace, cfg.Key)
}

// ReadView reads the view
func (ws *workingSet) ReadView(name string) (interface{}, error) {
	return ws.store.ReadView(name)
}

// WriteView writeback the view to factory
func (ws *workingSet) WriteView(name string, v interface{}) error {
	return ws.store.WriteView(name, v)
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

func (ws *workingSet) validateNonce(ctx context.Context, blk *block.Block) error {
	accountNonceMap := make(map[string][]uint64)
	for _, selp := range blk.Actions {
		caller := selp.SenderAddress()
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
		addr, _ := address.FromString(srcAddr)
		confirmedState, err := accountutil.AccountState(ctx, ws, addr)
		if err != nil {
			return errors.Wrapf(err, "failed to get the confirmed nonce of address %s", srcAddr)
		}
		receivedNonces := receivedNonces
		sort.Slice(receivedNonces, func(i, j int) bool { return receivedNonces[i] < receivedNonces[j] })
		pendingNonce := confirmedState.PendingNonce()
		for i, nonce := range receivedNonces {
			if nonce != pendingNonce+uint64(i) {
				return errors.Wrapf(
					action.ErrNonceTooHigh,
					"the %d nonce %d of address %s (init pending nonce %d) is not continuously increasing",
					i,
					nonce,
					srcAddr,
					pendingNonce,
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
	if err := ws.validate(ctx); err != nil {
		return err
	}

	reg := protocol.MustGetRegistry(ctx)
	for _, act := range actions {
		ctxWithActionContext, err := withActionCtx(ctx, act)
		if err != nil {
			return err
		}
		for _, p := range reg.All() {
			if validator, ok := p.(protocol.ActionValidator); ok {
				if err := validator.Validate(ctxWithActionContext, act.Action(), ws); err != nil {
					return err
				}
			}
		}
	}
	for _, p := range reg.All() {
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

func updateReceiptIndex(receipts []*action.Receipt) {
	var txIndex, logIndex uint32
	for _, r := range receipts {
		logIndex = r.UpdateIndex(txIndex, logIndex)
		txIndex++
	}
}

func (ws *workingSet) ValidateBlock(ctx context.Context, blk *block.Block) error {
	if err := ws.validateNonce(ctx, blk); err != nil {
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
	if !blk.VerifyDeltaStateDigest(digest) {
		return errors.Wrapf(block.ErrDeltaStateMismatch, "digest in block '%x' vs digest in workingset '%x'", blk.DeltaStateDigest(), digest)
	}
	receiptRoot := calculateReceiptRoot(ws.receipts)
	if !blk.VerifyReceiptRoot(receiptRoot) {
		return errors.Wrapf(block.ErrReceiptRootMismatch, "receipt root in block '%x' vs receipt root in workingset '%x'", blk.ReceiptRoot(), receiptRoot)
	}

	return nil
}
