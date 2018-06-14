// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

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
type CandidateMinPQ struct {
	Capacity int
	pq       []*Candidate
}

// CandidateMaxPQ is a max Priority Queue
type CandidateMaxPQ struct {
	Capacity int
	pq       []*Candidate
}

func (pqStruct CandidateMinPQ) Len() int {
	return len(pqStruct.pq)
}

func (pqStruct CandidateMinPQ) Less(i, j int) bool {
	pq := pqStruct.pq
	return pq[i].Votes.Cmp(pq[j].Votes) < 0
}

func (pqStruct CandidateMinPQ) Swap(i, j int) {
	pq := pqStruct.pq
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].minIndex = i
	pq[j].minIndex = j
}

// Push pushes a candidate in the heap
func (pqStruct *CandidateMinPQ) Push(x interface{}) {
	pq := &pqStruct.pq
	n := len(*pq)
	candidate := x.(*Candidate)
	candidate.minIndex = n
	*pq = append(*pq, candidate)
}

// Pop pops a candidate in the heap
func (pqStruct *CandidateMinPQ) Pop() interface{} {
	pq := &pqStruct.pq
	old := *pq
	n := len(old)
	item := old[n-1]
	item.minIndex = -1
	*pq = old[0 : n-1]
	return item
}

// Top returns the candidate with smallest votes
func (pqStruct *CandidateMinPQ) Top() interface{} {
	if pqStruct.Len() == 0 {
		return nil
	}
	return pqStruct.pq[0]
}

func (pqStruct *CandidateMinPQ) update(candidate *Candidate, newVote *big.Int) {
	candidate.Votes = newVote
	heap.Fix(pqStruct, candidate.minIndex)
}

// CandidateList return a list of candidates
func (pqStruct *CandidateMinPQ) CandidateList() []*Candidate {
	pq := pqStruct.pq
	candidates := make([]*Candidate, len(pq))
	for i := 0; i < len(pq); i++ {
		candidates[i] = pq[i]
	}
	return candidates
}

func (pqStruct *CandidateMinPQ) exist(address string) *Candidate {
	pq := pqStruct.pq
	for i := 0; i < len(pq); i++ {
		if pq[i].Address == address {
			return pq[i]
		}
	}
	return nil
}

func (pqStruct *CandidateMinPQ) shouldTake(vote *big.Int) bool {
	return pqStruct.Len() < pqStruct.Capacity || vote.Cmp(pqStruct.Top().(*Candidate).Votes) >= 0
}

func (pqStruct CandidateMaxPQ) Len() int {
	return len(pqStruct.pq)
}

func (pqStruct CandidateMaxPQ) Less(i, j int) bool {
	pq := pqStruct.pq
	return pq[i].Votes.Cmp(pq[j].Votes) > 0
}

func (pqStruct CandidateMaxPQ) Swap(i, j int) {
	pq := pqStruct.pq
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].maxIndex = i
	pq[j].maxIndex = j
}

// Push pushes a candidate in the heap
func (pqStruct *CandidateMaxPQ) Push(x interface{}) {
	pq := &pqStruct.pq
	n := len(*pq)
	candidate := x.(*Candidate)
	candidate.maxIndex = n
	*pq = append(*pq, candidate)
}

// Pop pops a candidate in the heap
func (pqStruct *CandidateMaxPQ) Pop() interface{} {
	pq := &pqStruct.pq
	old := *pq
	n := len(old)
	item := old[n-1]
	item.maxIndex = -1
	*pq = old[0 : n-1]
	return item
}

// Top return the candidate with highest votes
func (pqStruct *CandidateMaxPQ) Top() interface{} {
	if pqStruct.Len() == 0 {
		return nil
	}
	return pqStruct.pq[0]
}

func (pqStruct *CandidateMaxPQ) update(candidate *Candidate, newVote *big.Int) {
	candidate.Votes = newVote
	heap.Fix(pqStruct, candidate.maxIndex)
}

// CandidateList return a list of candidates
func (pqStruct *CandidateMaxPQ) CandidateList() []*Candidate {
	pq := pqStruct.pq
	candidates := make([]*Candidate, len(pq))
	for i := 0; i < len(pq); i++ {
		candidates[i] = pq[i]
	}
	return candidates
}
