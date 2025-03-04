// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actioniterator

import (
	"bytes"
	"container/heap"

	"github.com/iotexproject/iotex-core/v2/action"
)

// ActionByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
// It's essentially a big root heap of actions
type actionByPrice []*action.SealedEnvelope

func (s actionByPrice) Len() int { return len(s) }
func (s actionByPrice) Less(i, j int) bool {
	switch s[i].GasPrice().Cmp(s[j].GasPrice()) {
	case 1:
		return true
	case 0:
		hi, _ := s[i].Hash()
		hj, _ := s[j].Hash()
		return bytes.Compare(hi[:], hj[:]) > 0
	default:
		return false
	}
}

func (s actionByPrice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Push define the push function of heap
func (s *actionByPrice) Push(x interface{}) {
	*s = append(*s, x.(*action.SealedEnvelope))
}

// Pop define the pop function of heap
func (s *actionByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// ActionIterator define the interface of action iterator
type ActionIterator interface {
	Next() (*action.SealedEnvelope, bool)
	PopAccount()
}

type actionIterator struct {
	accountActs map[string][]*action.SealedEnvelope
	heads       actionByPrice
}

// NewActionIterator return a new action iterator
func NewActionIterator(accountActs map[string][]*action.SealedEnvelope) ActionIterator {
	heads := make(actionByPrice, 0, len(accountActs))
	for sender, accActs := range accountActs {
		if len(accActs) == 0 {
			continue
		}

		heads = append(heads, accActs[0])
		if len(accActs) > 1 {
			accountActs[sender] = accActs[1:]
		} else {
			accountActs[sender] = []*action.SealedEnvelope{}
		}
	}
	heap.Init(&heads)
	return &actionIterator{
		accountActs: accountActs,
		heads:       heads,
	}
}

// loadNextActionForTopAccount load next action of account of top action
func (ai *actionIterator) loadNextActionForTopAccount() {
	callerAddrStr := ai.heads[0].SenderAddress().String()
	if actions, ok := ai.accountActs[callerAddrStr]; ok && len(actions) > 0 {
		ai.heads[0], ai.accountActs[callerAddrStr] = actions[0], actions[1:]
		heap.Fix(&ai.heads, 0)
	} else {
		heap.Pop(&ai.heads)
	}
}

// Next load next action of account of top action
func (ai *actionIterator) Next() (*action.SealedEnvelope, bool) {
	if len(ai.heads) == 0 {
		return nil, false
	}

	headAction := ai.heads[0]
	ai.loadNextActionForTopAccount()
	return headAction, true
}

// PopAccount will remove all actions related to this account
func (ai *actionIterator) PopAccount() {
	if len(ai.heads) != 0 {
		heap.Pop(&ai.heads)
	}
}
