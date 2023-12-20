// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"container/heap"
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

func (ap *accountPool) Peek() ActQueue {
	if len(ap.accounts) == 0 {
		return nil
	}
	return ap.priorityQueue[0].actQueue
}

func (ap *accountPool) Range(callback func(addr string, acct ActQueue)) {
	for addr, account := range ap.accounts {
		callback(addr, account.actQueue)
	}
}

func (ap *accountPool) AddAccount(addr string, acct ActQueue) {
	ap.accounts[addr] = &accountItem{
		index:    len(ap.accounts),
		actQueue: acct,
	}
	heap.Push(&ap.priorityQueue, ap.accounts[addr])
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
	is, igp := aq[i].actQueue.NextAction()
	js, jgp := aq[j].actQueue.NextAction()
	if !is && js {
		return true
	}
	if !js && is {
		return false
	}

	return igp.Cmp(jgp) < 0
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
