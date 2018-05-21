// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package txpool

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	bc "github.com/iotexproject/iotex-core-internal/blockchain"
	"github.com/iotexproject/iotex-core-internal/common"
	"github.com/iotexproject/iotex-core-internal/iotxaddress"
	"github.com/iotexproject/iotex-core-internal/statefactory"
	"github.com/iotexproject/iotex-core-internal/trie"
)

const (
	GlobalSlots  = 5120
	AccountSlots = 80 // Maximum transactions an account can hold
)

var (
	isTestnet = true

	chainid = []byte{0x00, 0x00, 0x00, 0x01}

	hashToAddr = make(map[common.Hash32B]*iotxaddress.Address)

	// ErrNonceTooLow is returned if the nonce of a transaction is lower than the
	// one present in the local chain.
	ErrNonceTooLow = errors.New("nonce too low")

	// ErrInsufficientSpace is returned if the total space of queues is insufficient.
	ErrInsufficientSpace = errors.New("insufficient space for transaction")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

	// ErrNegativeValue is a sanity error to ensure noone is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")

	//ErrReplaceTx is returned if the nonce of transaction is already in txQueue.
	ErrReplaceTx = errors.New("replacement transaction")
)

// ActPool is the interface of actpool
type ActPool interface {
	// Reset resets actpool state.
	Reset()
	// Pending retrieves all currently processable transactions in actpool.
	Pending() (map[common.Hash32B][]*bc.Tx, error)
	// AddTx enqueues a single transaction into the pool if it is valid.
	AddTx(tx *bc.Tx) error
}

// actPool implements ActPool interface.
// Note that all locks should be placed in public functions (no lock inside of any private function).
type actPool struct {
	lastUpdatedUnixTime int64
	mutex               sync.RWMutex
	pendingSF           statefactory.StateFactory // Pending state tracking virtual nonces
	accountTxs          map[common.Hash32B]TxQueue
	all                 map[common.Hash32B]*bc.Tx
	beats               map[common.Hash32B]time.Time // Last heartbeat from each known account
}

// NewActPool constructs a new actpool.
func NewActPool(trie trie.Trie) ActPool {
	// Create the transaction pool with its initial settings.
	ap := &actPool{
		pendingSF:  statefactory.NewVirtualStateFactory(trie),
		accountTxs: make(map[common.Hash32B]TxQueue),
		beats:      make(map[common.Hash32B]time.Time),
		all:        make(map[common.Hash32B]*bc.Tx),
	}
	return ap
}

// Reset resets actpool state.
func (ap *actPool) Reset() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	ap.reset()
}

func (ap *actPool) reset() {
	// validate the pool of transactions, this will remove
	// any transactions that have been included in the block or
	// have been invalidated because of another transaction.
	ap.removeCommittedTxs()

	for addrHash, queue := range ap.accountTxs {
		txs := queue.AcceptedTxs() // Heavy but will be cached and is needed by the miner anyway
		if err := ap.pendingSF.SetNonce(hashToAddr[addrHash], txs[len(txs)-1].Nonce+1); err != nil {
			glog.Errorf("Error when resetting the actPool state: %v\n", err)
			return
		}
	}
}

// Pending retrieves all currently processable transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (ap *actPool) Pending() (map[common.Hash32B][]*bc.Tx, error) {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	pending := make(map[common.Hash32B][]*bc.Tx)
	for addrHash, queue := range ap.accountTxs {
		pending[addrHash] = queue.AcceptedTxs()
	}
	return pending, nil
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (ap *actPool) validateTx(tx *bc.Tx) error {
	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	if tx.TotalSize() > 32*1024 {
		return ErrOversizedData
	}
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Amount.Sign() < 0 {
		return ErrNegativeValue
	}

	from, err := iotxaddress.GetAddress(tx.SenderPublicKey, isTestnet, chainid)
	if err != nil {
		glog.Errorf("Error when validating Tx: %v\n", err)
		return err
	}

	// Ensure the transaction adheres to nonce ordering
	nonce, err := ap.pendingSF.Nonce(from)
	if err != nil {
		glog.Errorf("Error when validating Tx: %v\n", err)
		return err
	}
	if nonce > tx.Nonce {
		return ErrNonceTooLow
	}
	// Transactor should have enough funds to cover the costs
	// cost == V + F
	balance, err := ap.pendingSF.Balance(from)
	if err != nil {
		glog.Errorf("Error when validating Tx: %v\n", err)
		return err
	}
	if balance.Cmp(tx.Amount) < 0 {
		return ErrInsufficientFunds
	}
	return nil
}

// AddTx validates a transaction and inserts it into account queue
func (ap *actPool) AddTx(tx *bc.Tx) error {
	hash := tx.Hash()
	// If the transaction is already known, discard it
	if ap.all[hash] != nil {
		glog.Info("Discarding already known transaction", "hash", hash)
		return fmt.Errorf("known transaction: %x", hash)
	}
	// If the transaction fails basic validation, discard it
	if err := ap.validateTx(tx); err != nil {
		glog.Info("Discarding invalid transaction", "hash", hash, "err", err)
		return err
	}
	// If the transaction pool is full, discard underpriced transactions
	if uint64(len(ap.all)) >= GlobalSlots {
		glog.Info("Discarding transaction due to insufficient space", "hash", hash)
		return ErrInsufficientSpace
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
		// Nonce already accepted
		glog.Info("Discarding transaction because replacement Tx is not supported", "hash", hash)
		return ErrReplaceTx
	}

	if queue.Len() >= AccountSlots {
		glog.Info("Discarding transaction due to insufficient space", "hash", hash)
		return ErrInsufficientSpace
	}
	queue.Put(tx)
	ap.all[hash] = tx
	// If the pending nonce equals this nonce, update beat map and update pending nonce.
	nonce, err := ap.pendingSF.Nonce(from)
	if err != nil {
		glog.Errorf("Error when adding Tx: %v\n", err)
		return err
	}
	if tx.Nonce == nonce {
		ap.beats[addrHash] = time.Now()
		newPendingNonce := queue.UpdatedPendingNonce(tx.Nonce)
		if err := ap.pendingSF.SetNonce(from, newPendingNonce); err != nil {
			glog.Errorf("Error when adding Tx: %v\n", err)
			return err
		}
	}
	return nil
}

// removeCommittedTxs removes processed (committed to block) transactions from the pools
func (ap *actPool) removeCommittedTxs() {
	for addrHash, queue := range ap.accountTxs {
		nonce, err := ap.pendingSF.Nonce(hashToAddr[addrHash])
		if err != nil {
			glog.Errorf("Error when removing committed Txs: %v\n", err)
			return
		}

		// Drop all transactions that are deemed too old (low nonce)
		for _, tx := range queue.FilterNonce(nonce) {
			hash := tx.Hash()
			glog.Info("Removed old accepted transaction", "hash", hash)
			delete(ap.all, hash)
		}

		balance, err := ap.pendingSF.Balance(hashToAddr[addrHash])
		if err != nil {
			glog.Errorf("Error when removing committed Txs: %v\n", err)
			return
		}
		// Drop all transactions that are too costly (low balance or out of gas)
		drops := queue.FilterCost(balance)
		for _, tx := range drops {
			hash := tx.Hash()
			glog.Info("Removed unpayable accepted transaction", "hash", hash)
			delete(ap.all, hash)
		}
		// Delete the entire queue entry if it became empty.
		if queue.Empty() {
			delete(ap.accountTxs, addrHash)
			delete(ap.beats, addrHash)
		}
	}
}
