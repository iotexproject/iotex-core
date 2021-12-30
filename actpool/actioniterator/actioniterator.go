// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actioniterator

import (
	"container/heap"

	"github.com/iotexproject/iotex-core/action"
)

// ActionByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
// It's essentially a big root heap of actions
type actionByPrice []action.SealedEnvelope

func (s actionByPrice) Len() int           { return len(s) }
func (s actionByPrice) Less(i, j int) bool { return s[i].GasPrice().Cmp(s[j].GasPrice()) > 0 }
func (s actionByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Push define the push function of heap
func (s *actionByPrice) Push(x interface{}) {
	*s = append(*s, x.(action.SealedEnvelope))
}

// Pop define the pop function of heap
func (s *actionByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// // ActionIterator define the interface of action iterator
// type ActionIterator interface {
// 	Next() (action.SealedEnvelope, bool)
// 	PopAccount()
// }

// ActionIterator define the interface of action iterator
type ActionIterator struct {
	accountActs map[string][]action.SealedEnvelope
	heads       actionByPrice
}

// NewActionIterator return a new action iterator
func NewActionIterator(accountActs map[string][]action.SealedEnvelope) *ActionIterator {
	heads := make(actionByPrice, 0, len(accountActs))
	for sender, accActs := range accountActs {
		if len(accActs) == 0 {
			continue
		}

		heads = append(heads, accActs[0])
		if len(accActs) > 1 {
			accountActs[sender] = accActs[1:]
		} else {
			accountActs[sender] = []action.SealedEnvelope{}
		}
	}
	heap.Init(&heads)
	return &ActionIterator{
		accountActs: accountActs,
		heads:       heads,
	}
}

// LoadNext load next action of account of top action
func (ai *ActionIterator) loadNextActionForTopAccount() {
	sender := ai.heads[0].SrcPubkey()
	callerAddrStr := sender.Address().String()
	if actions, ok := ai.accountActs[callerAddrStr]; ok && len(actions) > 0 {
		ai.heads[0], ai.accountActs[callerAddrStr] = actions[0], actions[1:]
		heap.Fix(&ai.heads, 0)
	} else {
		heap.Pop(&ai.heads)
	}
}

// Next load next action of account of top action
func (ai *ActionIterator) Next() (action.SealedEnvelope, bool) {
	if len(ai.heads) == 0 {
		return action.SealedEnvelope{}, false
	}

	headAction := ai.heads[0]
	ai.loadNextActionForTopAccount()
	return headAction, true
}

// PopAccount will remove all actions related to this account
func (ai *ActionIterator) PopAccount() {
	if len(ai.heads) != 0 {
		heap.Pop(&ai.heads)
	}
}
