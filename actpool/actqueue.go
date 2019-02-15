// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"container/heap"
	"math/big"
	"sort"
	"time"

	"github.com/iotexproject/iotex-core/pkg/log"
	"go.uber.org/zap"

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
)

type nonceWithTTL struct {
	nonce    uint64
	deadline time.Time
}

type noncePriorityQueue []nonceWithTTL

func (h noncePriorityQueue) Len() int           { return len(h) }
func (h noncePriorityQueue) Less(i, j int) bool { return h[i].nonce < h[j].nonce }
func (h noncePriorityQueue) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *noncePriorityQueue) Push(x interface{}) {
	in, ok := x.(nonceWithTTL)
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
	Overlaps(action.SealedEnvelope) bool
	Put(action.SealedEnvelope) error
	FilterNonce(uint64) []action.SealedEnvelope
	UpdateQueue(uint64) []action.SealedEnvelope
	SetPendingNonce(uint64)
	PendingNonce() uint64
	SetPendingBalance(*big.Int)
	PendingBalance() *big.Int
	Len() int
	Empty() bool
	PendingActs() []action.SealedEnvelope
	AllActs() []action.SealedEnvelope
}

// actQueue is a queue of actions from an account
type actQueue struct {
	ap      *actPool
	address string
	// Map that stores all the actions belonging to an account associated with nonces
	items map[uint64]action.SealedEnvelope
	// Priority Queue that stores all the nonces belonging to an account. Nonces are used as indices for action map
	index noncePriorityQueue
	// Current pending nonce tracking previous actions that can be committed to the next block for the account
	pendingNonce uint64
	// Current pending balance for the account
	pendingBalance *big.Int
	clock          clock.Clock
	ttl            time.Duration
}

// ActQueueOption is the option for actQueue.
type ActQueueOption interface {
	SetActQueueOption(*actQueue)
}

// NewActQueue create a new action queue
func NewActQueue(ap *actPool, address string, ops ...ActQueueOption) ActQueue {
	aq := &actQueue{
		ap:             ap,
		address:        address,
		items:          make(map[uint64]action.SealedEnvelope),
		index:          noncePriorityQueue{},
		pendingNonce:   uint64(1), // Taking coinbase Action into account, pendingNonce should start with 1
		pendingBalance: big.NewInt(0),
		clock:          clock.New(),
		ttl:            0,
	}
	for _, op := range ops {
		op.SetActQueueOption(aq)
	}
	return aq
}

// Overlap returns whether the current queue contains the given nonce
func (q *actQueue) Overlaps(act action.SealedEnvelope) bool {
	_, exist := q.items[act.Nonce()]
	return exist
}

// Put inserts a new action into the map, also updating the queue's nonce index
func (q *actQueue) Put(act action.SealedEnvelope) error {
	nonce := act.Nonce()
	if _, exist := q.items[nonce]; exist {
		return errors.Wrapf(action.ErrNonce, "duplicate nonce")
	}
	heap.Push(&q.index, nonceWithTTL{nonce: nonce, deadline: q.clock.Now().Add(q.ttl)})
	q.items[nonce] = act
	return nil
}

// FilterNonce removes all actions from the map with a nonce lower than the given threshold
func (q *actQueue) FilterNonce(threshold uint64) []action.SealedEnvelope {
	var removed []action.SealedEnvelope
	// Pop off priority queue and delete corresponding entries from map until the threshold is reached
	for q.index.Len() > 0 && (q.index)[0].nonce < threshold {
		nonce := heap.Pop(&q.index).(nonceWithTTL).nonce
		removed = append(removed, q.items[nonce])
		delete(q.items, nonce)
	}
	return removed
}

func (q *actQueue) cleanTimeout() []action.SealedEnvelope {
	removedFromQueue := make([]action.SealedEnvelope, 0)
	for i := 0; i < len(q.index); i++ {
		if q.clock.Now().After(q.index[i].deadline) {
			// remove
			removedFromQueue = append(removedFromQueue, q.items[q.index[i].nonce])
			delete(q.items, q.index[i].nonce)
			q.index = append(q.index[:i], q.index[i+1:]...)
		}
	}
	return removedFromQueue
}

// UpdateQueue updates the pending nonce and balance of the queue
func (q *actQueue) UpdateQueue(nonce uint64) []action.SealedEnvelope {
	removedFromQueue := make([]action.SealedEnvelope, 0)
	// First remove all timed out actions
	if q.ttl != 0 {
		removedFromQueue = append(removedFromQueue, q.cleanTimeout()...)
	}

	// Now, starting from the current pending nonce, incrementally find the next pending nonce
	// while updating pending balance if actions are payable
	for ; ; nonce++ {
		_, exist := q.items[nonce]
		if !exist {
			break
		}
		if !q.enoughBalance(q.items[nonce], true) {
			break
		}
	}
	q.pendingNonce = nonce

	// Find the index of new pending nonce within the queue
	sort.Sort(q.index)
	i := 0
	for ; i < q.index.Len(); i++ {
		if q.index[i].nonce >= nonce {
			break
		}
	}
	// Case I: An unpayable action has been found while updating pending nonce/balance
	// Remove all the subsequent actions in the queue starting from the index of new pending nonce
	if _, exist := q.items[nonce]; exist {
		removedFromQueue = append(removedFromQueue, q.removeActs(i)...)
		return removedFromQueue
	}

	// Case II: All actions are payable while updating pending nonce/balance
	// Check all the subsequent actions in the queue starting from the index of new pending nonce
	// Find the nonce index of the first unpayable action
	// Remove all the subsequent actions in the queue starting from that index
	for ; i < q.index.Len(); i++ {
		nonce = q.index[i].nonce
		act := q.items[nonce]
		if !q.enoughBalance(act, false) {
			break
		}
	}
	removedFromQueue = append(removedFromQueue, q.removeActs(i)...)
	return removedFromQueue
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

// PendingActs creates a consecutive nonce-sorted slice of actions
func (q *actQueue) PendingActs() []action.SealedEnvelope {
	if q.Len() == 0 {
		return []action.SealedEnvelope{}
	}
	acts := make([]action.SealedEnvelope, 0, len(q.items))
	confirmedNonce, err := q.ap.bc.Nonce(q.address)
	if err != nil {
		log.L().Error("Error when getting the nonce", zap.String("address", q.address), zap.Error(err))
		return nil
	}
	nonce := confirmedNonce + 1
	for ; ; nonce++ {
		if _, exist := q.items[nonce]; !exist {
			break
		}
		acts = append(acts, q.items[nonce])
	}
	return acts
}

// AllActs returns all the actions currently in queue
func (q *actQueue) AllActs() []action.SealedEnvelope {
	acts := make([]action.SealedEnvelope, 0, len(q.items))
	if q.Len() == 0 {
		return acts
	}
	sort.Sort(q.index)
	for _, nonce := range q.index {
		acts = append(acts, q.items[nonce.nonce])
	}
	return acts
}

// removeActs removes all the actions starting at idx from queue
func (q *actQueue) removeActs(idx int) []action.SealedEnvelope {
	removedFromQueue := make([]action.SealedEnvelope, 0)
	for i := idx; i < q.index.Len(); i++ {
		removedFromQueue = append(removedFromQueue, q.items[q.index[i].nonce])
		delete(q.items, q.index[i].nonce)
	}
	q.index = q.index[:idx]
	heap.Init(&q.index)
	return removedFromQueue
}

// enoughBalance helps check whether queue's pending balance is sufficient for the given action
func (q *actQueue) enoughBalance(act action.SealedEnvelope, updateBalance bool) bool {
	cost, _ := act.Cost()
	if q.pendingBalance.Cmp(cost) < 0 {
		return false
	}

	if updateBalance {
		q.pendingBalance.Sub(q.pendingBalance, cost)
	}

	return true
}
