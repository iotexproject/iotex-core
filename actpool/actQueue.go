// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"container/heap"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/proto"
)

type noncePriorityQueue []uint64

func (h noncePriorityQueue) Len() int           { return len(h) }
func (h noncePriorityQueue) Less(i, j int) bool { return h[i] < h[j] }
func (h noncePriorityQueue) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *noncePriorityQueue) Push(x interface{}) {
	in, ok := x.(uint64)
	if !ok {
		return
	}
	*h = append(*h, in)
}

func (h *noncePriorityQueue) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// ActQueue is the interface of actQueue
type ActQueue interface {
	Overlaps(*iproto.ActionPb) bool
	Put(*iproto.ActionPb) error
	FilterNonce(uint64) []*iproto.ActionPb
	UpdateNonce(uint64)
	SetConfirmedNonce(uint64)
	ConfirmedNonce() uint64
	SetPendingNonce(uint64)
	PendingNonce() uint64
	SetPendingBalance(*big.Int)
	PendingBalance() *big.Int
	Len() int
	Empty() bool
	ConfirmedActs() []*iproto.ActionPb
}

// actQueue is a queue of actions from an account
type actQueue struct {
	// Map that stores all the actions belonging to an account associated with nonces
	items map[uint64]*iproto.ActionPb
	// Priority Queue that stores all the nonces belonging to an account. Nonces are used as indices for action map
	index noncePriorityQueue
	// Current nonce tracking previous actions that can be committed to the next block
	confirmedNonce uint64
	// Current pending nonce for the account
	pendingNonce uint64
	// Current pending balance for the account
	pendingBalance *big.Int
}

// NewActQueue create a new action queue
func NewActQueue() ActQueue {
	return &actQueue{
		items:          make(map[uint64]*iproto.ActionPb),
		index:          noncePriorityQueue{},
		confirmedNonce: uint64(1), // Taking coinbase Action into account, confirmedNonce should start with 1
		pendingNonce:   uint64(1), // Taking coinbase Action into account, pendingNonce should start with 1
		pendingBalance: big.NewInt(0),
	}
}

// Overlap returns whether the current queue contains the given nonce
func (q *actQueue) Overlaps(act *iproto.ActionPb) bool {
	var nonce uint64
	switch {
	case act.GetTransfer() != nil:
		tsf := &action.Transfer{}
		tsf.ConvertFromTransferPb(act.GetTransfer())
		nonce = tsf.Nonce
	case act.GetVote() != nil:
		vote := &action.Vote{}
		vote.ConvertFromVotePb(act.GetVote())
		nonce = vote.Nonce
	}
	return q.items[nonce] != nil
}

// Put inserts a new action into the map, also updating the queue's nonce index
func (q *actQueue) Put(act *iproto.ActionPb) error {
	var nonce uint64
	switch {
	case act.GetTransfer() != nil:
		tsf := &action.Transfer{}
		tsf.ConvertFromTransferPb(act.GetTransfer())
		nonce = tsf.Nonce
	case act.GetVote() != nil:
		vote := &action.Vote{}
		vote.ConvertFromVotePb(act.GetVote())
		nonce = vote.Nonce
	}
	if q.items[nonce] != nil {
		return errors.Wrapf(ErrNonce, "duplicate nonce")
	}
	heap.Push(&q.index, nonce)
	q.items[nonce] = act
	return nil
}

// FilterNonce removes all actions from the map with a nonce lower than the given threshold
func (q *actQueue) FilterNonce(threshold uint64) []*iproto.ActionPb {
	var removed []*iproto.ActionPb
	// Pop off priority queue and delete corresponding entries from map until the threshold is reached
	for q.index.Len() > 0 && (q.index)[0] < threshold {
		nonce := heap.Pop(&q.index).(uint64)
		removed = append(removed, q.items[nonce])
		delete(q.items, nonce)
	}
	return removed
}

// UpdatePendingNonce returns the next pending nonce starting from the given nonce
func (q *actQueue) UpdateNonce(nonce uint64) {
	for ; q.items[nonce] != nil; nonce++ {
		if nonce != q.confirmedNonce {
			continue
		}
		if q.items[nonce].GetVote() != nil {
			q.confirmedNonce++
			continue
		}
		tsf := &action.Transfer{}
		tsf.ConvertFromTransferPb(q.items[nonce].GetTransfer())
		if q.pendingBalance.Cmp(tsf.Amount) >= 0 {
			q.confirmedNonce++
			q.pendingBalance.Sub(q.pendingBalance, tsf.Amount)
		}
	}
	q.pendingNonce = nonce
}

// SetConfirmedNonce sets the new confirmed nonce for the queue
func (q *actQueue) SetConfirmedNonce(nonce uint64) {
	q.confirmedNonce = nonce
}

// ConfirmedNonce returns the current confirmed nonce of the queue
func (q *actQueue) ConfirmedNonce() uint64 {
	return q.confirmedNonce
}

// SetPendingNonce sets pending nonce for the queue
func (q *actQueue) SetPendingNonce(nonce uint64) {
	q.pendingNonce = nonce
}

// PendingNonce returns the current pending nonce of the queue
func (q *actQueue) PendingNonce() uint64 {
	return q.pendingNonce
}

// SetPendingBalance sets pending balance for the queue
func (q *actQueue) SetPendingBalance(balance *big.Int) {
	q.pendingBalance = balance
}

// PendingBalance returns the current pending balance of the queue
func (q *actQueue) PendingBalance() *big.Int {
	return q.pendingBalance
}

// Len returns the length of the action map
func (q *actQueue) Len() int {
	return len(q.items)
}

// Empty returns whether the queue of actions is empty or not
func (q *actQueue) Empty() bool {
	return q.Len() == 0
}

// ConfirmedTsfs creates a consecutive nonce-sorted slice of actions
func (q *actQueue) ConfirmedActs() []*iproto.ActionPb {
	if q.Len() == 0 {
		return []*iproto.ActionPb{}
	}
	acts := make([]*iproto.ActionPb, 0, len(q.items))
	nonce := q.index[0]
	for q.items[nonce] != nil && nonce < q.confirmedNonce {
		acts = append(acts, q.items[nonce])
		nonce++
	}
	return acts
}
