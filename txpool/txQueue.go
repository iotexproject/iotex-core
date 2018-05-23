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

	trx "github.com/iotexproject/iotex-core/blockchain/trx"
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
	Overlaps(*trx.Tx) bool
	Put(*trx.Tx) error
	FilterNonce(uint64) []*trx.Tx
	UpdatedPendingNonce(uint64, bool) uint64
	SetConfirmedNonce(uint64)
	ConfirmedNonce() uint64
	SetPendingBalance(*big.Int)
	Len() int
	Empty() bool
	ConfirmedTxs() []*trx.Tx
	UpdateConfirmedNonce()
}

// txQueue is a queue of transactions from an account
type txQueue struct {
	items          map[uint64]*trx.Tx // Map that stores all the transactions belonging to an account associated with nonces
	index          noncePriorityQueue // Priority Queue that stores all the nonces belonging to an account. Nonces are used as indices for transaction map
	pendingNonce   uint64             // Current pending nonce for the account
	confirmedNonce uint64             // Current nonce tracking previous transactions that can be committed to the next block
	pendingBalance *big.Int           // Current pending balance for the account
}

// NewTxQueue create a new transaction queue
func NewTxQueue() *txQueue {
	return &txQueue{
		items:          make(map[uint64]*trx.Tx),
		index:          noncePriorityQueue{},
		pendingNonce:   uint64(0),
		confirmedNonce: uint64(0),
		pendingBalance: big.NewInt(0),
	}
}

// Overlap returns whether the current queue contains the given nonce
func (q *txQueue) Overlaps(tx *trx.Tx) bool {
	return q.items[tx.Nonce] != nil
}

// Put inserts a new transaction into the map, also updating the queue's nonce index
func (q *txQueue) Put(tx *trx.Tx) error {
	nonce := tx.Nonce
	if q.items[nonce] != nil {
		return errors.Wrapf(ErrNonce, "duplicate nonce")
	}
	heap.Push(&q.index, nonce)
	q.items[nonce] = tx
	return nil
}

// FilterNonce removes all transactions from the map with a nonce lower than the given threshold
func (q *txQueue) FilterNonce(threshold uint64) []*trx.Tx {
	var removed []*trx.Tx
	// Pop off priority queue and delete corresponding entries from map until the threshold is reached
	for q.index.Len() > 0 && (q.index)[0] < threshold {
		nonce := heap.Pop(&q.index).(uint64)
		removed = append(removed, q.items[nonce])
		delete(q.items, nonce)
	}
	return removed
}

// UpdatedPendingNonce returns the next pending nonce given the current pending nonce
func (q *txQueue) UpdatedPendingNonce(nonce uint64, updateConfirmedNonce bool) uint64 {
	for q.items[nonce] != nil {
		if updateConfirmedNonce && q.pendingBalance.Cmp(q.items[nonce].Amount) >= 0 {
			q.confirmedNonce++
			q.pendingBalance.Sub(q.pendingBalance, q.items[nonce].Amount)
		}
		nonce++
	}
	q.pendingNonce = nonce
	return nonce
}

// SetConfirmedNonce sets the new confirmed nonce for the queue
func (q *txQueue) SetConfirmedNonce(nonce uint64) {
	q.confirmedNonce = nonce
}

// ConfirmedNonce returns the current confirmed nonce for the queue
func (q *txQueue) ConfirmedNonce() uint64 {
	return q.confirmedNonce
}

// SetPendingBalance sets pending balance for the queue
func (q *txQueue) SetPendingBalance(balance *big.Int) {
	q.pendingBalance = balance
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
func (q *txQueue) ConfirmedTxs() []*trx.Tx {
	txs := make([]*trx.Tx, 0, len(q.items))
	nonce := q.index[0]
	for q.items[nonce] != nil && nonce < q.confirmedNonce {
		txs = append(txs, q.items[nonce])
		nonce++
	}
	return txs
}

// UpdateConfirmedNonce updates confirmed nonce for the queue
func (q *txQueue) UpdateConfirmedNonce() {
	if q.index[0] < q.confirmedNonce {
		q.confirmedNonce = q.index[0]
	}
	for q.items[q.confirmedNonce] != nil {
		if q.pendingBalance.Cmp(q.items[q.confirmedNonce].Amount) < 0 {
			break
		}
		q.pendingBalance.Sub(q.pendingBalance, q.items[q.confirmedNonce].Amount)
		q.confirmedNonce++
	}
}
