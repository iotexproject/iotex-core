// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import "time"

type (
	nonceWithTTL struct {
		ascIdx   int
		descIdx  int
		nonce    uint64
		deadline time.Time
	}

	ascNoncePriorityQueue  []*nonceWithTTL
	descNoncePriorityQueue []*nonceWithTTL
)

func (h ascNoncePriorityQueue) Len() int           { return len(h) }
func (h ascNoncePriorityQueue) Less(i, j int) bool { return h[i].nonce < h[j].nonce }
func (h ascNoncePriorityQueue) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].ascIdx = i
	h[j].ascIdx = j
}

func (h *ascNoncePriorityQueue) Push(x interface{}) {
	if in, ok := x.(*nonceWithTTL); ok {
		in.ascIdx = len(*h)
		*h = append(*h, in)
	}
}

func (h *ascNoncePriorityQueue) Pop() interface{} {
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

func (h descNoncePriorityQueue) Len() int           { return len(h) }
func (h descNoncePriorityQueue) Less(i, j int) bool { return h[i].nonce > h[j].nonce }
func (h descNoncePriorityQueue) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].descIdx = i
	h[j].descIdx = j
}

func (h *descNoncePriorityQueue) Push(x interface{}) {
	if in, ok := x.(*nonceWithTTL); ok {
		in.descIdx = len(*h)
		*h = append(*h, in)
	}
}

func (h *descNoncePriorityQueue) Pop() interface{} {
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
