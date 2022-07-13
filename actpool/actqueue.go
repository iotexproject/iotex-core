// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"container/heap"
	"context"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/facebookgo/clock"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type nonceWithTTL struct {
	idx      int
	nonce    uint64
	deadline time.Time
}

type noncePriorityQueue []*nonceWithTTL

func (h noncePriorityQueue) Len() int           { return len(h) }
func (h noncePriorityQueue) Less(i, j int) bool { return h[i].nonce < h[j].nonce }
func (h noncePriorityQueue) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].idx = i
	h[j].idx = j
}

func (h *noncePriorityQueue) Push(x interface{}) {
	if in, ok := x.(*nonceWithTTL); ok {
		in.idx = len(*h)
		*h = append(*h, in)
	}
}

func (h *noncePriorityQueue) Pop() interface{} {
	old := *h
	n := len(old)
	if n == 0 {
		return nil
	}
	x := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[0 : n-1]
	return x
}

// ActQueue is the interface of actQueue
type ActQueue interface {
	Put(action.SealedEnvelope) error
	CleanConfirmedAct() []action.SealedEnvelope
	UpdateQueue() []action.SealedEnvelope
	SetPendingNonce(uint64)
	PendingNonce() uint64
	AccountNonce() uint64
	SetAccountBalance(*big.Int)
	PendingBalance() *big.Int
	AccountBalance() *big.Int
	Len() int
	Empty() bool
	PendingActs(context.Context) []action.SealedEnvelope
	AllActs() []action.SealedEnvelope
	Reset()
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
	// Pending balance map
	pendingBalance map[uint64]*big.Int
	// Current account nonce
	accountNonce uint64
	// Current account balance
	accountBalance *big.Int
	clock          clock.Clock
	ttl            time.Duration
	mu             sync.RWMutex
}

// NewActQueue create a new action queue
func NewActQueue(ap *actPool, address string, pendingNonce uint64, balance *big.Int, ops ...ActQueueOption) ActQueue {
	aq := &actQueue{
		ap:             ap,
		address:        address,
		items:          make(map[uint64]action.SealedEnvelope),
		index:          noncePriorityQueue{},
		pendingBalance: make(map[uint64]*big.Int),
		pendingNonce:   pendingNonce,
		accountNonce:   pendingNonce,
		accountBalance: new(big.Int).Set(balance),
		clock:          clock.New(),
		ttl:            0,
	}
	for _, op := range ops {
		op.SetActQueueOption(aq)
	}
	return aq
}

// Put inserts a new action into the map, also updating the queue's nonce index
func (q *actQueue) Put(act action.SealedEnvelope) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	nonce := act.Nonce()

	if cost, _ := act.Cost(); q.getPendingBalanceAtNonce(nonce).Cmp(cost) < 0 {
		return action.ErrInsufficientFunds
	}

	if actInPool, exist := q.items[nonce]; exist {
		// act of higher gas price can cut in line
		if nonce < q.pendingNonce && act.GasPrice().Cmp(actInPool.GasPrice()) != 1 {
			return action.ErrReplaceUnderpriced
		}
		// update action in q.items and q.index
		q.items[nonce] = act
		for i := range q.index {
			if q.index[i].nonce == nonce {
				q.index[i].deadline = q.clock.Now().Add(q.ttl)
				break
			}
		}
		q.updateFromNonce(nonce)
		return nil
	}
	heap.Push(&q.index, &nonceWithTTL{nonce: nonce, deadline: q.clock.Now().Add(q.ttl)})
	q.items[nonce] = act
	if nonce == q.pendingNonce {
		q.updateFromNonce(q.pendingNonce)
	}
	return nil
}

func (q *actQueue) getPendingBalanceAtNonce(nonce uint64) *big.Int {
	if nonce > q.pendingNonce {
		return q.getPendingBalanceAtNonce(q.pendingNonce)
	}
	if _, exist := q.pendingBalance[nonce]; !exist {
		return new(big.Int).Set(q.accountBalance)
	}
	return new(big.Int).Set(q.pendingBalance[nonce])
}

func (q *actQueue) updateFromNonce(start uint64) {
	if start > q.pendingNonce {
		return
	}

	for balance := q.getPendingBalanceAtNonce(start); ; start++ {
		act, exist := q.items[start]
		if !exist {
			break
		}

		cost, _ := act.Cost()
		if balance.Cmp(cost) < 0 {
			break
		}

		balance = new(big.Int).Sub(balance, cost)
		q.pendingBalance[start+1] = new(big.Int).Set(balance)
	}

	q.pendingNonce = start
}

// CleanConfirmedAct removes all actions from the map with a nonce lower than account's nonce
func (q *actQueue) CleanConfirmedAct() []action.SealedEnvelope {
	q.mu.Lock()
	defer q.mu.Unlock()
	var removed []action.SealedEnvelope
	// Pop off priority queue and delete corresponding entries from map until the threshold is reached
	for q.index.Len() > 0 && (q.index)[0].nonce < q.accountNonce {
		nonce := heap.Pop(&q.index).(*nonceWithTTL).nonce
		removed = append(removed, q.items[nonce])
		delete(q.items, nonce)
		delete(q.pendingBalance, nonce)
	}
	return removed
}

func (q *actQueue) cleanTimeout() []action.SealedEnvelope {
	if q.ttl == 0 {
		return []action.SealedEnvelope{}
	}
	var (
		removedFromQueue = make([]action.SealedEnvelope, 0)
		timeNow          = q.clock.Now()
		size             = len(q.index)
	)
	for i := 0; i < size; {
		if timeNow.After(q.index[i].deadline) {
			nonce := q.index[i].nonce
			if nonce < q.pendingNonce {
				q.pendingNonce = nonce
			}
			removedFromQueue = append(removedFromQueue, q.items[nonce])
			delete(q.items, nonce)
			delete(q.pendingBalance, nonce)
			q.index[i] = q.index[size-1]
			size--
			continue
		}
		i++
	}
	q.index = q.index[:size]
	// using heap.Init is better here, more detail to see BenchmarkHeapInitAndRemove
	heap.Init(&q.index)
	return removedFromQueue
}

// UpdateQueue updates the pending nonce and balance of the queue
func (q *actQueue) UpdateQueue() []action.SealedEnvelope {
	q.mu.Lock()
	defer q.mu.Unlock()
	// First remove all timed out actions
	removedFromQueue := q.cleanTimeout()
	// Now, starting from the current pending nonce, incrementally find the next pending nonce
	q.updateFromNonce(q.pendingNonce)
	return removedFromQueue
}

// SetPendingNonce sets pending nonce for the queue
func (q *actQueue) SetPendingNonce(nonce uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pendingNonce = nonce
	q.accountNonce = nonce
}

// PendingNonce returns the current pending nonce of the queue
func (q *actQueue) PendingNonce() uint64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.pendingNonce
}

// AccountNonce returns the current account nonce
func (q *actQueue) AccountNonce() uint64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.accountNonce
}

// SetAccountBalance sets account balance for the queue
func (q *actQueue) SetAccountBalance(balance *big.Int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.accountBalance.Set(balance)
	q.pendingBalance = make(map[uint64]*big.Int)
}

// PendingBalance returns the current pending balance of the queue
func (q *actQueue) PendingBalance() *big.Int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.getPendingBalanceAtNonce(q.pendingNonce)
}

// AccountBalance returns the current account balance
func (q *actQueue) AccountBalance() *big.Int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return new(big.Int).Set(q.accountBalance)
}

// Len returns the length of the action map
func (q *actQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}

// Empty returns whether the queue of actions is empty or not
func (q *actQueue) Empty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items) == 0
}

// Reset makes the queue into a dummy queue
func (q *actQueue) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = make(map[uint64]action.SealedEnvelope)
	q.index = noncePriorityQueue{}
	q.pendingNonce = 0
	q.accountBalance = big.NewInt(0)
}

// PendingActs creates a consecutive nonce-sorted slice of actions
func (q *actQueue) PendingActs(ctx context.Context) []action.SealedEnvelope {
	if q.Len() == 0 {
		return nil
	}
	addr, err := address.FromString(q.address)
	if err != nil {
		log.L().Error("Error when getting the address", zap.String("address", q.address), zap.Error(err))
		return nil
	}
	confirmedState, err := accountutil.AccountState(ctx, q.ap.sf, addr)
	if err != nil {
		log.L().Error("Error when getting the nonce", zap.String("address", q.address), zap.Error(err))
		return nil
	}

	var (
		nonce   = confirmedState.PendingNonce()
		balance = new(big.Int).Set(confirmedState.Balance)
		acts    = make([]action.SealedEnvelope, 0, len(q.items))
	)
	q.mu.RLock()
	defer q.mu.RUnlock()
	for ; ; nonce++ {
		act, exist := q.items[nonce]
		if !exist {
			break
		}

		cost, _ := act.Cost()
		if balance.Cmp(cost) < 0 {
			break
		}

		balance = new(big.Int).Sub(balance, cost)
		acts = append(acts, act)
	}
	return acts
}

// AllActs returns all the actions currently in queue
func (q *actQueue) AllActs() []action.SealedEnvelope {
	q.mu.Lock()
	defer q.mu.Unlock()
	acts := make([]action.SealedEnvelope, 0, len(q.items))
	if len(q.items) == 0 {
		return acts
	}
	sort.Sort(q.index)
	for _, nonce := range q.index {
		acts = append(acts, q.items[nonce.nonce])
	}
	return acts
}
