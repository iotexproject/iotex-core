// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txpool

import (
	"container/heap"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
)

type noncePriorityQueue []uint64

func (h noncePriorityQueue) Len() int           { return len(h) }
func (h noncePriorityQueue) Less(i, j int) bool { return h[i] < h[j] }
func (h noncePriorityQueue) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *noncePriorityQueue) Push(x interface{}) {
	*h = append(*h, x.(uint64))
}

func (h *noncePriorityQueue) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// TxQueue is the interface of txQueue
type TxQueue interface {
	Overlaps(*action.Transfer) bool
	Put(*action.Transfer) error
	FilterNonce(uint64) []*action.Transfer
	UpdateNonce(uint64)
	SetConfirmedNonce(uint64)
	ConfirmedNonce() uint64
	SetPendingNonce(uint64)
	PendingNonce() uint64
	SetPendingBalance(*big.Int)
	PendingBalance() *big.Int
	Len() int
	Empty() bool
	ConfirmedTxs() []*action.Transfer
}

// txQueue is a queue of transactions from an account
type txQueue struct {
	items          map[uint64]*action.Transfer // Map that stores all the transactions belonging to an account associated with nonces
	index          noncePriorityQueue          // Priority Queue that stores all the nonces belonging to an account. Nonces are used as indices for transaction map
	confirmedNonce uint64                      // Current nonce tracking previous transactions that can be committed to the next block
	pendingNonce   uint64                      // Current pending nonce for the account
	pendingBalance *big.Int                    // Current pending balance for the account
}

// NewTxQueue create a new transaction queue
func NewTxQueue() TxQueue {
	return &txQueue{
		items:          make(map[uint64]*action.Transfer),
		index:          noncePriorityQueue{},
		confirmedNonce: uint64(1), // Taking coinbase Tx into account, confirmedNonce should start with 1
		pendingNonce:   uint64(1), // Taking coinbase Tx into account, pendingNonce should start with 1
		pendingBalance: big.NewInt(0),
	}
}

// Overlap returns whether the current queue contains the given nonce
func (q *txQueue) Overlaps(tx *action.Transfer) bool {
	return q.items[tx.Nonce] != nil
}

// Put inserts a new transaction into the map, also updating the queue's nonce index
func (q *txQueue) Put(tx *action.Transfer) error {
	nonce := tx.Nonce
	if q.items[nonce] != nil {
		return errors.Wrapf(ErrNonce, "duplicate nonce")
	}
	heap.Push(&q.index, nonce)
	q.items[nonce] = tx
	return nil
}

// FilterNonce removes all transactions from the map with a nonce lower than the given threshold
func (q *txQueue) FilterNonce(threshold uint64) []*action.Transfer {
	var removed []*action.Transfer
	// Pop off priority queue and delete corresponding entries from map until the threshold is reached
	for q.index.Len() > 0 && (q.index)[0] < threshold {
		nonce := heap.Pop(&q.index).(uint64)
		removed = append(removed, q.items[nonce])
		delete(q.items, nonce)
	}
	return removed
}

// UpdatePendingNonce returns the next pending nonce starting from the given nonce
func (q *txQueue) UpdateNonce(nonce uint64) {
	for q.items[nonce] != nil {
		if nonce == q.confirmedNonce && q.pendingBalance.Cmp(q.items[nonce].Amount) >= 0 {
			q.confirmedNonce++
			q.pendingBalance.Sub(q.pendingBalance, q.items[nonce].Amount)
		}
		nonce++
	}
	q.pendingNonce = nonce
}

// SetConfirmedNonce sets the new confirmed nonce for the queue
func (q *txQueue) SetConfirmedNonce(nonce uint64) {
	q.confirmedNonce = nonce
}

// ConfirmedNonce returns the current confirmed nonce for the queue
func (q *txQueue) ConfirmedNonce() uint64 {
	return q.confirmedNonce
}

// SetPendingNonce sets pending nonce for the queue
func (q *txQueue) SetPendingNonce(nonce uint64) {
	q.pendingNonce = nonce
}

// PendingNonce returns the current pending nonce of the queue
func (q *txQueue) PendingNonce() uint64 {
	return q.pendingNonce
}

// SetPendingBalance sets pending balance for the queue
func (q *txQueue) SetPendingBalance(balance *big.Int) {
	q.pendingBalance = balance
}

// PendingBalance returns the current pending balance for the queue
func (q *txQueue) PendingBalance() *big.Int {
	return q.pendingBalance
}

// Len returns the length of the transaction map
func (q *txQueue) Len() int {
	return len(q.items)
}

// Empty returns whether the queue of transactions is empty or not
func (q *txQueue) Empty() bool {
	return q.Len() == 0
}

// ConfirmedTxs creates a consecutive nonce-sorted slice of transactions
func (q *txQueue) ConfirmedTxs() []*action.Transfer {
	txs := make([]*action.Transfer, 0, len(q.items))
	nonce := q.index[0]
	for q.items[nonce] != nil && nonce < q.confirmedNonce {
		txs = append(txs, q.items[nonce])
		nonce++
	}
	return txs
}
