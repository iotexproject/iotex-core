// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

var (
	_stateDBMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_state_db",
			Help: "IoTeX State DB",
		},
		[]string{"type"},
	)

	errUnsupportWeb3Staking = errors.New("unsupported web3 staking")
)

func init() {
	prometheus.MustRegister(_stateDBMtc)
}

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
	if protocol.MustGetFeatureCtx(ctx).CorrectTxLogIndex {
		updateReceiptIndex(receipts)
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
		return nil, action.ErrGasLimit
	}
	if !protocol.MustGetFeatureCtx(ctx).EnableWeb3Staking && isWeb3StakingAction(elp) {
		return nil, errUnsupportWeb3Staking
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
	if err := ws.store.Commit(); err != nil {
		return err
	}
	if err := protocolCommit(ctx, ws); err != nil {
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
	values, err := ws.store.States(cfg.Namespace, cfg.Keys)
	if err != nil {
		return 0, nil, err
	}
	return ws.height, state.NewIterator(values), nil
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
		addr, _ := address.FromString(srcAddr)
		confirmedState, err := accountutil.AccountState(ws, addr)
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
					"the %d nonce %d of address %s (confirmed nonce %d) is not continuously increasing",
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
	ctxWithBlockContext := ctx
	if ap != nil {
		actionIterator := actioniterator.NewActionIterator(ap.PendingActionMap())
		for {
			nextAction, ok := actionIterator.Next()
			if !ok {
				break
			}
			if nextAction.GasLimit() > blkCtx.GasLimit {
				actionIterator.PopAccount()
				continue
			}
			actionCtx, err := withActionCtx(ctxWithBlockContext, nextAction)
			if err == nil {
				for _, p := range reg.All() {
					if validator, ok := p.(protocol.ActionValidator); ok {
						if err = validator.Validate(actionCtx, nextAction.Action(), ws); err != nil {
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
			receipt, err := ws.runAction(actionCtx, nextAction)
			switch errors.Cause(err) {
			case nil:
				// do nothing
			case action.ErrChainID, errUnsupportWeb3Staking:
				continue
			case action.ErrGasLimit:
				actionIterator.PopAccount()
				continue
			default:
				nextActionHash, hashErr := nextAction.Hash()
				if hashErr != nil {
					return nil, errors.Wrapf(hashErr, "Failed to get hash for %x", nextActionHash)
				}
				return nil, errors.Wrapf(err, "Failed to update state changes for selp %x", nextActionHash)
			}
			if receipt != nil {
				blkCtx.GasLimit -= receipt.GasConsumed
				ctxWithBlockContext = protocol.WithBlockCtx(ctx, blkCtx)
				receipts = append(receipts, receipt)
			}
			executedActions = append(executedActions, nextAction)

			// To prevent loop all actions in act_pool, we stop processing action when remaining gas is below
			// than certain threshold
			if blkCtx.GasLimit < allowedBlockGasResidue {
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
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, selp)
	}
	if protocol.MustGetFeatureCtx(ctx).CorrectTxLogIndex {
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
	if !blk.VerifyDeltaStateDigest(digest) {
		return block.ErrDeltaStateMismatch
	}
	if !blk.VerifyReceiptRoot(calculateReceiptRoot(ws.receipts)) {
		return block.ErrReceiptRootMismatch
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

func isWeb3StakingAction(selp action.SealedEnvelope) bool {
	if selp.Encoding() == uint32(iotextypes.Encoding_ETHEREUM_RLP) {
		act := selp.Action()
		switch act.(type) {
		case *action.CreateStake,
			*action.DepositToStake,
			*action.ChangeCandidate,
			*action.Unstake,
			*action.WithdrawStake,
			*action.Restake,
			*action.TransferStake,
			*action.CandidateRegister,
			*action.CandidateUpdate:
			return true
		default:
			return false
		}
	}
	return false
}
