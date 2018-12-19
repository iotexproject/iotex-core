// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

// ActPool is the interface of actpool
type ActPool interface {
	// Reset resets actpool state
	Reset()
	// PickActs returns all currently accepted actions in actpool
	PickActs() []action.SealedEnvelope
	// PendingActionMap returns an action map with all accepted actions
	PendingActionMap() map[string][]action.SealedEnvelope
	// Add adds an action into the pool after passing validation
	Add(act action.SealedEnvelope) error
	// GetPendingNonce returns pending nonce in pool given an account address
	GetPendingNonce(addr string) (uint64, error)
	// GetUnconfirmedActs returns unconfirmed actions in pool given an account address
	GetUnconfirmedActs(addr string) []action.SealedEnvelope
	// GetActionByHash returns the pending action in pool given action's hash
	GetActionByHash(hash hash.Hash32B) (action.SealedEnvelope, error)
	// GetSize returns the act pool size
	GetSize() uint64
	// GetCapacity returns the act pool capacity
	GetCapacity() uint64
	// AddActionValidators add validators
	AddActionValidators(...protocol.ActionValidator)

	AddActionEnvelopeValidators(...protocol.ActionEnvelopeValidator)
}

// actPool implements ActPool interface
type actPool struct {
	mutex                    sync.RWMutex
	cfg                      config.ActPool
	bc                       blockchain.Blockchain
	accountActs              map[string]ActQueue
	allActions               map[hash.Hash32B]action.SealedEnvelope
	actionEnvelopeValidators []protocol.ActionEnvelopeValidator
	validators               []protocol.ActionValidator
}

// NewActPool constructs a new actpool
func NewActPool(bc blockchain.Blockchain, cfg config.ActPool) (ActPool, error) {
	if bc == nil {
		return nil, errors.New("Try to attach a nil blockchain")
	}
	ap := &actPool{
		cfg:         cfg,
		bc:          bc,
		accountActs: make(map[string]ActQueue),
		allActions:  make(map[hash.Hash32B]action.SealedEnvelope),
	}
	return ap, nil
}

// AddActionValidators add validators
func (ap *actPool) AddActionValidators(validators ...protocol.ActionValidator) {
	ap.validators = append(ap.validators, validators...)
}

func (ap *actPool) AddActionEnvelopeValidators(fs ...protocol.ActionEnvelopeValidator) {
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

	// Remove confirmed actions in actpool
	ap.removeConfirmedActs()
	for from, queue := range ap.accountActs {
		// Reset pending balance for each account
		balance, err := ap.bc.Balance(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when resetting actpool state")
			return
		}
		queue.SetPendingBalance(balance)

		// Reset pending nonce and remove invalid actions for each account
		confirmedNonce, err := ap.bc.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when resetting actpool state")
			return
		}
		pendingNonce := confirmedNonce + 1
		queue.SetStartNonce(pendingNonce)
		queue.SetPendingNonce(pendingNonce)
		ap.updateAccount(from)
	}
}

// PickActs returns all currently accepted transfers and votes for all accounts
func (ap *actPool) PickActs() []action.SealedEnvelope {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	numActs := uint64(0)
	actions := make([]action.SealedEnvelope, 0)
	for _, queue := range ap.accountActs {
		for _, act := range queue.PendingActs() {
			actions = append(actions, act)
			numActs++
			if ap.cfg.MaxNumActsToPick > 0 && numActs >= ap.cfg.MaxNumActsToPick {
				logger.Debug().
					Uint64("limit", ap.cfg.MaxNumActsToPick).
					Msg("reach the max number of actions to pick")
				return actions
			}
		}
	}
	return actions
}

// PendingActionIterator returns an action interator with all accepted actions
func (ap *actPool) PendingActionMap() map[string][]action.SealedEnvelope {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	actionMap := make(map[string][]action.SealedEnvelope)
	for from, queue := range ap.accountActs {
		actionMap[from] = append(actionMap[from], queue.PendingActs()...)
	}
	return actionMap
}

func (ap *actPool) Add(act action.SealedEnvelope) error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	// Reject action if pool space is full
	if uint64(len(ap.allActions)) >= ap.cfg.MaxNumActsPerPool {
		return errors.Wrap(action.ErrActPool, "insufficient space for action")
	}
	hash := act.Hash()
	// Reject action if it already exists in pool
	if _, exist := ap.allActions[hash]; exist {
		return fmt.Errorf("reject existed action: %x", hash)
	}

	// envelope validation
	for _, validator := range ap.actionEnvelopeValidators {
		if err := validator.Validate(context.Background(), act); err != nil {
			return errors.Wrapf(err, "reject invalid action: %x", hash)
		}
	}
	// Reject action if it's invalid
	for _, validator := range ap.validators {
		if err := validator.Validate(context.Background(), act.Action()); err != nil {
			return errors.Wrapf(err, "reject invalid action: %x", hash)
		}
	}
	return ap.enqueueAction(act.SrcAddr(), act, hash, act.Nonce())
}

// GetPendingNonce returns pending nonce in pool or confirmed nonce given an account address
func (ap *actPool) GetPendingNonce(addr string) (uint64, error) {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	confirmedNonce, err := ap.bc.Nonce(addr)
	pendingNonce := confirmedNonce + 1
	return pendingNonce, err
}

// GetUnconfirmedActs returns unconfirmed actions in pool given an account address
func (ap *actPool) GetUnconfirmedActs(addr string) []action.SealedEnvelope {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	if queue, ok := ap.accountActs[addr]; ok {
		return queue.AllActs()
	}
	return make([]action.SealedEnvelope, 0)
}

// GetActionByHash returns the pending action in pool given action's hash
func (ap *actPool) GetActionByHash(hash hash.Hash32B) (action.SealedEnvelope, error) {
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	act, ok := ap.allActions[hash]
	if !ok {
		return action.SealedEnvelope{}, errors.Wrapf(action.ErrHash, "action hash %x does not exist in pool", hash)
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
	ap.mutex.RLock()
	defer ap.mutex.RUnlock()

	return ap.cfg.MaxNumActsPerPool
}

//======================================
// private functions
//======================================
func (ap *actPool) enqueueAction(sender string, act action.SealedEnvelope, hash hash.Hash32B, actNonce uint64) error {
	queue := ap.accountActs[sender]
	if queue == nil {
		queue = NewActQueue()
		ap.accountActs[sender] = queue
		confirmedNonce, err := ap.bc.Nonce(sender)
		if err != nil {
			return errors.Wrapf(err, "failed to get sender's nonce for action %x", hash)
		}
		// Initialize pending nonce for new account
		pendingNonce := confirmedNonce + 1
		queue.SetPendingNonce(pendingNonce)
		queue.SetStartNonce(pendingNonce)
		// Initialize balance for new account
		balance, err := ap.bc.Balance(sender)
		if err != nil {
			return errors.Wrapf(err, "failed to get sender's balance for action %x", hash)
		}
		queue.SetPendingBalance(balance)
	}
	if queue.Overlaps(act) {
		// Nonce already exists
		return errors.Wrapf(action.ErrNonce, "duplicate nonce for action %x", hash)
	}

	if actNonce-queue.StartNonce() >= ap.cfg.MaxNumActsPerAcct {
		// Nonce exceeds current range
		logger.Debug().
			Hex("hash", hash[:]).
			Uint64("startNonce", queue.StartNonce()).Uint64("actNonce", actNonce).
			Msg("Rejecting action because nonce is too large")
		return errors.Wrapf(action.ErrNonce, "nonce too large")
	}

	cost, err := act.Cost()
	if err != nil {
		return errors.Wrapf(err, "failed to get cost of action %x", hash)
	}
	if queue.PendingBalance().Cmp(cost) < 0 {
		// Pending balance is insufficient
		return errors.Wrapf(action.ErrBalance, "insufficient balance for action %x", hash)
	}

	if err := queue.Put(act); err != nil {
		return errors.Wrapf(err, "cannot put action %x into ActQueue", hash)
	}
	ap.allActions[hash] = act
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
		confirmedNonce, err := ap.bc.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when removing confirmed actions")
			return
		}
		pendingNonce := confirmedNonce + 1
		// Remove all actions that are committed to new block
		acts := queue.FilterNonce(pendingNonce)
		ap.removeInvalidActs(acts)

		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			delete(ap.accountActs, from)
		}
	}
}

func (ap *actPool) removeInvalidActs(acts []action.SealedEnvelope) {
	for _, act := range acts {
		hash := act.Hash()
		logger.Debug().
			Hex("hash", hash[:]).
			Msg("Removed invalidated action")
		delete(ap.allActions, hash)
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
