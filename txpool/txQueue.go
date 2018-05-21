// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package txpool

import (
	"container/heap"
	"math/big"

	"github.com/iotexproject/iotex-core/blockchain"
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

// TxList is the interface of txList
type TxQueue interface {
	Overlaps(tx *blockchain.Tx) bool
	Put(tx *blockchain.Tx) error
	FilterNonce(threshold uint64) []*blockchain.Tx
	FilterCost(costLimit *big.Int) []*blockchain.Tx
	UpdatedPendingNonce(nonce uint64) uint64
	Len() int
	Empty() bool
	AcceptedTxs() []*blockchain.Tx
}

// txList is a "list" of transactions belonging to an account
type txQueue struct {
	items   map[uint64]*blockchain.Tx // Map that stores all the transactions belonging to an account associated with nonces
	index   noncePriorityQueue        // Priority Queue that stores all the nonces belonging to an account. Nonces are used as indices for transaction map.
	costcap *big.Int                  // Price of the highest costing transaction (reset only if exceeds balance)
}

// NewTxList create a new transaction list for maintaining nonce-indexable fast,
// gapped, sortable transaction lists
func NewTxQueue() *txQueue {
	return &txQueue{
		items:   make(map[uint64]*blockchain.Tx),
		index:   noncePriorityQueue{},
		costcap: big.NewInt(0),
	}
}

// Overlap returns whether the current list contains the given nonce
func (q *txQueue) Overlaps(tx *blockchain.Tx) bool {
	return q.items[tx.Nonce] != nil
}

// Put inserts a new transaction into the map, also updating the list's nonce index and potentially costcap
func (q *txQueue) Put(tx *blockchain.Tx) error {
	nonce := tx.Nonce
	if q.items[nonce] != nil {
		return ErrReplaceTx
	}
	heap.Push(&q.index, nonce)
	q.items[nonce] = tx
	if cost := tx.Amount; q.costcap.Cmp(cost) < 0 {
		q.costcap = cost
	}
	return nil
}

// FilterNonce removes all transactions from the map with a nonce lower than the
// provided threshold
// Every removed transaction is returned for any post-removal
// maintenance
func (q *txQueue) FilterNonce(threshold uint64) []*blockchain.Tx {
	var removed []*blockchain.Tx

	// Pop off priority queue and delete corresponding entries from map until the threshold is reached.
	for q.index.Len() > 0 && (q.index)[0] < threshold {
		nonce := heap.Pop(&q.index).(uint64)
		removed = append(removed, q.items[nonce])
		delete(q.items, nonce)
	}
	return removed
}

// FilterCost removes all transactions from the list with a cost higher
// than the provided threshold. Every removed transaction is returned for any
// post-removal maintenance
//
// This method uses the cached costcap to quickly decide if there's even
// a point in calculating all the costs or if the balance covers all
func (q *txQueue) FilterCost(costLimit *big.Int) []*blockchain.Tx {
	// If all transactions are below the threshold, short circuit
	if q.costcap.Cmp(costLimit) <= 0 {
		return nil
	}
	q.costcap = new(big.Int).Set(costLimit) // Lower the caps to the thresholds

	// Filter out all the transactions above the account's funds
	var removed []*blockchain.Tx
	for nonce, tx := range q.items {
		if tx.Amount.Cmp(costLimit) > 0 {
			removed = append(removed, tx)
			delete(q.items, nonce)
		}
	}
	// If transactions were removed, the priority queue is ruined and needs to be reinitiate
	if len(removed) > 0 {
		q.index = make([]uint64, 0, len(q.items))
		for nonce := range q.items {
			q.index = append(q.index, nonce)
		}
		heap.Init(&q.index)
	}
	return removed
}

// UpdatedPendingNonce returns the next pending nonce given the current pending nonce
func (q *txQueue) UpdatedPendingNonce(nonce uint64) uint64 {
	for q.items[nonce] != nil {
		nonce++
	}
	return nonce
}

// Len returns the length of the transaction map
func (q *txQueue) Len() int {
	return len(q.items)
}

// Empty returns whether the list of transactions is empty or not
func (q *txQueue) Empty() bool {
	return q.Len() == 0
}

// AcceptedTxs creates a consecutive nonce-sorted slice of transactions
func (q *txQueue) AcceptedTxs() []*blockchain.Tx {
	txs := make([]*blockchain.Tx, 0, len(q.items))
	nonce := q.index[0]
	for q.items[nonce] != nil {
		txs = append(txs, q.items[nonce])
		nonce++
	}
	return txs
}
