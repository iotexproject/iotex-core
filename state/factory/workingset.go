// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync/atomic"
	"time"

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
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
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

	errInvalidSystemActionLayout = errors.New("system action layout is invalid")
	errUnfoldTxContainer         = errors.New("failed to unfold tx container")
	errDeployerNotWhitelisted    = errors.New("deployer not whitelisted")
)

func init() {
	prometheus.MustRegister(_stateDBMtc)
	prometheus.MustRegister(_mintAbility)
}

type (
	workingSet struct {
		height      uint64
		store       workingSetStore
		finalized   bool
		abandoned   atomic.Bool
		dock        protocol.Dock
		txValidator *protocol.GenericValidator
		receipts    []*action.Receipt
		parent      *workingSet
	}
)

func newWorkingSet(height uint64, store workingSetStore, parent *workingSet) *workingSet {
	ws := &workingSet{
		height: height,
		store:  store,
		dock:   protocol.NewDock(),
		parent: parent,
	}
	ws.txValidator = protocol.NewGenericValidator(ws, accountutil.AccountState)
	return ws
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

func (ws *workingSet) isAbandoned() bool {
	return ws.abandoned.Load()
}

func (ws *workingSet) abandon() {
	ws.abandoned.Store(true)
}

func (ws *workingSet) verifyParent() error {
	if ws.parent != nil && ws.parent.isAbandoned() {
		ws.abandon()
		return errors.New("workingset abandoned")
	}
	return nil
}

func (ws *workingSet) detachParent() {
	ws.parent = nil
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
	fCtx := protocol.MustGetFeatureCtx(ctx)
	// if it's a tx container, unfold the tx inside
	if fCtx.UseTxContainer && !fCtx.UnfoldContainerBeforeValidate {
		if container, ok := selp.Envelope.(action.TxContainer); ok {
			if err := container.Unfold(selp, ctx, ws.checkContract); err != nil {
				return nil, errors.Wrap(errUnfoldTxContainer, err.Error())
			}
		}
	}
	// verify the tx is not container format (unfolded correctly)
	if fCtx.VerifyNotContainerBeforeRun && selp.Encoding() == uint32(iotextypes.Encoding_TX_CONTAINER) {
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
	defer ws.ResetSnapshots()
	if err := ws.freshAccountConversion(ctx, &actCtx); err != nil {
		return nil, err
	}
	for _, actionHandler := range reg.All() {
		receipt, err := actionHandler.Handle(ctx, selp.Envelope, ws)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"error when action %x mutates states",
				selpHash,
			)
		}
		if receipt != nil {
			if fCtx.EnableBlobTransaction && len(selp.BlobHashes()) > 0 {
				if err = ws.handleBlob(ctx, selp, receipt); err != nil {
					return nil, err
				}
			}
			fmt.Printf("action %x added to block\n", selpHash)
			return receipt, nil
		}
	}
	return nil, errors.New("receipt is empty")
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

func (ws *workingSet) getDirty(ns string, key []byte) ([]byte, bool) {
	return ws.store.GetDirty(ns, key)
}

// Commit persists all changes in RunActions() into the DB
func (ws *workingSet) Commit(ctx context.Context) error {
	if err := ws.verifyParent(); err != nil {
		return err
	}
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
	_stateDBMtc.WithLabelValues("get").Inc()
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	if cfg.Keys != nil {
		return 0, errors.Wrap(ErrNotSupported, "Read state with keys option has not been implemented yet")
	}
	if ws.parent != nil {
		if value, dirty := ws.getDirty(cfg.Namespace, cfg.Key); dirty {
			return ws.height, state.Deserialize(s, value)
		}
		if value, dirty := ws.parent.getDirty(cfg.Namespace, cfg.Key); dirty {
			return ws.height, state.Deserialize(s, value)
		}
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
		return 0, nil, errors.Wrap(ErrNotSupported, "Read states with key option has not been implemented yet")
	}
	// TODO: check parent
	keys, values, err := ws.store.States(cfg.Namespace, cfg.Keys)
	if err != nil {
		return 0, nil, err
	}
	iter, err := state.NewIterator(keys, values)
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
	ss, err := state.Serialize(s)
	if err != nil {
		return ws.height, errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}
	return ws.height, ws.store.Put(cfg.Namespace, cfg.Key, ss)
}

// DelState deletes a state from DB
func (ws *workingSet) DelState(opts ...protocol.StateOption) (uint64, error) {
	_stateDBMtc.WithLabelValues("delete").Inc()
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

// CreateGenesisStates initialize the genesis states
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
		sort.Slice(receivedNonces, func(i, j int) bool { return receivedNonces[i] < receivedNonces[j] })
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
	return ws.processWithCorrectOrder(ctx, actions)
}

func (ws *workingSet) processWithCorrectOrder(ctx context.Context, actions []*action.SealedEnvelope) error {
	if err := ws.verifyParent(); err != nil {
		return err
	}
	if err := ws.validate(ctx); err != nil {
		return err
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
	for _, act := range actions {
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
		receipt, err := ws.runAction(actionCtx, act)
		if err != nil {
			return errors.Wrap(err, "error when run action")
		}
		receipts = append(receipts, receipt)
		if !action.IsSystemAction(act) {
			blkCtx.GasLimit -= receipt.GasConsumed
			if fCtx.EnableDynamicFeeTx && receipt.PriorityFee() != nil {
				(&blkCtx.AccumulatedTips).Add(&blkCtx.AccumulatedTips, receipt.PriorityFee())
			}
			ctxWithBlockContext = protocol.WithBlockCtx(ctx, blkCtx)
		}
	}
	if fCtx.CorrectTxLogIndex {
		updateReceiptIndex(receipts)
	}
	ws.receipts = receipts
	return ws.finalize()
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
	postSystemActions, err := ws.generateSystemActions(ctx)
	if err != nil {
		return err
	}
	// system actions should be at the end of the action list, and they should be continuous
	expectedStartIdx := len(actions) - len(postSystemActions)
	sysActCnt := 0
	for i := range actions {
		if action.IsSystemAction(actions[i]) {
			if i != expectedStartIdx+sysActCnt {
				return errors.Wrapf(errInvalidSystemActionLayout, "the %d-th action should not be a system action", i)
			}
			if actions[i].Envelope.Proto().String() != postSystemActions[sysActCnt].Proto().String() {
				return errors.Wrapf(errInvalidSystemActionLayout, "the %d-th action is not the expected system action", i)
			}
			sysActCnt++
		}
	}
	if sysActCnt != len(postSystemActions) {
		return errors.Wrapf(errInvalidSystemActionLayout, "the number of system actions is incorrect, expected %d, got %d", len(postSystemActions), sysActCnt)
	}
	return nil
}

func (ws *workingSet) pickAndRunActions(
	ctx context.Context,
	ap actpool.ActPool,
	postSystemActions []*action.SealedEnvelope,
	allowedBlockGasResidue uint64,
) ([]*action.SealedEnvelope, error) {
	err := ws.validate(ctx)
	if err != nil {
		return nil, err
	}
	receipts := make([]*action.Receipt, 0)
	executedActions := make([]*action.SealedEnvelope, 0)
	reg := protocol.MustGetRegistry(ctx)

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
		blobLimit           = params.MaxBlobGasPerBlock / params.BlobTxBlobGasPerBlob
		deadline            *time.Time
		fullGas             = blkCtx.GasLimit
	)
	if ap != nil {
		if dl, ok := ctx.Deadline(); ok {
			deadline = &dl
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
			if nextAction.Gas() > blkCtx.GasLimit {
				actionIterator.PopAccount()
				continue
			}
			if blobCnt+uint64(len(nextAction.BlobHashes())) > uint64(blobLimit) {
				actionIterator.PopAccount()
				continue
			}
			if fCtx.UnfoldContainerBeforeValidate {
				if container, ok := nextAction.Envelope.(action.TxContainer); ok {
					if err := container.Unfold(nextAction, ctx, ws.checkContract); err != nil {
						log.L().Debug("failed to unfold tx container", zap.Uint64("height", ws.height), zap.Error(err))
						ap.DeleteAction(nextAction.SenderAddress())
						actionIterator.PopAccount()
						continue
					}
				}
			}
			if err := ws.txValidator.ValidateWithState(ctxWithBlockContext, nextAction); err != nil {
				log.L().Debug("failed to ValidateWithState", zap.Uint64("height", ws.height), zap.Error(err))
				ap.DeleteAction(nextAction.SenderAddress())
				if errors.Cause(err) != action.ErrNonceTooLow {
					actionIterator.PopAccount()
				}
				continue
			}
			actionCtx, err := withActionCtx(ctxWithBlockContext, nextAction)
			if err == nil {
				for _, p := range reg.All() {
					if validator, ok := p.(protocol.ActionValidator); ok {
						if err = validator.Validate(actionCtx, nextAction.Envelope, ws); err != nil {
							break
						}
					}
				}
			}
			caller := nextAction.SenderAddress()
			if err != nil {
				if caller == nil {
					return nil, errors.New("failed to get address")
				}
				log.L().Debug("failed to validate tx", zap.Uint64("height", ws.height), zap.Error(err))
				ap.DeleteAction(caller)
				actionIterator.PopAccount()
				continue
			}
			receipt, err := ws.runAction(actionCtx, nextAction)
			switch errors.Cause(err) {
			case nil:
				// do nothing
			case action.ErrGasLimit:
				actionIterator.PopAccount()
				continue
			case action.ErrChainID, errUnfoldTxContainer, errDeployerNotWhitelisted:
				log.L().Debug("runAction() failed", zap.Uint64("height", ws.height), zap.Error(err))
				ap.DeleteAction(caller)
				actionIterator.PopAccount()
				continue
			default:
				ap.DeleteAction(caller)
				actionIterator.PopAccount()
				nextActionHash, hashErr := nextAction.Hash()
				if hashErr != nil {
					return nil, errors.Wrapf(hashErr, "Failed to get hash for %x", nextActionHash)
				}
				return nil, errors.Wrapf(err, "Failed to update state changes for selp %x", nextActionHash)
			}
			blkCtx.GasLimit -= receipt.GasConsumed
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

	for _, selp := range postSystemActions {
		actionCtx, err := withActionCtx(ctxWithBlockContext, selp)
		if err != nil {
			return nil, err
		}
		receipt, err := ws.runAction(actionCtx, selp)
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

	return executedActions, ws.finalize()
}

func updateReceiptIndex(receipts []*action.Receipt) {
	var txIndex, logIndex uint32
	for _, r := range receipts {
		logIndex = r.UpdateIndex(txIndex, logIndex)
		txIndex++
	}
}

func (ws *workingSet) ValidateBlock(ctx context.Context, blk *block.Block) error {
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
		blobLimit := uint64(params.MaxBlobGasPerBlock / params.BlobTxBlobGasPerBlob)
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
	if err := ws.processWithCorrectOrder(ctx, blk.RunnableActions().Actions()); err != nil {
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

func (ws *workingSet) CreateBuilder(
	ctx context.Context,
	ap actpool.ActPool,
	postSystemActions []*action.SealedEnvelope,
	allowedBlockGasResidue uint64,
) (*block.Builder, error) {
	actions, err := ws.pickAndRunActions(ctx, ap, postSystemActions, allowedBlockGasResidue)
	if err != nil {
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
