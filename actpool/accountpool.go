// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"container/heap"
	"math/big"
	"time"

	"github.com/iotexproject/iotex-core/v2/action"
)

type (
	accountItem struct {
		index    int
		actQueue ActQueue
	}

	accountPriorityQueue []*accountItem

	accountPool struct {
		accounts      map[string]*accountItem
		priorityQueue accountPriorityQueue
	}
)

func newAccountPool() *accountPool {
	ap := &accountPool{
		priorityQueue: accountPriorityQueue{},
		accounts:      map[string]*accountItem{},
	}
	heap.Init(&ap.priorityQueue)

	return ap
}

func (ap *accountPool) Account(addr string) ActQueue {
	if account, ok := ap.accounts[addr]; ok {
		return account.actQueue
	}
	return nil
}

func (ap *accountPool) PopAccount(addr string) ActQueue {
	if account, ok := ap.accounts[addr]; ok {
		heap.Remove(&ap.priorityQueue, account.index)
		delete(ap.accounts, addr)
		return account.actQueue
	}

	return nil
}

func (ap *accountPool) PutAction(
	addr string,
	actpool *actPool,
	pendingNonce uint64,
	confirmedBalance *big.Int,
	expiry time.Duration,
	act *action.SealedEnvelope,
) error {
	account, ok := ap.accounts[addr]
	if !ok {
		queue := NewActQueue(
			actpool,
			addr,
			pendingNonce,
			confirmedBalance,
			WithTimeOut(expiry),
		)
		if err := queue.Put(act); err != nil {
			return err
		}
		ap.accounts[addr] = &accountItem{
			index:    len(ap.accounts),
			actQueue: queue,
		}
		heap.Push(&ap.priorityQueue, ap.accounts[addr])
		return nil
	}

	if err := account.actQueue.Put(act); err != nil {
		return err
	}
	heap.Fix(&ap.priorityQueue, account.index)

	return nil
}

func (ap *accountPool) PopPeek() *action.SealedEnvelope {
	if len(ap.accounts) == 0 {
		return nil
	}
	act := ap.priorityQueue[0].actQueue.PopActionWithLargestNonce()
	heap.Fix(&ap.priorityQueue, 0)

	return act
}

func (ap *accountPool) Range(callback func(addr string, acct ActQueue)) {
	for addr, account := range ap.accounts {
		callback(addr, account.actQueue)
	}
	heap.Init(&ap.priorityQueue)
}

func (ap *accountPool) DeleteIfEmpty(addr string) {
	account, ok := ap.accounts[addr]
	if !ok {
		return
	}
	if account.actQueue.Empty() {
		heap.Remove(&ap.priorityQueue, account.index)
		delete(ap.accounts, addr)
	}
}

func (aq accountPriorityQueue) Len() int { return len(aq) }
func (aq accountPriorityQueue) Less(i, j int) bool {
	return newEvictionRank(aq[i].actQueue).less(newEvictionRank(aq[j].actQueue))
}

// evictionRank is the per-account key that orders the account priority queue.
// The account at the top of the queue (the "least" one) is the one whose
// largest-nonce action is dropped first when the pool is full.
type evictionRank struct {
	hasNext bool
	settled bool
	gp      *big.Int
	queued  int
}

func newEvictionRank(q ActQueue) evictionRank {
	settled, gp := q.NextAction()
	return evictionRank{
		hasNext: gp != nil,
		settled: settled,
		gp:      gp,
		queued:  q.NumQueuedActs(),
	}
}

// less reports whether a should be evicted before b:
//  1. an account with no action is never preferred for eviction
//  2. an account holding more non-executable (queued) actions goes first, so
//     actions that cannot be included in the next block are dropped before
//     any executable one, regardless of the fee of the account's head action
//  3. an account without an executable action goes before one with
//  4. lower head gas fee cap goes first
func (a evictionRank) less(b evictionRank) bool {
	if !b.hasNext {
		return true
	}
	if !a.hasNext {
		return false
	}
	if a.queued != b.queued {
		return a.queued > b.queued
	}
	if !a.settled && b.settled {
		return true
	}
	if !b.settled && a.settled {
		return false
	}
	return a.gp.Cmp(b.gp) < 0
}

func (aq accountPriorityQueue) Swap(i, j int) {
	aq[i], aq[j] = aq[j], aq[i]
	aq[i].index = i
	aq[j].index = j
}

func (aq *accountPriorityQueue) Push(x interface{}) {
	if in, ok := x.(*accountItem); ok {
		in.index = len(*aq)
		*aq = append(*aq, in)
	}
}

func (aq *accountPriorityQueue) Pop() interface{} {
	old := *aq
	n := len(old)
	if n == 0 {
		return nil
	}
	x := old[n-1]
	old[n-1] = nil // avoid memory leak
	*aq = old[0 : n-1]
	return x
}
