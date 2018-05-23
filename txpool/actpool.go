// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package txpool

import (
	"fmt"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	trx "github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
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
	pendingSF  statefactory.StateFactory // Pending state tracking virtual nonces
	accountTxs map[common.Hash32B]TxQueue
	allTxs     map[common.Hash32B]*trx.Tx
}

// NewActPool constructs a new actpool
func NewActPool(trie trie.Trie) ActPool {
	ap := &actPool{
		pendingSF:  statefactory.NewVirtualStateFactory(trie),
		accountTxs: make(map[common.Hash32B]TxQueue),
		allTxs:     make(map[common.Hash32B]*trx.Tx),
	}
	return ap
}

func (ap *actPool) Reset() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	// Remove committed transactions in actpool
	ap.removeCommittedTxs()
	// Reset pending nonce for each account
	for addrHash, queue := range ap.accountTxs {
		txs := queue.AcceptedTxs(false)
		if len(txs) == 0 {
			continue
		}
		if err := ap.pendingSF.SetNonce(hashToAddr[addrHash], txs[len(txs)-1].Nonce+1); err != nil {
			glog.Errorf("Error when resetting actPool state: %v\n", err)
			return
		}
	}
	// Reset pending balance and confirmed nonce for each account
	for addrHash, queue := range ap.accountTxs {
		balance, err := ap.pendingSF.Balance(hashToAddr[addrHash])
		if err != nil {
			glog.Errorf("Error when resetting actpool state: %v\n", err)
			return
		}
		queue.SetPendingBalance(balance)
		queue.ResetConfirmedNonce()
	}
}

// PickTxs returns all currently accepted transactions for all accounts
func (ap *actPool) PickTxs() (map[common.Hash32B][]*trx.Tx, error) {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	pending := make(map[common.Hash32B][]*trx.Tx)
	for addrHash, queue := range ap.accountTxs {
		pending[addrHash] = queue.AcceptedTxs(true)
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
		glog.Errorf("Error when validating Tx: %v\n", err)
		return err
	}

	// Reject transaction if nonce is too low
	nonce, err := ap.pendingSF.Nonce(from)
	if err != nil {
		glog.Errorf("Error when validating Tx: %v\n", err)
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
		glog.Info("Rejecting existed transaction", "hash", hash)
		return fmt.Errorf("existed transaction: %x", hash)
	}
	// Reject transaction if it fails validation
	if err := ap.validateTx(tx); err != nil {
		glog.Info("Rejecting invalid transaction", "hash", hash, "err", err)
		return err
	}
	// Reject transaction if pool space is full
	if uint64(len(ap.allTxs)) >= GlobalSlots {
		glog.Info("Rejecting transaction due to insufficient space", "hash", hash)
		return errors.Wrapf(ErrActPool, "insufficient space for transaction")
	}
	from, err := iotxaddress.GetAddress(tx.SenderPublicKey, isTestnet, chainid)
	if err != nil {
		glog.Errorf("Error when adding Tx: %v\n", err)
		return err
	}
	addrHash := from.HashAddress()
	queue := ap.accountTxs[addrHash]

	if queue == nil {
		ap.accountTxs[addrHash] = NewTxQueue()
		hashToAddr[addrHash] = from
	}

	if queue.Overlaps(tx) {
		// Nonce already exists
		glog.Info("Rejecting transaction because replacement Tx is not supported", "hash", hash)
		return errors.Wrapf(ErrNonce, "duplicate nonce")
	}

	if queue.Len() >= AccountSlots {
		glog.Info("Rejecting transaction due to insufficient space", "hash", hash)
		return errors.Wrapf(ErrActPool, "insufficient space for transaction")
	}
	queue.Put(tx)
	ap.allTxs[hash] = tx

	// If the pending nonce equals this nonce, update pending nonce
	nonce, err := ap.pendingSF.Nonce(from)
	if err != nil {
		glog.Errorf("Error when adding Tx: %v\n", err)
		return err
	}
	if tx.Nonce == nonce {
		// Flag indicating whether we need to update confirmed nonce as well
		updateConfirmedNonce := nonce == queue.ConfirmedNonce()
		newPendingNonce := queue.UpdatedPendingNonce(tx.Nonce, updateConfirmedNonce)
		if err := ap.pendingSF.SetNonce(from, newPendingNonce); err != nil {
			glog.Errorf("Error when adding Tx: %v\n", err)
			return err
		}
	}
	return nil
}

// removeCommittedTxs removes processed (committed to block) transactions from pool
func (ap *actPool) removeCommittedTxs() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	for addrHash, queue := range ap.accountTxs {
		nonce, err := ap.pendingSF.Nonce(hashToAddr[addrHash])
		if err != nil {
			glog.Errorf("Error when removing committed Txs: %v\n", err)
			return
		}

		// Remove all transactions that are committed to new block
		for _, tx := range queue.FilterNonce(nonce) {
			hash := tx.Hash()
			glog.Info("Removed committed transaction", "hash", hash)
			delete(ap.allTxs, hash)
		}

		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			delete(ap.accountTxs, addrHash)
		}
	}
}
