// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"encoding/hex"
	"fmt"
	"maps"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"slices"
	"time"

	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/state/factory/erigonstore"
)

var (
	_stateDBMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_state_db",
			Help: "IoTeX State DB",
		},
		[]string{"type"},
	)
	_mintAbility = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_mint_ability",
			Help: "IoTeX Mint Ability",
		},
		[]string{"type"},
	)
	_mintSkipMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_mint_skipped_actions_total",
			Help: "Number of actions left out of a draft block while minting, by reason and action type",
		},
		[]string{"reason", "action"},
	)

	errInvalidSystemActionLayout = errors.New("system action layout is invalid")
	errDeployerNotWhitelisted    = errors.New("deployer not whitelisted")
	// errMintDeadline tells the mint loop to stop picking actions and seal
	// the draft, because the deadline passed while an action was running
	errMintDeadline = errors.New("mint deadline passed while running an action")
	// errMintActionPanic marks an action that panicked while running during mint
	errMintActionPanic = errors.New("recovered from panic while running action")
)

// _mintDropLogLimit caps the actions a draft logs in detail when it drops
// them for failing to run, so that a pool full of failing actions cannot
// flood the log; the rest are counted and summed up once the draft is done.
const _mintDropLogLimit = 16

func init() {
	prometheus.MustRegister(_stateDBMtc)
	prometheus.MustRegister(_mintAbility)
	prometheus.MustRegister(_mintSkipMtc)
}

// deductBlockGas takes the gas reported by a receipt out of the gas a block has
// left. The counter is unsigned, so before the fix height the subtraction wraps
// around when a receipt reports more gas than remains; from that height on it
// saturates at zero, which keeps the remaining budget monotonic and makes the
// next action in the block run out of gas instead of being waved through.
func deductBlockGas(fCtx protocol.FeatureCtx, remaining, consumed uint64) uint64 {
	if fCtx.CheckedBlockGasDeduction && remaining < consumed {
		log.L().Warn("receipt reports more gas than the block has left",
			zap.Uint64("remaining", remaining), zap.Uint64("consumed", consumed))
		return 0
	}
	return remaining - consumed
}

type (
	// WorkingSetStoreFactory is the factory to create working set store
	WorkingSetStoreFactory interface {
		CreateWorkingSetStore(context.Context, uint64, db.KVStore) (workingSetStore, error)
	}
	workingSet struct {
		workingSetStoreFactory WorkingSetStoreFactory
		height                 uint64
		views                  protocol.Views
		viewsSnapshots         map[int]int
		store                  workingSetStore
		finalized              bool
		txValidator            *protocol.GenericValidator
		receipts               []*action.Receipt
		captureStateDiff       bool              // capture the write queue for a state diff callback
		stateDiffEntries       []WriteQueueEntry // captured write queue for state diff broadcasting
		stateDiffDigest        []byte            // cached digest bytes for state diff callback
		mintDrops              int               // actions dropped from the draft for failing to run
		mintOwnerOrOpChanged   bool              // a candidate's owner or operator changed in this block (mint only)
		mintDeferred           map[string]bool   // senders held back to the next block (mint only)
	}
)

func newWorkingSet(height uint64, views protocol.Views, store workingSetStore, storeFactory WorkingSetStoreFactory) *workingSet {
	ws := &workingSet{
		height:                 height,
		views:                  views,
		viewsSnapshots:         make(map[int]int),
		mintDeferred:           make(map[string]bool),
		store:                  store,
		workingSetStoreFactory: storeFactory,
	}
	// Set here so every path, including a working set derived from an
	// uncommitted parent, follows the factory's current callback state.
	if sdb, ok := storeFactory.(*stateDB); ok {
		ws.captureStateDiff = sdb.diffCallback.Load() != nil
	}
	ws.txValidator = protocol.NewGenericValidator(ws, accountutil.AccountState)
	return ws
}

// dumpWriteQueue writes the ordered state-write queue for the given height to
// $IOTEX_DIGEST_DUMP_DIR, so the same block can be replayed under two binaries
// and the dumps diffed; the first differing line is the point of divergence.
// No-op unless the env var is set, so this costs nothing on a normal node.
func (ws *workingSet) dumpWriteQueue(height uint64) {
	dir := os.Getenv("IOTEX_DIGEST_DUMP_DIR")
	if dir == "" {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("digest-%d.txt", height))
	f, err := os.Create(path)
	if err != nil {
		log.L().Error("failed to create digest dump file", zap.String("path", path), zap.Error(err))
		return
	}
	defer f.Close()
	if err := ws.store.DumpWriteQueue(f); err != nil {
		log.L().Error("failed to dump write queue", zap.Uint64("height", height), zap.Error(err))
		return
	}
	log.L().Info("dumped state write queue for digest mismatch",
		zap.Uint64("height", height), zap.String("path", path))
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

func withActionCtx(ctx context.Context, selp *action.SealedEnvelope) (context.Context, error) {
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
	selp *action.SealedEnvelope,
	revertAllSnapshots bool,
) (*action.Receipt, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	if protocol.MustGetBlockCtx(ctx).GasLimit < actCtx.IntrinsicGas {
		return nil, action.ErrGasLimit
	}
	// Reject execution of chainID not equal the node's chainID
	if !action.IsSystemAction(selp) {
		if err := validateChainID(ctx, selp.ChainID()); err != nil {
			return nil, err
		}
	}
	// verify the tx is not container format (unfolded correctly)
	if selp.Encoding() == uint32(iotextypes.Encoding_TX_CONTAINER) {
		return nil, errors.Wrap(action.ErrInvalidAct, "cannot run tx container without unfolding")
	}
	// for replay tx, check against deployer whitelist
	g := genesis.MustExtractGenesisContext(ctx)
	if !selp.Protected() && !g.IsDeployerWhitelisted(selp.SenderAddress()) {
		return nil, errors.Wrap(errDeployerNotWhitelisted, selp.SenderAddress().String())
	}
	// Handle action
	reg, ok := protocol.GetRegistry(ctx)
	if !ok {
		return nil, errors.New("protocol is empty")
	}
	selpHash, err := selp.Hash()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get hash")
	}
	// Finalize the tx only once it has run through: erigon drops its revert
	// journal on FinalizeTx, so a failed action that mint is about to revert
	// must not be finalized first.
	succeeded := false
	defer func() {
		if succeeded {
			ws.finalizeTx(ctx)
		}
	}()
	if revertAllSnapshots {
		defer ws.store.ResetSnapshots()
	}
	if err := ws.freshAccountConversion(ctx, &actCtx); err != nil {
		return nil, err
	}
	fCtx := protocol.MustGetFeatureCtx(ctx)
	var receipt *action.Receipt
	// System actions (e.g. GrantReward) are implementation details and must not
	// appear in block-level traces (debug_traceBlock*). Skip TraceStart/TraceEnd
	// for them so CaptureTx is never called on their behalf.
	isSystemAct := action.IsSystemAction(selp)
	var traceErr error
	if !isSystemAct {
		traceErr = evm.TraceStart(ctx, ws, selp.Envelope)
		if traceErr != nil {
			log.L().Error("failed to start tracing EVM execution", zap.Error(traceErr))
		}
	}
	for _, actionHandler := range reg.All() {
		receipt, err = actionHandler.Handle(ctx, selp.Envelope, ws)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"error when action %x mutates states",
				selpHash,
			)
		}
		if receipt != nil {
			break
		}
	}
	if receipt == nil {
		return nil, errors.New("receipt is empty")
	}
	if !isSystemAct && traceErr == nil {
		evm.TraceEnd(ctx, receipt)
	}
	if fCtx.EnableBlobTransaction && len(selp.BlobHashes()) > 0 {
		if err = ws.handleBlob(ctx, selp, receipt); err != nil {
			return nil, err
		}
	}
	for _, p := range reg.All() {
		if pp, ok := p.(protocol.PostActionHandler); ok {
			if err := pp.HandleReceipt(ctx, selp.Envelope, ws, receipt); err != nil {
				return nil, errors.Wrapf(err, "error when handle action %x receipt", selpHash)
			}
		}
	}
	succeeded = true
	return receipt, nil
}

func (ws *workingSet) handleBlob(ctx context.Context, act *action.SealedEnvelope, receipt *action.Receipt) error {
	// Deposit blob fee
	receipt.BlobGasUsed = act.BlobGas()
	receipt.BlobGasPrice = protocol.CalcBlobFee(protocol.MustGetBlockchainCtx(ctx).Tip.ExcessBlobGas)
	blobFee := new(big.Int).Mul(receipt.BlobGasPrice, new(big.Int).SetUint64(receipt.BlobGasUsed))
	logs, err := rewarding.DepositGas(ctx, ws, new(big.Int), protocol.BlobGasFeeOption(blobFee))
	if err != nil {
		return err
	}
	receipt.AddTransactionLogs(logs...)
	return nil
}

func validateChainID(ctx context.Context, chainID uint32) error {
	blkChainCtx := protocol.MustGetBlockchainCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.AllowCorrectChainIDOnly && chainID != blkChainCtx.ChainID {
		return errors.Wrapf(action.ErrChainID, "expecting %d, got %d", blkChainCtx.ChainID, chainID)
	}
	if featureCtx.AllowCorrectDefaultChainID && (chainID != blkChainCtx.ChainID && chainID != 0) {
		return errors.Wrapf(action.ErrChainID, "expecting %d, got %d", blkChainCtx.ChainID, chainID)
	}
	return nil
}

func (ws *workingSet) checkContract(ctx context.Context, to *common.Address) (bool, bool, bool, error) {
	if to == nil {
		return true, false, false, nil
	}
	var (
		addr, _ = address.FromBytes(to.Bytes())
		ioAddr  = addr.String()
	)
	if ioAddr == address.StakingProtocolAddr {
		return false, true, false, nil
	}
	if ioAddr == address.RewardingProtocol {
		return false, false, true, nil
	}
	sender, err := accountutil.AccountState(ctx, ws, addr)
	if err != nil {
		return false, false, false, errors.Wrapf(err, "failed to get account of %s", to.Hex())
	}
	return sender.IsContract(), false, false, nil
}

func (ws *workingSet) finalize(ctx context.Context) error {
	if ws.finalized {
		return errors.New("Cannot finalize a working set twice")
	}
	if err := ws.store.Finalize(ctx); err != nil {
		return err
	}
	// Capture write queue entries and digest for state diff broadcasting.
	// Must happen after Finalize (which writes height) but before Commit (which flushes).
	if sdbStore := ws.getStateDBStore(); ws.captureStateDiff && sdbStore != nil {
		ws.stateDiffEntries = sdbStore.CaptureWriteQueue()
		d := sdbStore.Digest()
		ws.stateDiffDigest = d[:]
	}
	ws.finalized = true

	return nil
}

// getStateDBStore extracts the underlying *stateDBWorkingSetStore,
// handling both direct and wrapped (workingSetStoreWithSecondary) cases.
func (ws *workingSet) getStateDBStore() *stateDBWorkingSetStore {
	if s, ok := ws.store.(*stateDBWorkingSetStore); ok {
		return s
	}
	if s, ok := ws.store.(*workingSetStoreWithSecondary); ok {
		if inner, ok := s.writer.(*stateDBWorkingSetStore); ok {
			return inner
		}
	}
	return nil
}

func (ws *workingSet) finalizeTx(ctx context.Context) {
	if err := ws.store.FinalizeTx(ctx); err != nil {
		log.L().Panic("failed to finalize tx", zap.Error(err))
	}
}

func (ws *workingSet) Snapshot() int {
	id := ws.store.Snapshot()
	vid := ws.views.Snapshot()
	ws.viewsSnapshots[id] = vid

	return id
}

func (ws *workingSet) Revert(snapshot int) error {
	vid, ok := ws.viewsSnapshots[snapshot]
	if !ok {
		return errors.Errorf("snapshot %d not found", snapshot)
	}
	if err := ws.views.Revert(vid); err != nil {
		return errors.Wrapf(err, "failed to revert views to snapshot %d", vid)
	}
	return ws.store.RevertSnapshot(snapshot)
}

// freshAccountConversion happens between UseZeroNonceForFreshAccount height
// and RefactorFreshAccountConversion height
func (ws *workingSet) freshAccountConversion(ctx context.Context, actCtx *protocol.ActionCtx) error {
	// check legacy fresh account conversion
	fCtx := protocol.MustGetFeatureCtx(ctx)
	if fCtx.UseZeroNonceForFreshAccount && !fCtx.RefactorFreshAccountConversion {
		sender, err := accountutil.AccountState(ctx, ws, actCtx.Caller)
		if err != nil {
			return errors.Wrapf(err, "failed to get the confirmed nonce of sender %s", actCtx.Caller.String())
		}
		if sender.ConvertFreshAccountToZeroNonceType(actCtx.Nonce) {
			if err = accountutil.StoreAccount(ws, actCtx.Caller, sender); err != nil {
				return errors.Wrapf(err, "failed to store converted sender %s", actCtx.Caller.String())
			}
		}
	}
	return nil
}

// Commit persists all changes in RunActions() into the DB
func (ws *workingSet) Commit(ctx context.Context, retention uint64) error {
	if err := protocolPreCommit(ctx, ws); err != nil {
		return err
	}
	if err := ws.store.Commit(ctx, retention); err != nil {
		return err
	}
	if err := protocolCommit(ctx, ws); err != nil {
		// TODO (zhi): wrap the error and eventually panic it in caller side
		return err
	}
	return nil
}

// State pulls a state from DB
func (ws *workingSet) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	_stateDBMtc.WithLabelValues("get").Inc()
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	if cfg.Keys != nil {
		return 0, errors.Wrap(ErrNotSupported, "Read state with keys option has not been implemented yet")
	}
	store, err := ws.matchStore(cfg)
	if err != nil {
		return 0, err
	}
	key, err := ws.matchStoreKey(store, cfg)
	if err != nil {
		return 0, err
	}
	return ws.height, store.GetObject(cfg.Namespace, key, s)
}

func (ws *workingSet) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, nil, err
	}
	if cfg.Key != nil {
		return 0, nil, errors.Wrap(ErrNotSupported, "Read states with key option has not been implemented yet")
	}
	// Keys and Range/Limit describe two different queries, so combining them has no
	// well-defined answer -- reject rather than silently pick one
	if err := validateStatesConfig(cfg); err != nil {
		return 0, nil, err
	}
	store, err := ws.matchStore(cfg)
	if err != nil {
		return 0, nil, err
	}
	// rangeScanFromConfig returns nil unless Range/Limit was explicitly requested,
	// which keeps every existing caller on the untouched legacy path
	iter, err := store.States(cfg.Namespace, cfg.Object, cfg.Keys, rangeScanFromConfig(cfg))
	if err != nil {
		return 0, nil, err
	}

	return ws.height, iter, nil
}

// PutState puts a state into DB
func (ws *workingSet) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	_stateDBMtc.WithLabelValues("put").Inc()
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	store, err := ws.matchStore(cfg)
	if err != nil {
		return ws.height, err
	}
	for _, store := range ws.matchStoreWrites(store) {
		key, err := ws.matchStoreKey(store, cfg)
		if err != nil {
			return ws.height, err
		}
		if err := store.PutObject(cfg.Namespace, key, s); err != nil {
			return ws.height, err
		}
	}
	return ws.height, nil
}

// DelState deletes a state from DB
func (ws *workingSet) DelState(opts ...protocol.StateOption) (uint64, error) {
	_stateDBMtc.WithLabelValues("delete").Inc()
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	store, err := ws.matchStore(cfg)
	if err != nil {
		return ws.height, err
	}
	for _, store := range ws.matchStoreWrites(store) {
		key, err := ws.matchStoreKey(store, cfg)
		if err != nil {
			return ws.height, err
		}
		if err := store.DeleteObject(cfg.Namespace, key, cfg.Object); err != nil {
			return ws.height, err
		}
	}
	return ws.height, nil
}

// ReadView reads the view
func (ws *workingSet) ReadView(name string) (protocol.View, error) {
	return ws.views.Read(name)
}

// WriteView writeback the view to factory
func (ws *workingSet) WriteView(name string, v protocol.View) error {
	ws.views.Write(name, v)
	return nil
}

// CreateGenesisStates initialize the genesis states
func (ws *workingSet) CreateGenesisStates(ctx context.Context) error {
	if err := ws.store.CreateGenesisStates(ctx); err != nil {
		return err
	}
	if reg, ok := protocol.GetRegistry(ctx); ok {
		for _, p := range reg.All() {
			if gsc, ok := p.(protocol.GenesisStateCreator); ok {
				if err := gsc.CreateGenesisStates(ctx, ws); err != nil {
					return errors.Wrap(err, "failed to create genesis states for protocol")
				}
			}
		}
	}

	return ws.finalize(ctx)
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
	return ws.checkNonceContinuity(ctx, accountNonceMap)
}

func (ws *workingSet) validateNonceSkipSystemAction(ctx context.Context, blk *block.Block) error {
	accountNonceMap := make(map[string][]uint64)
	for _, selp := range blk.Actions {
		if action.IsSystemAction(selp) {
			continue
		}

		caller := selp.SenderAddress()
		if caller == nil {
			return errors.New("failed to get address")
		}
		srcAddr := caller.String()
		if _, ok := accountNonceMap[srcAddr]; !ok {
			accountNonceMap[srcAddr] = make([]uint64, 0)
		}
		accountNonceMap[srcAddr] = append(accountNonceMap[srcAddr], selp.Nonce())
	}
	return ws.checkNonceContinuity(ctx, accountNonceMap)
}

func (ws *workingSet) checkNonceContinuity(ctx context.Context, accountNonceMap map[string][]uint64) error {
	var (
		pendingNonce uint64
		useZeroNonce = protocol.MustGetFeatureCtx(ctx).UseZeroNonceForFreshAccount
	)
	// Verify each account's Nonce
	for srcAddr, receivedNonces := range accountNonceMap {
		addr, _ := address.FromString(srcAddr)
		confirmedState, err := accountutil.AccountState(ctx, ws, addr)
		if err != nil {
			return errors.Wrapf(err, "failed to get the confirmed nonce of address %s", srcAddr)
		}
		slices.Sort(receivedNonces)
		if useZeroNonce {
			pendingNonce = confirmedState.PendingNonceConsideringFreshAccount()
		} else {
			pendingNonce = confirmedState.PendingNonce()
		}
		for i, nonce := range receivedNonces {
			if nonce != pendingNonce+uint64(i) {
				return errors.Wrapf(
					action.ErrNonceTooHigh,
					"the %d-th nonce %d of address %s (init pending nonce %d) is not continuously increasing",
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

func (ws *workingSet) Process(ctx context.Context, actions []*action.SealedEnvelope) error {
	if protocol.MustGetFeatureCtx(ctx).CorrectValidationOrder {
		return ws.process(ctx, actions)
	}
	return ws.processLegacy(ctx, actions)
}

func (ws *workingSet) process(ctx context.Context, actions []*action.SealedEnvelope) error {
	if err := ws.validate(ctx); err != nil {
		return err
	}
	userActions, systemActions := ws.splitActions(actions)
	// due to archive states only support for account and contract, this may cause
	// validation failure for PutPollResult system action
	// TODO: remove this when archive states support state for all protocols
	ignoreSystemValidation := protocol.MustGetBlockCtx(ctx).Simulate
	if protocol.MustGetFeatureCtx(ctx).PreStateSystemAction && !ignoreSystemValidation {
		if err := ws.validatePostSystemActions(ctx, systemActions); err != nil {
			return err
		}
	}
	reg := protocol.MustGetRegistry(ctx)
	for _, p := range reg.All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return err
			}
		}
	}
	var (
		receipts            = make([]*action.Receipt, 0)
		ctxWithBlockContext = ctx
		blkCtx              = protocol.MustGetBlockCtx(ctx)
		fCtx                = protocol.MustGetFeatureCtx(ctx)
	)
	for _, act := range userActions {
		if err := ws.txValidator.ValidateWithState(ctxWithBlockContext, act); err != nil {
			return err
		}
		actionCtx, err := withActionCtx(ctxWithBlockContext, act)
		if err != nil {
			return err
		}
		for _, p := range reg.All() {
			if validator, ok := p.(protocol.ActionValidator); ok {
				if err := validator.Validate(actionCtx, act.Envelope, ws); err != nil {
					return err
				}
			}
		}
		receipt, err := ws.runAction(actionCtx, act, true)
		if err != nil {
			return errors.Wrap(err, "error when run action")
		}
		receipts = append(receipts, receipt)
		if !action.IsSystemAction(act) {
			blkCtx.GasLimit = deductBlockGas(fCtx, blkCtx.GasLimit, receipt.GasConsumed)
			if fCtx.EnableDynamicFeeTx && receipt.PriorityFee() != nil {
				(&blkCtx.AccumulatedTips).Add(&blkCtx.AccumulatedTips, receipt.PriorityFee())
			}
			ctxWithBlockContext = protocol.WithBlockCtx(ctx, blkCtx)
		}
	}
	// Handle post system actions
	if !protocol.MustGetFeatureCtx(ctx).PreStateSystemAction && !ignoreSystemValidation {
		if err := ws.validatePostSystemActions(ctxWithBlockContext, systemActions); err != nil {
			return err
		}
	}
	for _, act := range systemActions {
		actionCtx, err := withActionCtx(ctxWithBlockContext, act)
		if err != nil {
			return err
		}
		receipt, err := ws.runAction(actionCtx, act, true)
		if err != nil {
			return errors.Wrap(err, "error when run action")
		}
		receipts = append(receipts, receipt)
	}
	if fCtx.CorrectTxLogIndex {
		updateReceiptIndex(receipts)
	}
	ws.receipts = receipts
	return ws.finalize(ctx)
}

func (ws *workingSet) processLegacy(ctx context.Context, actions []*action.SealedEnvelope) error {
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
				if err := validator.Validate(ctxWithActionContext, act.Envelope, ws); err != nil {
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

	receipts, err := ws.runActionsLegacy(ctx, actions)
	if err != nil {
		return err
	}
	ws.receipts = receipts
	return ws.finalize(ctx)
}

func (ws *workingSet) runActionsLegacy(
	ctx context.Context,
	elps []*action.SealedEnvelope,
) ([]*action.Receipt, error) {
	// Handle actions
	receipts := make([]*action.Receipt, 0)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	fCtx := protocol.MustGetFeatureCtx(ctx)
	for _, elp := range elps {
		ctxWithActionContext, err := withActionCtx(ctx, elp)
		if err != nil {
			return nil, err
		}
		receipt, err := ws.runAction(protocol.WithBlockCtx(ctxWithActionContext, blkCtx), elp, true)
		if err != nil {
			return nil, errors.Wrap(err, "error when run action")
		}
		receipts = append(receipts, receipt)
		if fCtx.EnableDynamicFeeTx && receipt.PriorityFee() != nil {
			(&blkCtx.AccumulatedTips).Add(&blkCtx.AccumulatedTips, receipt.PriorityFee())
		}
	}
	if fCtx.CorrectTxLogIndex {
		updateReceiptIndex(receipts)
	}
	return receipts, nil
}

func (ws *workingSet) generateSystemActions(ctx context.Context) ([]action.Envelope, error) {
	reg := protocol.MustGetRegistry(ctx)
	postSystemActions := []action.Envelope{}
	for _, p := range reg.All() {
		if psc, ok := p.(protocol.PostSystemActionsCreator); ok {
			elps, err := psc.CreatePostSystemActions(ctx, ws)
			if err != nil {
				return nil, err
			}
			postSystemActions = append(postSystemActions, elps...)
		}
	}
	return postSystemActions, nil
}

// validateSystemActionLayout verify whether the post system actions are appended tail
func (ws *workingSet) validateSystemActionLayout(ctx context.Context, actions []*action.SealedEnvelope) error {
	// system actions should be at the end of the action list, and they should be continuous
	hitSystemAction := false
	for i := range actions {
		if hitSystemAction {
			if !action.IsSystemAction(actions[i]) {
				return errors.Wrapf(errInvalidSystemActionLayout, "the %d-th action should be a system action", i)
			}
			continue
		} else if action.IsSystemAction(actions[i]) {
			hitSystemAction = true
		}
	}
	return nil
}

func (ws *workingSet) validatePostSystemActions(ctx context.Context, systemActions []*action.SealedEnvelope) error {
	postSystemActions, err := ws.generateSystemActions(ctx)
	if err != nil {
		return err
	}
	if len(postSystemActions) != len(systemActions) {
		log.L().Error("the number of system actions is incorrect",
			zap.Int("expected", len(postSystemActions)),
			zap.Int("got", len(systemActions)),
			zap.Uint64("height", protocol.MustGetBlockCtx(ctx).BlockHeight),
		)
		for i, act := range postSystemActions {
			log.L().Error("expected system action",
				zap.Int("index", i),
				zap.String("action", fmt.Sprintf("%+T", act.Action())),
			)
		}
		return errors.Wrapf(errInvalidSystemActionLayout, "the number of system actions is incorrect, expected %d, got %d", len(postSystemActions), len(systemActions))
	}
	reg := protocol.MustGetRegistry(ctx)
	for i, act := range systemActions {
		actionCtx, err := withActionCtx(ctx, act)
		if err != nil {
			return err
		}
		for _, p := range reg.All() {
			if validator, ok := p.(protocol.ActionValidator); ok {
				if err := validator.Validate(actionCtx, act.Envelope, ws); err != nil {
					return err
				}
			}
		}
		if actual, expect := act.Envelope.Proto().String(), postSystemActions[i].Proto().String(); actual != expect {
			return errors.Wrapf(errInvalidSystemActionLayout, "the %d-th action is not the expected system action: %v, got %v", i, expect, actual)
		}
	}
	return nil
}

func (ws *workingSet) splitActions(acts []*action.SealedEnvelope) (userActions []*action.SealedEnvelope, systemActions []*action.SealedEnvelope) {
	for _, act := range acts {
		if action.IsSystemAction(act) {
			systemActions = append(systemActions, act)
		} else {
			userActions = append(userActions, act)
		}
	}
	return userActions, systemActions
}

func (ws *workingSet) pickAndRunActions(
	ctx context.Context,
	ap actpool.ActPool,
	sign func(elp action.Envelope) (*action.SealedEnvelope, error),
	allowedBlockGasResidue uint64,
) ([]*action.SealedEnvelope, error) {
	err := ws.validate(ctx)
	if err != nil {
		return nil, err
	}
	receipts := make([]*action.Receipt, 0)
	executedActions := make([]*action.SealedEnvelope, 0)
	reg := protocol.MustGetRegistry(ctx)

	var (
		systemActions []*action.SealedEnvelope
	)
	if protocol.MustGetFeatureCtx(ctx).PreStateSystemAction {
		systemActions, err = ws.generateSignedSystemActions(ctx, sign)
		if err != nil {
			return nil, err
		}
	}

	for _, p := range reg.All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return nil, err
			}
		}
	}

	// initial action iterator
	var (
		ctxWithBlockContext = ctx
		blkCtx              = protocol.MustGetBlockCtx(ctx)
		fCtx                = protocol.MustGetFeatureCtx(ctx)
		blobCnt             = uint64(0)
		blobLimit           = action.MaxBlobGasPerBlock / params.BlobTxBlobGasPerBlob
		deadline            *time.Time
		fullGas             = blkCtx.GasLimit
	)
	if ap != nil {
		if dl, ok := ctx.Deadline(); ok {
			deadline = &dl
		}
		bp := ap.BundlePool()
		if bp != nil {
			bids, _, bundles, err := bp.BundlesAtHeight(ws.height)
			switch errors.Cause(err) {
			case nil:
				for i, bundle := range bundles {
					bh := bundle.Hash()
					log.L().Info("processing bundle", zap.String("uuid", bids[i]), zap.Uint64("height", ws.height), zap.String("hash", hex.EncodeToString(bh[:])), zap.Int("size", bundle.Len()), zap.Uint64("gas", bundle.Gas()))
					bBlkCtx := blkCtx
					// A struct copy shares the big.Int's words, and adding the
					// bundle's tips to them in place would also change blkCtx.
					bBlkCtx.AccumulatedTips = *new(big.Int).Set(&blkCtx.AccumulatedTips)
					bGasLimit := bBlkCtx.GasLimit
					if deadline != nil && time.Now().After(*deadline) {
						duration := time.Since(bBlkCtx.BlockTimeStamp)
						log.L().Warn("Stop processing actions due to deadline, please consider increasing hardware", zap.Time("deadline", *deadline), zap.Duration("duration", duration), zap.Int("actions", len(executedActions)), zap.Uint64("gas", fullGas-bGasLimit))
						_mintAbility.WithLabelValues("saturation").Set(1)
						break
					}
					if bundle.Gas() > bGasLimit {
						log.L().Info("Skip bundle exceeds gas limit", zap.String("hash", hex.EncodeToString(bh[:])), zap.Uint64("height", ws.height), zap.Uint64("gas", bundle.Gas()), zap.Uint64("blkGasLimit", bGasLimit))
						continue
					}
					bBlobCnt := blobCnt
					bReceipts := make([]*action.Receipt, 0, bundle.Len())
					// ws.Snapshot() rather than ws.store.Snapshot(): a bundle
					// that gives up part way through has to put the protocol
					// views back as well as the store, and only the working
					// set snapshot records both. Mint is the only caller of
					// this path, so a proposer that skips a bundle carries on
					// from the state it held before the bundle ran.
					si, ownerOrOpChanged, deferred := ws.Snapshot(), ws.mintOwnerOrOpChanged, maps.Clone(ws.mintDeferred)
					if err := bundle.ForEach(func(selp *action.SealedEnvelope) error {
						_, _, receipt, err := ws.validateAndRun(ctxWithBlockContext, reg, selp, bGasLimit, bBlobCnt, uint64(blobLimit), false)
						if err != nil {
							return errors.Wrapf(err, "failed to run action in bundle %s at height %d", bids[i], ws.height)
						}
						if receipt == nil {
							h, err := selp.Hash()
							if err != nil {
								log.L().Error("failed to get hash for action in bundle", zap.String("uuid", bids[i]), zap.Uint64("height", ws.height), zap.Error(err))
							}
							return errors.Errorf("receipt is nil for transaction %x", h)
						}
						bGasLimit = deductBlockGas(fCtx, bGasLimit, receipt.GasConsumed)
						if fCtx.EnableDynamicFeeTx && receipt.PriorityFee() != nil {
							(&bBlkCtx.AccumulatedTips).Add(&bBlkCtx.AccumulatedTips, receipt.PriorityFee())
						}
						ctxWithBlockContext = protocol.WithBlockCtx(ctx, bBlkCtx)
						bReceipts = append(bReceipts, receipt)
						bBlobCnt += uint64(len(selp.BlobHashes()))

						return nil
					}); err != nil {
						log.L().Warn("failed to process bundle", zap.String("uuid", bids[i]), zap.Uint64("height", ws.height), zap.Error(err))
						if err := ws.Revert(si); err != nil {
							return nil, errors.Wrapf(err, "failed to revert snapshot %d for bundle %s at height %d", si, bids[i], ws.height)
						}
						// drop the bundle's block context too, or what runs
						// next sees the tips of actions that are not in the block
						ctxWithBlockContext = protocol.WithBlockCtx(ctx, blkCtx)
						ws.mintOwnerOrOpChanged, ws.mintDeferred = ownerOrOpChanged, deferred
						continue
					}
					for _, receipt := range bReceipts {
						blkCtx.GasLimit = deductBlockGas(fCtx, blkCtx.GasLimit, receipt.GasConsumed)
						if fCtx.EnableDynamicFeeTx && receipt.PriorityFee() != nil {
							(&blkCtx.AccumulatedTips).Add(&blkCtx.AccumulatedTips, receipt.PriorityFee())
						}
						receipts = append(receipts, receipt)
					}
					ctxWithBlockContext = protocol.WithBlockCtx(ctx, blkCtx)
					bundle.ForEach(func(selp *action.SealedEnvelope) error {
						executedActions = append(executedActions, selp)
						blobCnt += uint64(len(selp.BlobHashes()))
						return nil
					})
					log.L().Info("processed bundle", zap.String("hash", hex.EncodeToString(bh[:])), zap.Uint64("height", ws.height))
				}
			case actpool.ErrNoBundlesForHeight:
				log.L().Debug("no bundles for height", zap.Uint64("height", ws.height))
			default:
				return nil, errors.Wrapf(err, "failed to get bundles at height %d", ws.height)
			}
		}
		actionIterator := actioniterator.NewActionIterator(ap.PendingActionMap())
		for {
			if deadline != nil && time.Now().After(*deadline) {
				duration := time.Since(blkCtx.BlockTimeStamp)
				log.L().Warn("Stop processing actions due to deadline, please consider increasing hardware", zap.Time("deadline", *deadline), zap.Duration("duration", duration), zap.Int("actions", len(executedActions)), zap.Uint64("gas", fullGas-blkCtx.GasLimit))
				_mintAbility.WithLabelValues("saturation").Set(1)
				break
			}
			nextAction, ok := actionIterator.Next()
			if !ok {
				_mintAbility.WithLabelValues("saturation").Set(0)
				break
			}
			popAccount, deleteAction, receipt, err := ws.validateAndRun(ctxWithBlockContext, reg, nextAction, blkCtx.GasLimit, blobCnt, uint64(blobLimit), true)
			if popAccount {
				actionIterator.SkipAccount(nextAction.SenderAddress())
			}
			if deleteAction {
				ap.DeleteAction(nextAction.SenderAddress())
			}
			if errors.Is(err, errMintDeadline) {
				log.L().Warn("Stop processing actions due to deadline, please consider increasing hardware", zap.Timep("deadline", deadline), zap.Duration("duration", time.Since(blkCtx.BlockTimeStamp)), zap.Int("actions", len(executedActions)), zap.Uint64("gas", fullGas-blkCtx.GasLimit))
				_mintAbility.WithLabelValues("saturation").Set(1)
				break
			}
			if err != nil {
				return nil, err
			}
			if receipt == nil {
				continue
			}
			blkCtx.GasLimit = deductBlockGas(fCtx, blkCtx.GasLimit, receipt.GasConsumed)
			if fCtx.EnableDynamicFeeTx && receipt.PriorityFee() != nil {
				(&blkCtx.AccumulatedTips).Add(&blkCtx.AccumulatedTips, receipt.PriorityFee())
			}
			ctxWithBlockContext = protocol.WithBlockCtx(ctx, blkCtx)
			receipts = append(receipts, receipt)
			executedActions = append(executedActions, nextAction)
			blobCnt += uint64(len(nextAction.BlobHashes()))

			// To prevent loop all actions in act_pool, we stop processing action when remaining gas is below
			// than certain threshold
			if blkCtx.GasLimit < allowedBlockGasResidue {
				_mintAbility.WithLabelValues("saturation").Set(0)
				break
			}
		}
	}

	if ws.mintDrops > _mintDropLogLimit {
		log.L().Warn("dropped more actions that failed during mint than were logged", zap.Uint64("height", ws.height), zap.Int("dropped", ws.mintDrops), zap.Int("logged", _mintDropLogLimit))
	}

	if !fCtx.PreStateSystemAction {
		systemActions, err = ws.generateSignedSystemActions(ctx, sign)
		if err != nil {
			return nil, err
		}
	}

	for _, selp := range systemActions {
		actionCtx, err := withActionCtx(ctxWithBlockContext, selp)
		if err != nil {
			return nil, err
		}
		receipt, err := ws.runAction(actionCtx, selp, true)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
		executedActions = append(executedActions, selp)
	}
	if fCtx.CorrectTxLogIndex {
		updateReceiptIndex(receipts)
	}
	ws.receipts = receipts

	return executedActions, ws.finalize(ctx)
}

func (ws *workingSet) validateAndRun(
	ctx context.Context,
	reg *protocol.Registry,
	nextAction *action.SealedEnvelope,
	gasLimit uint64,
	blobCnt uint64,
	blobLimit uint64,
	revertAllSnapshots bool,
) (bool, bool, *action.Receipt, error) {
	if nextAction.Gas() > gasLimit {
		log.L().Info("action gas exceeds limit", zap.Uint64("height", ws.height), zap.Uint64("gasLimit", gasLimit), zap.Uint64("actionGas", nextAction.Gas()))
		countMintSkip("gas_limit", nextAction)
		return true, false, nil, nil
	}
	if blobCnt+uint64(len(nextAction.BlobHashes())) > uint64(blobLimit) {
		log.L().Info("blob count exceeds limit", zap.Uint64("height", ws.height), zap.Uint64("blobCnt", blobCnt), zap.Uint64("blobLimit", blobLimit))
		countMintSkip("blob_limit", nextAction)
		return true, false, nil, nil
	}
	if container, ok := nextAction.Envelope.(action.TxContainer); ok {
		if err := container.Unfold(nextAction, ctx, ws.checkContract); err != nil {
			log.L().Info("failed to unfold tx container", zap.Uint64("height", ws.height), zap.Error(err))
			countMintSkip("unfold", nextAction)
			return true, true, nil, nil
		}
		// Unfold replaces the folded container with the native action, but
		// admission only ran SanityCheck on the container, whose check does not
		// decode the staking calldata. Re-run the stateless checks on the
		// unfolded action here, so that an action that is invalid only after
		// unfolding (e.g. depositToStake with amount 0) is left out of the block
		// instead of being minted into it.
		fCtx := protocol.MustGetFeatureCtx(ctx)
		if err := nextAction.Envelope.SanityCheck(); err != nil && !fCtx.Tolerate(err) {
			log.L().Info("unfolded action failed sanity check", zap.Uint64("height", ws.height), zap.Error(err))
			countMintSkip("sanity", nextAction)
			return true, true, nil, nil
		}
		if intrinsicGas, err := nextAction.IntrinsicGas(); err != nil {
			log.L().Info("failed to get unfolded action intrinsic gas", zap.Uint64("height", ws.height), zap.Error(err))
			countMintSkip("intrinsic_gas", nextAction)
			return true, true, nil, nil
		} else if intrinsicGas > nextAction.Gas() {
			log.L().Info("unfolded action intrinsic gas exceeds gas limit", zap.Uint64("height", ws.height), zap.Uint64("intrinsicGas", intrinsicGas), zap.Uint64("gas", nextAction.Gas()))
			countMintSkip("intrinsic_gas", nextAction)
			return true, true, nil, nil
		}
	}
	// a sender held back by the candidate cap below waits for the next block
	if sender := nextAction.SenderAddress(); sender != nil && ws.mintDeferred[sender.String()] {
		return false, false, nil, nil
	}
	if err := ws.txValidator.ValidateWithState(ctx, nextAction); err != nil {
		log.L().Debug("failed to ValidateWithState", zap.Uint64("height", ws.height), zap.Error(err))
		if !errors.Is(err, action.ErrNonceTooLow) {
			countMintSkip("validate", nextAction)
			return true, true, nil, nil
		}
		countMintSkip("nonce_too_low", nextAction)
		return false, false, nil, nil
	}
	actionCtx, err := withActionCtx(ctx, nextAction)
	if err == nil {
		for _, p := range reg.All() {
			if validator, ok := p.(protocol.ActionValidator); ok {
				if err = validator.Validate(actionCtx, nextAction.Envelope, ws); err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		log.L().Info("failed to validate tx", zap.Uint64("height", ws.height), zap.Error(err))
		countMintSkip("validate", nextAction)
		// without a sender there is nobody to evict
		return true, nextAction.SenderAddress() != nil, nil, nil
	}
	// after a candidate's owner or operator changed in this block, later
	// candidate actions, and the rest of their senders' actions, wait
	if ws.mintOwnerOrOpChanged && isCandidateAction(nextAction.Action()) {
		if sender := nextAction.SenderAddress(); sender != nil {
			ws.mintDeferred[sender.String()] = true
		}
		return false, false, nil, nil
	}
	// An action that fails says nothing about the rest of the block, so it is
	// reverted to this snapshot and dropped on its own; runAction must not
	// reset the store snapshots before that, so the reset is deferred here.
	si := ws.Snapshot()
	if revertAllSnapshots {
		defer ws.store.ResetSnapshots()
	}
	receipt, err := ws.runActionDuringMint(actionCtx, nextAction, false)
	switch {
	case err == nil:
		if receipt.Status == uint64(iotextypes.ReceiptStatus_Success) && changesOwnerOrOperator(nextAction.Action()) {
			ws.mintOwnerOrOpChanged = true
		}
		return false, false, receipt, nil
	case ctx.Err() != nil:
		// neither is the sender's fault, so it is not evicted
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return true, false, nil, errors.Wrap(err, "mint cancelled while running an action")
		}
		// out of time: drop this action and seal what the draft holds, as
		// the mint loop does when the deadline passes between actions
		if rerr := ws.revertDuringMint(si); rerr != nil {
			return true, false, nil, errors.Wrap(rerr, "failed to revert action at mint deadline")
		}
		countMintSkip("deadline", nextAction)
		return true, false, nil, errMintDeadline
	case errors.Cause(err) == action.ErrGasLimit:
		log.L().Info("runAction() failed due to gas limit", zap.Uint64("height", ws.height), zap.Error(err))
		countMintSkip("gas_limit", nextAction)
		return true, false, nil, nil
	}
	nextActionHash, _ := nextAction.Hash()
	if ws.mintDrops < _mintDropLogLimit {
		log.L().Warn("dropping action that failed during mint", zap.Uint64("height", ws.height), log.Hex("action", nextActionHash[:]), zap.Error(err))
	}
	ws.mintDrops++
	if rerr := ws.revertDuringMint(si); rerr != nil {
		return true, true, nil, errors.Wrapf(rerr, "failed to revert action %x", nextActionHash)
	}
	reason := "run_failed"
	if errors.Is(err, errMintActionPanic) {
		reason = "panic"
	}
	countMintSkip(reason, nextAction)
	return true, true, nil, nil
}

// countMintSkip counts an action left out of a draft block.
func countMintSkip(reason string, selp *action.SealedEnvelope) {
	actType := "unknown"
	if act := selp.Envelope.Action(); act != nil {
		t := reflect.TypeOf(act)
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		actType = t.Name()
	}
	_mintSkipMtc.WithLabelValues(reason, actType).Inc()
}

// revertDuringMint reverts the working set to snapshot si after an action
// failed. The draft can only be kept if the revert succeeds, so a panic in
// the revert is returned as an error and the caller abandons the draft.
func (ws *workingSet) revertDuringMint(si int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("recovered from panic while reverting snapshot %d: %v", si, r)
		}
	}()
	if err := ws.Revert(si); err != nil {
		return errors.Wrapf(err, "failed to revert snapshot %d", si)
	}
	return nil
}

// isCandidateAction reports whether act manages a candidate (not a bucket)
func isCandidateAction(act action.Action) bool {
	switch act.(type) {
	case *action.CandidateRegister, *action.CandidateUpdate, *action.CandidateTransferOwnership,
		*action.CandidateActivate, *action.CandidateDeactivate,
		*action.SetVoterRewardOptIn:
		return true
	}
	return false
}

// changesOwnerOrOperator reports whether act, if successful, may change a
// candidate's owner or operator (an unchanged operator also counts)
func changesOwnerOrOperator(act action.Action) bool {
	switch act := act.(type) {
	case *action.CandidateTransferOwnership:
		return true
	case *action.CandidateUpdate:
		return act.OperatorAddress() != nil
	}
	return false
}

// runActionDuringMint runs a single action while assembling a draft block, recovering from
// any panic raised in the process. A panic here otherwise unwinds past this whole draft (caught
// only by the mint goroutine's recover in blockpreparer.go) and, unlike a normal error return,
// skips the caller's sender-eviction logic — so the same poison action would be picked again on
// every subsequent mint attempt, stalling block production. Converting the panic into an
// ordinary error lets validateAndRun revert and drop the action and evict its sender, and the
// draft carries on without it.
func (ws *workingSet) runActionDuringMint(ctx context.Context, selp *action.SealedEnvelope, revertAllSnapshots bool) (receipt *action.Receipt, err error) {
	defer func() {
		if r := recover(); r != nil {
			_mintActionPanicMtc.Inc()
			actHash, hashErr := selp.Hash()
			if hashErr != nil {
				log.L().Error("failed to get action hash after recovering from mint-time panic", zap.Error(hashErr))
			}
			if ws.mintDrops < _mintDropLogLimit {
				log.L().Error("recovered from panic while running action during mint; sender will be evicted from the pool",
					log.Hex("action", actHash[:]),
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())))
			}
			receipt = nil
			err = errors.Wrapf(errMintActionPanic, "%x: %v", actHash, r)
		}
	}()
	return ws.runAction(ctx, selp, revertAllSnapshots)
}

func (ws *workingSet) generateSignedSystemActions(ctx context.Context, sign func(elp action.Envelope) (*action.SealedEnvelope, error)) ([]*action.SealedEnvelope, error) {
	unsignedSystemActions, err := ws.generateSystemActions(ctx)
	if err != nil {
		return nil, err
	}
	postSystemActions := make([]*action.SealedEnvelope, len(unsignedSystemActions))
	for i, elp := range unsignedSystemActions {
		selp, err := sign(elp)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to sign %+v", elp.Action())
		}
		postSystemActions[i] = selp
	}
	return postSystemActions, nil
}

func updateReceiptIndex(receipts []*action.Receipt) {
	var txIndex, logIndex uint32
	for _, r := range receipts {
		logIndex = r.UpdateIndex(txIndex, logIndex)
		txIndex++
	}
}

func (ws *workingSet) ValidateBlock(ctx context.Context, blk *block.Block) (err error) {
	// A proposed block is untrusted input. Recover any panic raised while
	// processing its actions and reject the block instead of letting the panic
	// kill the process — a single malformed action must not take a validating
	// node down. The working set is discarded by the caller on error, so no
	// partial state leaks, and no committed state changes (such a block could
	// never validate successfully either way), so this needs no fork gate. The
	// commit/PutBlock path (trusted data) intentionally stays fatal elsewhere.
	defer func() {
		if r := recover(); r != nil {
			_validateBlockPanicMtc.Inc()
			err = errors.Errorf("recovered from panic while validating block at height %d: %v", blk.Height(), r)
			log.L().Error("recovered from panic while validating block; block rejected, process kept alive",
				zap.Uint64("height", blk.Height()),
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()
	fCtx := protocol.MustGetFeatureCtx(ctx)
	if fCtx.SkipSystemActionNonce {
		if err := ws.validateNonceSkipSystemAction(ctx, blk); err != nil {
			return errors.Wrap(err, "failed to validate nonce")
		}
	} else {
		if err := ws.validateNonce(ctx, blk); err != nil {
			return errors.Wrap(err, "failed to validate nonce")
		}
	}
	if fCtx.ValidateSystemAction {
		if err := ws.validateSystemActionLayout(ctx, blk.RunnableActions().Actions()); err != nil {
			return err
		}
	}

	if fCtx.EnableDynamicFeeTx {
		bcCtx := protocol.MustGetBlockchainCtx(ctx)
		if err := protocol.VerifyEIP1559Header(
			genesis.MustExtractGenesisContext(ctx).Blockchain, &bcCtx.Tip, &blk.Header); err != nil {
			return err
		}
	}
	if fCtx.EnableBlobTransaction {
		blobCnt := uint64(0)
		blobLimit := uint64(action.MaxBlobGasPerBlock / params.BlobTxBlobGasPerBlob)
		for _, selp := range blk.Actions {
			blobCnt += uint64(len(selp.BlobHashes()))
			if blobCnt > blobLimit {
				return errors.New("too many blob transactions in a block")
			}
		}
		bcCtx := protocol.MustGetBlockchainCtx(ctx)
		if err := protocol.VerifyEIP4844Header(&bcCtx.Tip, &blk.Header); err != nil {
			return err
		}
	}
	if err := ws.Process(ctx, blk.RunnableActions().Actions()); err != nil {
		log.L().Error("Failed to update state.", zap.Uint64("height", ws.height), zap.Error(err))
		return err
	}
	fwCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	if !fwCtx.CandCenterHasAlias(blk.Height()) {
		if err := ws.views.Commit(ctx, ws); err != nil {
			return err
		}
	}

	digest, err := ws.digest()
	if err != nil {
		return err
	}
	if !blk.VerifyDeltaStateDigest(digest) {
		ws.dumpWriteQueue(blk.Height())
		return errors.Wrapf(block.ErrDeltaStateMismatch, "digest in block '%x' vs digest in workingset '%x' at height %d", blk.DeltaStateDigest(), digest, blk.Height())
	}
	receiptRoot := calculateReceiptRoot(ws.receipts)
	if !blk.VerifyReceiptRoot(receiptRoot) {
		return errors.Wrapf(block.ErrReceiptRootMismatch, "receipt root in block '%x' vs receipt root in workingset '%x'", blk.ReceiptRoot(), receiptRoot)
	}
	// The proposer derives the header's gasUsed/blobGasUsed from its own
	// receipts in CreateBuilder; re-derive them here from the receipts this
	// node produced and reject a header that disagrees. Each field is checked
	// only when the feature that makes the proposer set it is active, so the
	// validator never demands a value the proposer never wrote.
	if fCtx.ValidateHeaderGasUsed {
		if fCtx.EnableDynamicFeeTx {
			gasUsed := calculateGasUsed(ws.receipts)
			if !blk.VerifyGasUsed(gasUsed) {
				return errors.Wrapf(block.ErrGasUsedMismatch, "gas used in block %d vs gas used in workingset %d at height %d", blk.GasUsed(), gasUsed, blk.Height())
			}
		}
		if fCtx.EnableBlobTransaction {
			blobGasUsed := calculateBlobGasUsed(ws.receipts)
			if !blk.VerifyBlobGasUsed(blobGasUsed) {
				return errors.Wrapf(block.ErrBlobGasUsedMismatch, "blob gas used in block %d vs blob gas used in workingset %d at height %d", blk.BlobGasUsed(), blobGasUsed, blk.Height())
			}
		}
	}

	return nil
}

func (ws *workingSet) CreateBuilder(
	ctx context.Context,
	ap actpool.ActPool,
	sign func(elp action.Envelope) (*action.SealedEnvelope, error),
	allowedBlockGasResidue uint64,
) (*block.Builder, error) {
	actions, err := ws.pickAndRunActions(ctx, ap, sign, allowedBlockGasResidue)
	if err != nil {
		return nil, err
	}
	if err := ws.views.Commit(ctx, ws); err != nil {
		return nil, err
	}

	var (
		blkCtx = protocol.MustGetBlockCtx(ctx)
		bcCtx  = protocol.MustGetBlockchainCtx(ctx)
		fCtx   = protocol.MustGetFeatureCtx(ctx)
	)
	digest, err := ws.digest()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get digest")
	}

	ra := block.NewRunnableActionsBuilder().
		AddActions(actions...).
		Build()
	blkBuilder := block.NewBuilder(ra).
		SetHeight(blkCtx.BlockHeight).
		SetTimestamp(blkCtx.BlockTimeStamp).
		SetPrevBlockHash(bcCtx.Tip.Hash).
		SetDeltaStateDigest(digest).
		SetReceipts(ws.receipts).
		SetReceiptRoot(calculateReceiptRoot(ws.receipts)).
		SetLogsBloom(calculateLogsBloom(ctx, ws.receipts))
	if fCtx.EnableDynamicFeeTx {
		blkBuilder.SetGasUsed(calculateGasUsed(ws.receipts))
		blkBuilder.SetBaseFee(blkCtx.BaseFee)
	}
	if fCtx.EnableBlobTransaction {
		blkBuilder.SetBlobGasUsed(calculateBlobGasUsed(ws.receipts))
		blkBuilder.SetExcessBlobGas(blkCtx.ExcessBlobGas)
	}
	return blkBuilder, nil
}

func (ws *workingSet) NewWorkingSet(ctx context.Context) (*workingSet, error) {
	if !ws.finalized {
		return nil, errors.New("workingset has not been finalized yet")
	}
	kvStore := ws.store.KVStore()
	if kvStore == nil {
		return nil, errors.Errorf("KVStore() not supported in %T", ws.store)
	}
	store, err := ws.workingSetStoreFactory.CreateWorkingSetStore(ctx, ws.height+1, kvStore)
	if err != nil {
		return nil, err
	}
	views := ws.views.Fork()
	return newWorkingSet(ws.height+1, views, store, ws.workingSetStoreFactory), nil
}

func (ws *workingSet) Close() {
	ws.store.Close()
}

func (ws *workingSet) Erigon() (*erigonstate.IntraBlockState, bool) {
	switch st := ws.store.(type) {
	case *workingSetStoreWithSecondary:
		if wss, ok := st.writerSecondary.(*erigonstore.ErigonWorkingSetStore); ok {
			return wss.IntraBlockState(), false
		}
		return nil, false
	case *erigonWorkingSetStoreForSimulate:
		return st.IntraBlockState(), true
	default:
		return nil, false
	}
}

func (ws *workingSet) matchStore(cfg *protocol.StateConfig) (workingSetStore, error) {
	store := ws.store
	if cfg.ErigonStoreOnly {
		erigonStore, err := store.ErigonStore()
		if err != nil {
			return nil, err
		}
		store = erigonStore.(workingSetStore)
	}
	return store, nil
}

func (ws *workingSet) matchStoreWrites(store workingSetStore) []workingSetStore {
	if ss, ok := store.(*workingSetStoreWithSecondary); ok {
		return []workingSetStore{ss.writer, ss.writerSecondary}
	}
	return []workingSetStore{store}
}

func (ws *workingSet) matchStoreKey(store workingSetStore, cfg *protocol.StateConfig) ([]byte, error) {
	if len(cfg.ErigonStoreKey) == 0 {
		return cfg.Key, nil
	}
	switch store.(type) {
	case *erigonWorkingSetStoreForSimulate, *erigonstore.ErigonWorkingSetStore:
		return cfg.ErigonStoreKey, nil
	case *stateDBWorkingSetStore, *workingSetStoreWithSecondary:
		return cfg.Key, nil
	default:
		return nil, errors.Errorf("unsupported store type %T", store)
	}
}
