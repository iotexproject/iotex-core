// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"container/heap"

	"github.com/iotexproject/iotex-core/action"
)

// ActionByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
// It's essentially a big root heap of actions
type ActionByPrice []action.Action

func (s ActionByPrice) Len() int           { return len(s) }
func (s ActionByPrice) Less(i, j int) bool { return s[i].GasPrice().Cmp(s[j].GasPrice()) > 0 }
func (s ActionByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Push define the push function of heap
func (s *ActionByPrice) Push(x interface{}) {
	*s = append(*s, x.(action.Action))
}

// Pop define the pop function of heap
func (s *ActionByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// ActionIterator define the interface of action iterator
type ActionIterator interface {
	TopAction() action.Action
	PopAction()
	LoadNextAction()
}

type actionIterator struct {
	accountActs map[string][]action.Action
	heads       ActionByPrice
}

// NewActionIterator return a new action iterator
func NewActionIterator(accountActs map[string][]action.Action) ActionIterator {
	heads := make(ActionByPrice, 0, len(accountActs))
	for sender, accActs := range accountActs {
		heads = append(heads, accActs[0])
		if len(accActs) > 1 {
			accountActs[sender] = accActs[1:]
		}
	}
	heap.Init(&heads)
	return &actionIterator{
		accountActs: accountActs,
		heads:       heads,
	}
}

// TopAction return the top action(largest price) within the heap
func (ai *actionIterator) TopAction() action.Action {
	if len(ai.heads) == 0 {
		return nil
	}
	return ai.heads[0]
}

// PopAction pop top action
func (ai *actionIterator) PopAction() {
	heap.Pop(&ai.heads)
}

// LoadNextAction load next action of account of top action
func (ai *actionIterator) LoadNextAction() {
	sender := ai.heads[0].SrcAddr()
	if actions, ok := ai.accountActs[sender]; ok && len(actions) > 0 {
		ai.heads[0], ai.accountActs[sender] = actions[0], actions[1:]
		heap.Fix(&ai.heads, 0)
	} else {
		heap.Pop(&ai.heads)
	}
}
