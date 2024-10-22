// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"container/heap"
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// ActQueue is the interface of actQueue
type ActQueue interface {
	Put(*action.SealedEnvelope) error
	UpdateQueue() []*action.SealedEnvelope
	UpdateAccountState(uint64, *big.Int) []*action.SealedEnvelope
	AccountState() (uint64, *big.Int)
	PendingNonce() uint64
	NextAction() (bool, *big.Int)
	Len() int
	Empty() bool
	PendingActs(context.Context) []*action.SealedEnvelope
	AllActs() []*action.SealedEnvelope
	PopActionWithLargestNonce() *action.SealedEnvelope
	Reset()
}

// actQueue is a queue of actions from an account
type actQueue struct {
	ap      *actPool
	address string
	// Map that stores all the actions belonging to an account associated with nonces
	items map[uint64]*action.SealedEnvelope
	// Priority Queue that stores all the nonces belonging to an account. Nonces are used as indices for action map
	ascQueue  ascNoncePriorityQueue
	descQueue descNoncePriorityQueue
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
		items:          make(map[uint64]*action.SealedEnvelope),
		ascQueue:       ascNoncePriorityQueue{},
		descQueue:      descNoncePriorityQueue{},
		pendingNonce:   pendingNonce,
		pendingBalance: make(map[uint64]*big.Int),
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

func (q *actQueue) NextAction() (bool, *big.Int) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.ascQueue) == 0 {
		return false, nil
	}
	return q.pendingNonce > q.accountNonce, q.items[q.ascQueue[0].nonce].GasFeeCap()
}

// Put inserts a new action into the map, also updating the queue's nonce index
func (q *actQueue) Put(act *action.SealedEnvelope) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	nonce := act.Nonce()

	if cost, _ := act.Cost(); q.getPendingBalanceAtNonce(nonce).Cmp(cost) < 0 {
		return action.ErrInsufficientFunds
	}

	if actInPool, exist := q.items[nonce]; exist {
		// act of higher gas price can cut in line
		if nonce < q.pendingNonce && act.GasFeeCap().Cmp(actInPool.GasFeeCap()) != 1 {
			return errors.Wrapf(action.ErrReplaceUnderpriced, "gas fee cap %s < %s", act.GasFeeCap(), actInPool.GasFeeCap())
		}
		// 2x bumps in gas price are allowed for blob tx
		isPrevBlobTx, isBlobTx := len(actInPool.BlobHashes()) > 0, len(act.BlobHashes()) > 0
		if isPrevBlobTx {
			if !isBlobTx {
				return errors.Wrap(action.ErrReplaceUnderpriced, "blob tx can only replace blob tx")
			}
			var (
				priceBump        = big.NewInt(2)
				minGasFeeCap     = new(big.Int).Mul(actInPool.GasFeeCap(), priceBump)
				minGasTipCap     = new(big.Int).Mul(actInPool.GasTipCap(), priceBump)
				minBlobGasFeeCap = new(big.Int).Mul(actInPool.BlobGasFeeCap(), priceBump)
			)
			switch {
			case act.GasFeeCap().Cmp(minGasFeeCap) < 0:
				return errors.Wrapf(action.ErrReplaceUnderpriced, "gas fee cap %s < %s", act.GasFeeCap(), minGasFeeCap)
			case act.GasTipCap().Cmp(minGasTipCap) < 0:
				return errors.Wrapf(action.ErrReplaceUnderpriced, "gas tip cap %s < %s", act.GasTipCap(), minGasTipCap)
			case act.BlobGasFeeCap().Cmp(minBlobGasFeeCap) < 0:
				return errors.Wrapf(action.ErrReplaceUnderpriced, "blob gas fee cap %s < %s", act.BlobGasFeeCap(), minBlobGasFeeCap)
			}
		}
		// update action in q.items and q.index
		q.items[nonce] = act
		for i := range q.ascQueue {
			if q.ascQueue[i].nonce == nonce {
				q.ascQueue[i].deadline = q.clock.Now().Add(q.ttl)
				break
			}
		}
		q.updateFromNonce(nonce)
		q.ap.removeInvalidActs([]*action.SealedEnvelope{actInPool})
		return nil
	}
	nttl := &nonceWithTTL{nonce: nonce, deadline: q.clock.Now().Add(q.ttl)}
	heap.Push(&q.ascQueue, nttl)
	heap.Push(&q.descQueue, nttl)
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

// UpdateQueue updates the pending nonce and balance of the queue
func (q *actQueue) UpdateQueue() []*action.SealedEnvelope {
	q.mu.Lock()
	defer q.mu.Unlock()
	// First remove all timed out actions
	removedFromQueue := q.cleanTimeout()
	// Now, starting from the current pending nonce, incrementally find the next pending nonce
	q.updateFromNonce(q.pendingNonce)
	return removedFromQueue
}

func (q *actQueue) cleanTimeout() []*action.SealedEnvelope {
	if q.ttl == 0 {
		return []*action.SealedEnvelope{}
	}
	var (
		removedFromQueue = make([]*action.SealedEnvelope, 0)
		timeNow          = q.clock.Now()
		size             = len(q.ascQueue)
	)
	for i := 0; i < size; {
		nonce := q.ascQueue[i].nonce
		if timeNow.After(q.ascQueue[i].deadline) && nonce > q.pendingNonce {
			removedFromQueue = append(removedFromQueue, q.items[nonce])
			delete(q.items, nonce)
			delete(q.pendingBalance, nonce)
			q.ascQueue[i] = q.ascQueue[size-1]
			size--
			continue
		}
		i++
	}
	for i := 0; i < size; i++ {
		q.descQueue[i] = q.ascQueue[i]
		q.descQueue[i].ascIdx = i
		q.descQueue[i].descIdx = i
	}
	q.ascQueue = q.ascQueue[:size]
	q.descQueue = q.descQueue[:size]
	// using heap.Init is better here, more detail to see BenchmarkHeapInitAndRemove
	heap.Init(&q.ascQueue)
	heap.Init(&q.descQueue)
	return removedFromQueue
}

// UpdateAccountState updates the account's nonce and balance and cleans confirmed actions
func (q *actQueue) UpdateAccountState(nonce uint64, balance *big.Int) []*action.SealedEnvelope {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pendingNonce = nonce
	q.pendingBalance = make(map[uint64]*big.Int)
	q.accountNonce = nonce
	q.accountBalance.Set(balance)
	var removed []*action.SealedEnvelope
	// Pop off priority queue and delete corresponding entries from map
	for q.ascQueue.Len() > 0 && (q.ascQueue)[0].nonce < q.accountNonce {
		nttl := heap.Pop(&q.ascQueue).(*nonceWithTTL)
		heap.Remove(&q.descQueue, nttl.descIdx)
		nonce := nttl.nonce
		removed = append(removed, q.items[nonce])
		delete(q.items, nonce)

	}
	return removed
}

// AccountState returns the current account's nonce and balance
func (q *actQueue) AccountState() (uint64, *big.Int) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.accountNonce, new(big.Int).Set(q.accountBalance)
}

// PendingNonce returns the current pending nonce of the queue
func (q *actQueue) PendingNonce() uint64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.pendingNonce
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
	q.items = make(map[uint64]*action.SealedEnvelope)
	q.ascQueue = ascNoncePriorityQueue{}
	q.descQueue = descNoncePriorityQueue{}
	q.pendingNonce = 0
	q.pendingBalance = make(map[uint64]*big.Int)
	q.accountNonce = 0
	q.accountBalance = big.NewInt(0)
}

// PendingActs creates a consecutive nonce-sorted slice of actions
func (q *actQueue) PendingActs(ctx context.Context) []*action.SealedEnvelope {
	if q.Len() == 0 {
		return nil
	}
	addr, err := address.FromString(q.address)
	if err != nil {
		log.L().Error("Error when getting the address", zap.String("address", q.address), zap.Error(err))
		return nil
	}
	// TODO: no need to refetch confirmed state, leave it to block builder to validate
	confirmedState, err := accountutil.AccountState(ctx, q.ap.sf, addr)
	if err != nil {
		log.L().Error("Error when getting the nonce", zap.String("address", q.address), zap.Error(err))
		return nil
	}

	var (
		nonce   uint64
		balance = new(big.Int).Set(confirmedState.Balance)
		acts    = make([]*action.SealedEnvelope, 0, len(q.items))
	)
	if protocol.MustGetFeatureCtx(ctx).UseZeroNonceForFreshAccount {
		nonce = confirmedState.PendingNonceConsideringFreshAccount()
	} else {
		nonce = confirmedState.PendingNonce()
	}
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
func (q *actQueue) AllActs() []*action.SealedEnvelope {
	q.mu.RLock()
	defer q.mu.RUnlock()
	acts := make([]*action.SealedEnvelope, 0, len(q.items))
	if len(q.items) == 0 {
		return acts
	}
	for _, nonce := range q.ascQueue {
		acts = append(acts, q.items[nonce.nonce])
	}
	return acts
}

func (q *actQueue) PopActionWithLargestNonce() *action.SealedEnvelope {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	itemMeta := heap.Pop(&q.descQueue).(*nonceWithTTL)
	heap.Remove(&q.ascQueue, itemMeta.ascIdx)
	item := q.items[itemMeta.nonce]
	delete(q.items, itemMeta.nonce)
	q.updateFromNonce(itemMeta.nonce)
	return item
}
