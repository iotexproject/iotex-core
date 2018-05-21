// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package txpool

import (
	"container/heap"
	"errors"
	"math/big"

	"github.com/iotexproject/iotex-core-internal/blockchain"
)

var (
	//ErrReplaceTx is returned if the nonce of transaction is already in txList.
	ErrReplaceTx = errors.New("replacement transaction")
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
type TxList interface {
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
type txList struct {
	items   map[uint64]*blockchain.Tx // Map that stores all the transactions belonging to an account associated with nonces
	index   noncePriorityQueue        // Priority Queue that stores all the nonces belonging to an account. Nonces are used as indices for transaction map.
	costcap *big.Int                  // Price of the highest costing transaction (reset only if exceeds balance)
}

// NewTxList create a new transaction list for maintaining nonce-indexable fast,
// gapped, sortable transaction lists
func NewTxList() *txList {
	return &txList{
		items:   make(map[uint64]*blockchain.Tx),
		index:   noncePriorityQueue{},
		costcap: big.NewInt(0),
	}
}

// Overlap returns whether the current list contains the given nonce
func (l *txList) Overlaps(tx *blockchain.Tx) bool {
	return l.items[tx.Nonce] != nil
}

// Put inserts a new transaction into the map, also updating the list's nonce index and potentially costcap
func (l *txList) Put(tx *blockchain.Tx) error {
	nonce := tx.Nonce
	if l.items[nonce] != nil {
		return ErrReplaceTx
	}
	heap.Push(&l.index, nonce)
	l.items[nonce] = tx
	if cost := tx.Amount; l.costcap.Cmp(cost) < 0 {
		l.costcap = cost
	}
	return nil
}

// FilterNonce removes all transactions from the map with a nonce lower than the
// provided threshold
// Every removed transaction is returned for any post-removal
// maintenance
func (l *txList) FilterNonce(threshold uint64) []*blockchain.Tx {
	var removed []*blockchain.Tx

	// Pop off priority queue and delete corresponding entries from map until the threshold is reached.
	for l.index.Len() > 0 && (l.index)[0] < threshold {
		nonce := heap.Pop(&l.index).(uint64)
		removed = append(removed, l.items[nonce])
		delete(l.items, nonce)
	}
	return removed
}

// FilterCost removes all transactions from the list with a cost higher
// than the provided threshold. Every removed transaction is returned for any
// post-removal maintenance
//
// This method uses the cached costcap to quickly decide if there's even
// a point in calculating all the costs or if the balance covers all
func (l *txList) FilterCost(costLimit *big.Int) []*blockchain.Tx {
	// If all transactions are below the threshold, short circuit
	if l.costcap.Cmp(costLimit) <= 0 {
		return nil
	}
	l.costcap = new(big.Int).Set(costLimit) // Lower the caps to the thresholds

	// Filter out all the transactions above the account's funds
	var removed []*blockchain.Tx
	for nonce, tx := range l.items {
		if tx.Amount.Cmp(costLimit) > 0 {
			removed = append(removed, tx)
			delete(l.items, nonce)
		}
	}
	// If transactions were removed, the priority queue is ruined and needs to be reinitiate
	if len(removed) > 0 {
		l.index = make([]uint64, 0, len(l.items))
		for nonce := range l.items {
			l.index = append(l.index, nonce)
		}
		heap.Init(&l.index)
	}
	return removed
}

// UpdatedPendingNonce returns the next pending nonce given the current pending nonce
func (l *txList) UpdatedPendingNonce(nonce uint64) uint64 {
	for l.items[nonce] != nil {
		nonce++
	}
	return nonce
}

// Len returns the length of the transaction map
func (l *txList) Len() int {
	return len(l.items)
}

// Empty returns whether the list of transactions is empty or not
func (l *txList) Empty() bool {
	return l.Len() == 0
}

// AcceptedTxs creates a consecutive nonce-sorted slice of transactions
func (l *txList) AcceptedTxs() []*blockchain.Tx {
	txs := make([]*blockchain.Tx, 0, len(l.items))
	nonce := l.index[0]
	for l.items[nonce] != nil {
		txs = append(txs, l.items[nonce])
		nonce++
	}
	return txs
}
