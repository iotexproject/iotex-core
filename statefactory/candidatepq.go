// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package statefactory

import (
	"container/heap"
	"math/big"
)

// Candidate is used in the heap
type Candidate struct {
	Address  string
	Votes    *big.Int
	PubKey   []byte
	minIndex int
	maxIndex int
}

// CandidateMinPQ is a min Priority Queue
type CandidateMinPQ []*Candidate

// CandidateMaxPQ is a max Priority Queue
type CandidateMaxPQ []*Candidate

func (pq CandidateMinPQ) Len() int {
	return len(pq)
}

func (pq CandidateMinPQ) Less(i, j int) bool {
	return pq[i].Votes.Cmp(pq[j].Votes) < 0
}

func (pq CandidateMinPQ) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].minIndex = i
	pq[j].minIndex = j
}

// Push pushes a candidate in the heap
func (pq *CandidateMinPQ) Push(x interface{}) {
	n := len(*pq)
	candidate := x.(*Candidate)
	candidate.minIndex = n
	*pq = append(*pq, candidate)
}

// Pop pops a candidate in the heap
func (pq *CandidateMinPQ) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.minIndex = -1
	*pq = old[0 : n-1]
	return item
}

// Top returns the candidate with smallest votes
func (pq *CandidateMinPQ) Top() interface{} {
	return (*pq)[0]
}

func (pq *CandidateMinPQ) update(candidate *Candidate, newVote *big.Int) {
	candidate.Votes = newVote
	heap.Fix(pq, candidate.minIndex)
}

// CandidateList return a list of candidates
func (pq *CandidateMinPQ) CandidateList() []*Candidate {
	candidates := make([]*Candidate, len(*pq))
	for i := 0; i < len(*pq); i++ {
		candidates[i] = (*pq)[i]
	}
	return candidates
}

func (pq *CandidateMinPQ) exist(address string) *Candidate {
	for i := 0; i < len(*pq); i++ {
		if (*pq)[i].Address == address {
			return (*pq)[i]
		}
	}
	return nil
}

func (pq CandidateMaxPQ) Len() int {
	return len(pq)
}

func (pq CandidateMaxPQ) Less(i, j int) bool {
	return pq[i].Votes.Cmp(pq[j].Votes) > 0
}

func (pq CandidateMaxPQ) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].maxIndex = i
	pq[j].maxIndex = j
}

// Push pushes a candidate in the heap
func (pq *CandidateMaxPQ) Push(x interface{}) {
	n := len(*pq)
	candidate := x.(*Candidate)
	candidate.maxIndex = n
	*pq = append(*pq, candidate)
}

// Pop pops a candidate in the heap
func (pq *CandidateMaxPQ) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.maxIndex = -1
	*pq = old[0 : n-1]
	return item
}

// Top return the candidate with highest votes
func (pq *CandidateMaxPQ) Top() interface{} {
	return (*pq)[0]
}

func (pq *CandidateMaxPQ) update(candidate *Candidate, newVote *big.Int) {
	candidate.Votes = newVote
	heap.Fix(pq, candidate.maxIndex)
}

// CandidateList return a list of candidates
func (pq *CandidateMaxPQ) CandidateList() []*Candidate {
	candidates := make([]*Candidate, len(*pq))
	for i := 0; i < len(*pq); i++ {
		candidates[i] = (*pq)[i]
	}
	return candidates
}

func (pq *CandidateMaxPQ) exist(address string) *Candidate {
	for i := 0; i < len(*pq); i++ {
		if (*pq)[i].Address == address {
			return (*pq)[i]
		}
	}
	return nil
}
