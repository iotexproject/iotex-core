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

	trx "github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/statefactory"
	"github.com/iotexproject/iotex-core/trie"
)

const (
	GlobalSlots  = 5120 // Maximum transactions the whole actpool can hold
	AccountSlots = 80   // Maximum transactions an account can hold
)

var (
	isTestnet = true

	chainid = []byte{0x00, 0x00, 0x00, 0x01}

	hashToAddr = make(map[common.Hash32B]*iotxaddress.Address)

	ErrActPool = errors.New("invalid actpool")
	ErrNonce   = errors.New("invalid nonce")
	ErrBalance = errors.New("invalid balance")
)

// ActPool is the interface of actpool
type ActPool interface {
	// Reset resets actpool state
	Reset()
	// PickTxs returns all currently accepted transactions in actpool
	PickTxs() (map[common.Hash32B][]*trx.Tx, error)
	// AddTx adds a transaction into the pool after validation
	AddTx(tx *trx.Tx) error
}

// actPool implements ActPool interface
type actPool struct {
	mutex      sync.RWMutex
	sf         statefactory.StateFactory
	accountTxs map[common.Hash32B]TxQueue
	allTxs     map[common.Hash32B]*trx.Tx
}

// NewActPool constructs a new actpool
func NewActPool(trie trie.Trie) ActPool {
	ap := &actPool{
		sf:         statefactory.NewStateFactory(trie),
		accountTxs: make(map[common.Hash32B]TxQueue),
		allTxs:     make(map[common.Hash32B]*trx.Tx),
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
	for addrHash, queue := range ap.accountTxs {
		from := hashToAddr[addrHash]
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
		queue.SetConfirmedNonce(committedNonce)
		queue.UpdatePendingNonce(committedNonce, true)
	}
}

// PickTxs returns all currently accepted transactions for all accounts
func (ap *actPool) PickTxs() (map[common.Hash32B][]*trx.Tx, error) {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	pending := make(map[common.Hash32B][]*trx.Tx)
	for addrHash, queue := range ap.accountTxs {
		pending[addrHash] = queue.ConfirmedTxs()
	}
	return pending, nil
}

// validateTx checks whether a transaction is valid
func (ap *actPool) validateTx(tx *trx.Tx) error {
	// Reject oversized transaction
	if tx.TotalSize() > 32*1024 {
		return errors.Wrapf(ErrActPool, "oversized data")
	}
	// Reject transaction of negative amount
	if tx.Amount.Sign() < 0 {
		return errors.Wrapf(ErrBalance, "negative value")
	}

	from, err := iotxaddress.GetAddress(tx.SenderPublicKey, isTestnet, chainid)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating Tx")
		return err
	}
	// Reject transaction if nonce is too low
	nonce, err := ap.getNonce(from)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating Tx")
		return err
	}
	if nonce > tx.Nonce {
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}

// AddTx inserts a new transaction into account queue if it passes validation
func (ap *actPool) AddTx(tx *trx.Tx) error {
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
	from, err := iotxaddress.GetAddress(tx.SenderPublicKey, isTestnet, chainid)
	if err != nil {
		logger.Error().Err(err).Msg("Error when adding Tx")
		return err
	}
	addrHash := from.HashAddress()
	queue := ap.accountTxs[addrHash]

	if queue == nil {
		queue = NewTxQueue()
		ap.accountTxs[addrHash] = queue
		hashToAddr[addrHash] = from
		// Initialize pending nonce for new account
		nonce, err := ap.sf.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding Tx")
			return err
		}
		queue.SetPendingNonce(nonce)
		// Initialize balance for new account
		balance, err := ap.sf.Balance(from)
		if err != nil {
			glog.Errorf("Error when adding Tx: %v\n", err)
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
		// Flag indicating whether we need to update confirmed nonce as well
		updateConfirmedNonce := nonce == queue.ConfirmedNonce()
		queue.UpdatePendingNonce(tx.Nonce, updateConfirmedNonce)
	}
	return nil
}

// removeCommittedTxs removes processed (committed to block) transactions from pool
func (ap *actPool) removeCommittedTxs() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	for addrHash, queue := range ap.accountTxs {
		committedNonce, err := ap.sf.Nonce(hashToAddr[addrHash])
		if err != nil {
			logger.Error().Err(err).Msg("Error when removing commited Txs")
			return
		}
		// Remove all transactions that are committed to new block
		for _, tx := range queue.FilterNonce(committedNonce) {
			hash := tx.Hash()
			logger.Info().
				Bytes("hash", hash[:]).
				Msg("Removed committed transaction")
			delete(ap.allTxs, hash)
		}
		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			delete(ap.accountTxs, addrHash)
		}
	}
}

func (ap *actPool) getNonce(addr *iotxaddress.Address) (uint64, error) {
	addrHash := addr.HashAddress()
	if queue, ok := ap.accountTxs[addrHash]; ok {
		return queue.PendingNonce(), nil
	}
	return ap.sf.Nonce(addr)
}
