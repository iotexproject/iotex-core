// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txpool

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/statefactory"
)

const (
	// GlobalSlots indicate maximum transactions the whole actpool can hold
	GlobalSlots = 5120
	// AccountSlots indicate maximum transactions an account can hold
	AccountSlots = 80
)

var (
	// ErrInvalidAddr is the error that the address format is invalid, cannot be decoded
	ErrInvalidAddr = errors.New("address format is invalid")
	// ErrActPool indicates the error of actpool
	ErrActPool = errors.New("invalid actpool")
	// ErrNonce indicates the error of nonce
	ErrNonce = errors.New("invalid nonce")
	// ErrBalance indicates the error of balance
	ErrBalance = errors.New("invalid balance")
)

// ActPool is the interface of actpool
type ActPool interface {
	// Reset resets actpool state
	Reset()
	// PickTxs returns all currently accepted transactions in actpool
	PickTxs() (map[string][]*action.Transfer, error)
	// AddTx adds a transaction into the pool after validation
	AddTx(tx *action.Transfer) error
	// AddVote adds a vote into pool after validation
	AddVote(vote *action.Vote) error
}

// actPool implements ActPool interface
type actPool struct {
	mutex      sync.RWMutex
	sf         statefactory.StateFactory
	p2p        *network.Overlay
	accountTxs map[string]TxQueue
	allTxs     map[common.Hash32B]*action.Transfer
}

// NewActPool constructs a new actpool
func NewActPool(sf statefactory.StateFactory, p2p *network.Overlay) ActPool {
	ap := &actPool{
		sf:         sf,
		p2p:        p2p,
		accountTxs: make(map[string]TxQueue),
		allTxs:     make(map[common.Hash32B]*action.Transfer),
	}
	return ap
}

// Reset resets actpool state
// Step I: remove all the transactions in actpool that have already been committed to block
// Step II: update pending balance of each account if it still exists in pool
// Step III: update pending nonce and confirmed nonce in each account if it still exists in pool
// Specifically, first synchronize old confirmed nonce with committed nonce in order to prevent omitting reevaluation of
// uncommitted but confirmed Txs in pool after update of pending balance
// Then starting from the current committed nonce, iteratively update pending nonce if nonces are consecutive as well as
// confirmed nonce if pending balance is sufficient
func (ap *actPool) Reset() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	// Remove committed transactions in actpool
	ap.removeCommittedTxs()
	for from, queue := range ap.accountTxs {
		// Reset pending balance for each account
		balance, err := ap.sf.Balance(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when resetting actpool state")
			return
		}
		queue.SetPendingBalance(balance)

		// Reset confirmed nonce and pending nonce for each account
		committedNonce, err := ap.sf.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when resetting Tx")
			return
		}
		confirmedNonce := committedNonce + 1
		queue.SetConfirmedNonce(confirmedNonce)
		queue.UpdateNonce(confirmedNonce)
	}
}

// PickTxs returns all currently accepted transactions for all accounts
func (ap *actPool) PickTxs() (map[string][]*action.Transfer, error) {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	pending := make(map[string][]*action.Transfer)
	for from, queue := range ap.accountTxs {
		pending[from] = queue.ConfirmedTxs()
	}
	return pending, nil
}

// validateTx checks whether a transaction is valid
func (ap *actPool) validateTx(tx *action.Transfer) error {
	// Reject oversized transaction
	if tx.TotalSize() > 32*1024 {
		return errors.Wrapf(ErrActPool, "oversized data")
	}
	// Reject transaction of negative amount
	if tx.Amount.Sign() < 0 {
		return errors.Wrapf(ErrBalance, "negative value")
	}
	// check sender
	pkhash := iotxaddress.GetPubkeyHash(tx.Sender)
	if pkhash == nil {
		return ErrInvalidAddr
	}
	// Reject transaction if nonce is too low
	committedNonce, err := ap.sf.Nonce(tx.Sender)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating Tx")
		return err
	}
	confirmedNonce := committedNonce + 1
	if confirmedNonce > tx.Nonce {
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}

// AddTx inserts a new transaction into account queue if it passes validation
func (ap *actPool) AddTx(tx *action.Transfer) error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	hash := tx.Hash()
	// Reject transaction if it already exists in pool
	if ap.allTxs[hash] != nil {
		logger.Info().
			Bytes("hash", hash[:]).
			Msg("Rejecting existed transaction")
		return fmt.Errorf("existed transaction: %x", hash)
	}
	// Reject transaction if it fails validation
	if err := ap.validateTx(tx); err != nil {
		logger.Info().
			Bytes("hash", hash[:]).
			Err(err).
			Msg("Rejecting invalid transaction")
		return err
	}
	// Reject transaction if pool space is full
	if uint64(len(ap.allTxs)) >= GlobalSlots {
		logger.Info().
			Bytes("hash", hash[:]).
			Msg("Rejecting transaction due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for transaction")
	}
	// check sender
	pkhash := iotxaddress.GetPubkeyHash(tx.Sender)
	if pkhash == nil {
		return ErrInvalidAddr
	}
	queue := ap.accountTxs[tx.Sender]
	if queue == nil {
		queue = NewTxQueue()
		ap.accountTxs[tx.Sender] = queue
		// Initialize pending nonce and confirmed nonce for new account
		committedNonce, err := ap.sf.Nonce(tx.Sender)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding Tx")
			return err
		}
		confirmedNonce := committedNonce + 1
		queue.SetPendingNonce(confirmedNonce)
		queue.SetConfirmedNonce(confirmedNonce)
		// Initialize balance for new account
		balance, err := ap.sf.Balance(tx.Sender)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding Tx")
			return err
		}
		queue.SetPendingBalance(balance)
	}

	if queue.Overlaps(tx) {
		// Nonce already exists
		logger.Info().
			Bytes("hash", hash[:]).
			Msg("Rejecting transaction because replacement Tx is not supported")
		return errors.Wrapf(ErrNonce, "duplicate nonce")
	}

	if queue.Len() >= AccountSlots {
		logger.Info().
			Bytes("hash", hash[:]).
			Msg("Rejecting transaction due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for transaction")
	}
	queue.Put(tx)
	ap.allTxs[hash] = tx
	// If the pending nonce equals this nonce, update pending nonce
	nonce := queue.PendingNonce()
	if tx.Nonce == nonce {
		queue.UpdateNonce(tx.Nonce)
	}
	return nil
}

// AddVote inserts a new vote if it passes validation
func (ap *actPool) AddVote(tx *action.Vote) error {
	// TODO: Implement AddVote
	return nil
}

// removeCommittedTxs removes processed (committed to block) transactions from pool
func (ap *actPool) removeCommittedTxs() {
	for from, queue := range ap.accountTxs {
		committedNonce, err := ap.sf.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when removing commited Txs")
			return
		}
		confirmedNonce := committedNonce + 1
		// Remove all transactions that are committed to new block
		for _, tx := range queue.FilterNonce(confirmedNonce) {
			hash := tx.Hash()
			logger.Info().
				Bytes("hash", hash[:]).
				Msg("Removed committed transaction")
			delete(ap.allTxs, hash)
		}
		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			delete(ap.accountTxs, from)
		}
	}
}
