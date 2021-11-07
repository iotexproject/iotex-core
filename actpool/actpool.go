// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"encoding/hex"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/tracer"
)

var (
	actpoolMtc = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iotex_actpool_rejection_metrics",
		Help: "actpool metrics.",
	}, []string{"type"})
)

func init() {
	prometheus.MustRegister(actpoolMtc)
}

// ActPool is the interface of actpool
type ActPool interface {
	action.SealedEnvelopeValidator
	// Reset resets actpool state
	Reset()
	// PendingActionMap returns an action map with all accepted actions
	PendingActionMap() map[string][]action.SealedEnvelope
	// Add adds an action into the pool after passing validation
	Add(ctx context.Context, act action.SealedEnvelope) error
	// GetPendingNonce returns pending nonce in pool given an account address
	GetPendingNonce(addr string) (uint64, error)
	// GetUnconfirmedActs returns unconfirmed actions in pool given an account address
	GetUnconfirmedActs(addr string) []action.SealedEnvelope
	// GetActionByHash returns the pending action in pool given action's hash
	GetActionByHash(hash hash.Hash256) (action.SealedEnvelope, error)
	// GetSize returns the act pool size
	GetSize() uint64
	// GetCapacity returns the act pool capacity
	GetCapacity() uint64
	// GetGasSize returns the act pool gas size
	GetGasSize() uint64
	// GetGasCapacity returns the act pool gas capacity
	GetGasCapacity() uint64
	// DeleteAction deletes an invalid action from pool
	DeleteAction(address.Address)
	// ReceiveBlock will be called when a new block is committed
	ReceiveBlock(*block.Block) error

	AddActionEnvelopeValidators(...action.SealedEnvelopeValidator)
}

// SortedActions is a slice of actions that implements sort.Interface to sort by Value.
type SortedActions []action.SealedEnvelope

func (p SortedActions) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p SortedActions) Len() int           { return len(p) }
func (p SortedActions) Less(i, j int) bool { return p[i].Nonce() < p[j].Nonce() }

// Option sets action pool construction parameter
type Option func(pool *actPool) error

// EnableExperimentalActions enables the action pool to take experimental actions
func EnableExperimentalActions() Option {
	return func(pool *actPool) error {
		pool.enableExperimentalActions = true
		return nil
	}
}

// actPool implements ActPool interface
type actPool struct {
	mutex                     sync.RWMutex
	cfg                       config.ActPool
	sf                        protocol.StateReader
	accountActs               map[string]ActQueue
	accountDesActs            map[string]map[hash.Hash256]action.SealedEnvelope
	allActions                map[hash.Hash256]action.SealedEnvelope
	gasInPool                 uint64
	actionEnvelopeValidators  []action.SealedEnvelopeValidator
	timerFactory              *prometheustimer.TimerFactory
	enableExperimentalActions bool
	senderBlackList           map[string]bool
}

// NewActPool constructs a new actpool
func NewActPool(sf protocol.StateReader, cfg config.ActPool, opts ...Option) (ActPool, error) {
	if sf == nil {
		return nil, errors.New("Try to attach a nil state reader")
	}

	senderBlackList := make(map[string]bool)
	for _, bannedSender := range cfg.BlackList {
		senderBlackList[bannedSender] = true
	}

	ap := &actPool{
		cfg:             cfg,
		sf:              sf,
		senderBlackList: senderBlackList,
		accountActs:     make(map[string]ActQueue),
		accountDesActs:  make(map[string]map[hash.Hash256]action.SealedEnvelope),
		allActions:      make(map[hash.Hash256]action.SealedEnvelope),
	}
	for _, opt := range opts {
		if err := opt(ap); err != nil {
			return nil, err
		}
	}
	timerFactory, err := prometheustimer.New(
		"iotex_action_pool_perf",
		"Performance of action pool",
		[]string{"type"},
		[]string{"default"},
	)
	if err != nil {
		return nil, err
	}
	ap.timerFactory = timerFactory
	return ap, nil
}

func (ap *actPool) AddActionEnvelopeValidators(fs ...action.SealedEnvelopeValidator) {
	ap.actionEnvelopeValidators = append(ap.actionEnvelopeValidators, fs...)
}

// Reset resets actpool state
// Step I: remove all the actions in actpool that have already been committed to block
// Step II: update pending balance of each account if it still exists in pool
// Step III: update queue's status in each account and remove invalid actions following queue's update
// Specifically, first reset the pending nonce based on confirmed nonce in order to prevent omitting reevaluation of
// unconfirmed but pending actions in pool after update of pending balance
// Then starting from the current confirmed nonce, iteratively update pending nonce if nonces are consecutive and pending
// balance is sufficient, and remove all the subsequent actions once the pending balance becomes insufficient
func (ap *actPool) Reset() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	ap.reset()
}

func (ap *actPool) ReceiveBlock(*block.Block) error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	ap.reset()
	return nil
}

// PendingActionIterator returns an action interator with all accepted actions
func (ap *actPool) PendingActionMap() map[string][]action.SealedEnvelope {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	// Remove the actions that are already timeout
	ap.reset()

	actionMap := make(map[string][]action.SealedEnvelope)
	for from, queue := range ap.accountActs {
		actionMap[from] = append(actionMap[from], queue.PendingActs()...)
	}
	return actionMap
}

func (ap *actPool) Add(ctx context.Context, act action.SealedEnvelope) error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	ctx, span := tracer.NewSpan(ctx, "actPool.Add")
	defer span.End()

	// Reject action if pool space is full
	if uint64(len(ap.allActions)) >= ap.cfg.MaxNumActsPerPool {
		actpoolMtc.WithLabelValues("overMaxNumActsPerPool").Inc()
		return action.ErrTxPoolOverflow
	}
	intrinsicGas, err := act.IntrinsicGas()
	if err != nil {
		actpoolMtc.WithLabelValues("failedGetIntrinsicGas").Inc()
		return errors.Wrap(err, "failed to get action's intrinsic gas")
	}
	if ap.gasInPool+intrinsicGas > ap.cfg.MaxGasLimitPerPool {
		actpoolMtc.WithLabelValues("overMaxGasLimitPerPool").Inc()
		return action.ErrGasLimit
	}
	hash, err := act.Hash()
	if err != nil {
		return err
	}
	// Reject action if it already exists in pool
	if _, exist := ap.allActions[hash]; exist {
		actpoolMtc.WithLabelValues("existedAction").Inc()
		return action.ErrExistedInPool
	}
	// Reject action if the gas price is lower than the threshold
	if act.GasPrice().Cmp(ap.cfg.MinGasPrice()) < 0 {
		actpoolMtc.WithLabelValues("gasPriceLower").Inc()
		log.L().Info("action rejected due to low gas price",
			zap.String("act hash", hex.EncodeToString(hash[:])),
			zap.String("gas price", act.GasPrice().String()))
		return action.ErrUnderpriced
	}
	if err := ap.validate(ctx, act); err != nil {
		return err
	}

	caller := act.SrcPubkey().Address()
	if caller == nil {
		return action.ErrAddress
	}
	return ap.enqueueAction(caller, act, hash, act.Nonce())
}

// GetPendingNonce returns pending nonce in pool or confirmed nonce given an account address
func (ap *actPool) GetPendingNonce(addr string) (uint64, error) {
	addrStr, err := address.FromString(addr)
	if err != nil {
		return 0, err
	}
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	confirmedState, err := accountutil.AccountState(ap.sf, addrStr)
	if err != nil {
		return 0, err
	}
	return confirmedState.Nonce + 1, err
}

// GetUnconfirmedActs returns unconfirmed actions in pool given an account address
func (ap *actPool) GetUnconfirmedActs(addr string) []action.SealedEnvelope {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()
	var ret []action.SealedEnvelope
	if queue, ok := ap.accountActs[addr]; ok {
		ret = queue.AllActs()
	}
	if desMap, ok := ap.accountDesActs[addr]; ok {
		if desMap != nil {
			sortActions := make(SortedActions, 0)
			for _, v := range desMap {
				sortActions = append(sortActions, v)
			}
			sort.Stable(sortActions)
			ret = append(ret, sortActions...)
		}
	}
	return ret
}

// GetActionByHash returns the pending action in pool given action's hash
func (ap *actPool) GetActionByHash(hash hash.Hash256) (action.SealedEnvelope, error) {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	act, ok := ap.allActions[hash]
	if !ok {
		return action.SealedEnvelope{}, errors.Wrapf(action.ErrNotFound, "action hash %x does not exist in pool", hash)
	}
	return act, nil
}

// GetSize returns the act pool size
func (ap *actPool) GetSize() uint64 {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	return uint64(len(ap.allActions))
}

// GetCapacity returns the act pool capacity
func (ap *actPool) GetCapacity() uint64 {
	return ap.cfg.MaxNumActsPerPool
}

// GetGasSize returns the act pool gas size
func (ap *actPool) GetGasSize() uint64 {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	return ap.gasInPool
}

// GetGasCapacity returns the act pool gas capacity
func (ap *actPool) GetGasCapacity() uint64 {
	return ap.cfg.MaxGasLimitPerPool
}

func (ap *actPool) Validate(ctx context.Context, selp action.SealedEnvelope) error {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()
	return ap.validate(ctx, selp)
}

func (ap *actPool) DeleteAction(caller address.Address) {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()
	pendingActs := ap.accountActs[caller.String()].AllActs()
	ap.removeInvalidActs(pendingActs)
	delete(ap.accountActs, caller.String())
}

func (ap *actPool) validate(ctx context.Context, selp action.SealedEnvelope) error {
	span := tracer.SpanFromContext(ctx)
	span.AddEvent("actPool.validate")
	defer span.End()

	caller := selp.SrcPubkey().Address()
	if caller == nil {
		return errors.New("failed to get address")
	}
	if _, ok := ap.senderBlackList[caller.String()]; ok {
		actpoolMtc.WithLabelValues("blacklisted").Inc()
		return errors.Wrap(action.ErrAddress, "action source address is blacklisted")
	}
	// if already validated
	selpHash, err := selp.Hash()
	if err != nil {
		return err
	}
	if _, ok := ap.allActions[selpHash]; ok {
		return nil
	}
	for _, ev := range ap.actionEnvelopeValidators {
		if err := ev.Validate(ctx, selp); err != nil {
			return err
		}
	}

	return nil
}

//======================================
// private functions
//======================================
func (ap *actPool) enqueueAction(addr address.Address, act action.SealedEnvelope, actHash hash.Hash256, actNonce uint64) error {
	confirmedState, err := accountutil.AccountState(ap.sf, addr)
	if err != nil {
		actpoolMtc.WithLabelValues("failedToGetNonce").Inc()
		return errors.Wrapf(err, "failed to get sender's nonce for action %x", actHash)
	}
	confirmedNonce := confirmedState.Nonce
	if actNonce <= confirmedNonce {
		return action.ErrNonceTooLow
	}
	sender := addr.String()
	queue := ap.accountActs[sender]
	if queue == nil {
		queue = NewActQueue(ap, sender, WithTimeOut(ap.cfg.ActionExpiry))
		ap.accountActs[sender] = queue

		// Initialize pending nonce for new account
		pendingNonce := confirmedNonce + 1
		queue.SetPendingNonce(pendingNonce)
		// Initialize balance for new account
		state, err := accountutil.AccountState(ap.sf, addr)
		if err != nil {
			actpoolMtc.WithLabelValues("failedToGetBalance").Inc()
			return errors.Wrapf(err, "failed to get sender's balance for action %x", actHash)
		}
		queue.SetPendingBalance(state.Balance)
	}

	if actNonce-confirmedNonce >= ap.cfg.MaxNumActsPerAcct+1 {
		// Nonce exceeds current range
		log.L().Debug("Rejecting action because nonce is too large.",
			log.Hex("hash", actHash[:]),
			zap.Uint64("startNonce", confirmedNonce+1),
			zap.Uint64("actNonce", actNonce))
		actpoolMtc.WithLabelValues("nonceTooLarge").Inc()
		return action.ErrNonceTooHigh
	}

	cost, err := act.Cost()
	if err != nil {
		actpoolMtc.WithLabelValues("failedToGetCost").Inc()
		return errors.Wrapf(err, "failed to get cost of action %x", actHash)
	}
	if queue.PendingBalance().Cmp(cost) < 0 {
		// Pending balance is insufficient
		actpoolMtc.WithLabelValues("insufficientBalance").Inc()
		log.L().Info("insufficient balance for action",
			zap.String("act hash", hex.EncodeToString(actHash[:])),
			zap.String("cost", cost.String()),
			zap.String("pending balance", queue.PendingBalance().String()),
			zap.String("sender", sender),
		)
		return action.ErrInsufficientFunds
	}

	if err := queue.Put(act); err != nil {
		actpoolMtc.WithLabelValues("failedPutActQueue").Inc()
		log.L().Info("failed put action into ActQueue",
			zap.String("act hash", hex.EncodeToString(actHash[:])))
		return err
	}
	ap.allActions[actHash] = act

	//add actions to destination map
	desAddress, ok := act.Destination()
	if ok && !strings.EqualFold(sender, desAddress) {
		desQueue := ap.accountDesActs[desAddress]
		if desQueue == nil {
			ap.accountDesActs[desAddress] = make(map[hash.Hash256]action.SealedEnvelope)
		}
		ap.accountDesActs[desAddress][actHash] = act
	}

	intrinsicGas, _ := act.IntrinsicGas()
	ap.gasInPool += intrinsicGas
	// If the pending nonce equals this nonce, update queue
	nonce := queue.PendingNonce()
	if actNonce == nonce {
		ap.updateAccount(sender)
	}
	return nil
}

// removeConfirmedActs removes processed (committed to block) actions from pool
func (ap *actPool) removeConfirmedActs() {
	for from, queue := range ap.accountActs {
		addr, _ := address.FromString(from)
		confirmedState, err := accountutil.AccountState(ap.sf, addr)
		if err != nil {
			log.L().Error("Error when removing confirmed actions", zap.Error(err))
			return
		}
		pendingNonce := confirmedState.Nonce + 1
		// Remove all actions that are committed to new block
		acts := queue.FilterNonce(pendingNonce)
		ap.removeInvalidActs(acts)
		//del actions in destination map
		ap.deleteAccountDestinationActions(acts...)
		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			delete(ap.accountActs, from)
		}
	}
}

func (ap *actPool) removeInvalidActs(acts []action.SealedEnvelope) {
	for _, act := range acts {
		hash, err := act.Hash()
		if err != nil {
			log.L().Debug("Skipping action due to hash error", zap.Error(err))
			continue
		}
		log.L().Debug("Removed invalidated action.", log.Hex("hash", hash[:]))
		delete(ap.allActions, hash)
		intrinsicGas, _ := act.IntrinsicGas()
		ap.subGasFromPool(intrinsicGas)
		//del actions in destination map
		ap.deleteAccountDestinationActions(act)
	}
}

// deleteAccountDestinationActions just for destination map
func (ap *actPool) deleteAccountDestinationActions(acts ...action.SealedEnvelope) {
	for _, act := range acts {
		hash, err := act.Hash()
		if err != nil {
			log.L().Debug("Skipping action due to hash error", zap.Error(err))
			continue
		}
		desAddress, ok := act.Destination()
		if ok {
			dst := ap.accountDesActs[desAddress]
			if dst != nil {
				delete(dst, hash)
			}
		}
	}
}

// updateAccount updates queue's status and remove invalidated actions from pool if necessary
func (ap *actPool) updateAccount(sender string) {
	queue := ap.accountActs[sender]
	acts := queue.UpdateQueue(queue.PendingNonce())
	if len(acts) > 0 {
		ap.removeInvalidActs(acts)
	}
	// Delete the queue entry if it becomes empty
	if queue.Empty() {
		delete(ap.accountActs, sender)
	}
}

func (ap *actPool) reset() {
	timer := ap.timerFactory.NewTimer("reset")
	defer timer.End()

	// Remove confirmed actions in actpool
	ap.removeConfirmedActs()
	for from, queue := range ap.accountActs {
		// Reset pending balance for each account
		addr, _ := address.FromString(from)
		state, err := accountutil.AccountState(ap.sf, addr)
		if err != nil {
			log.L().Error("Error when resetting actpool state.", zap.Error(err))
			return
		}
		queue.SetPendingBalance(state.Balance)

		// Reset pending nonce and remove invalid actions for each account
		confirmedNonce := state.Nonce
		pendingNonce := confirmedNonce + 1
		queue.SetPendingNonce(pendingNonce)
		ap.updateAccount(from)
	}
}

func (ap *actPool) subGasFromPool(gas uint64) {
	if ap.gasInPool < gas {
		ap.gasInPool = 0
		return
	}
	ap.gasInPool -= gas
}
