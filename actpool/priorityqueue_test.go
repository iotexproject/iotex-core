// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAscNoncePriorityQueueOrder verifies the min-heap pops nonces in ascending order
// and keeps ascIdx consistent with the slice position after every heap mutation.
func TestAscNoncePriorityQueueOrder(t *testing.T) {
	r := require.New(t)
	pq := &ascNoncePriorityQueue{}
	for _, n := range []uint64{5, 1, 4, 2, 3} {
		heap.Push(pq, &nonceWithTTL{nonce: n})
	}
	r.Equal(5, pq.Len())
	// ascIdx of every element must equal its position in the backing slice
	for i, it := range *pq {
		r.Equal(i, it.ascIdx)
	}
	// the smallest nonce must be at the root
	r.Equal(uint64(1), (*pq)[0].nonce)
	// popping yields strictly ascending nonces
	var got []uint64
	for pq.Len() > 0 {
		got = append(got, heap.Pop(pq).(*nonceWithTTL).nonce)
	}
	r.Equal([]uint64{1, 2, 3, 4, 5}, got)
}

// TestDescNoncePriorityQueueOrder verifies the max-heap pops nonces in descending order
// and keeps descIdx consistent with the slice position after every heap mutation.
func TestDescNoncePriorityQueueOrder(t *testing.T) {
	r := require.New(t)
	pq := &descNoncePriorityQueue{}
	for _, n := range []uint64{5, 1, 4, 2, 3} {
		heap.Push(pq, &nonceWithTTL{nonce: n})
	}
	r.Equal(5, pq.Len())
	for i, it := range *pq {
		r.Equal(i, it.descIdx)
	}
	// the largest nonce must be at the root
	r.Equal(uint64(5), (*pq)[0].nonce)
	var got []uint64
	for pq.Len() > 0 {
		got = append(got, heap.Pop(pq).(*nonceWithTTL).nonce)
	}
	r.Equal([]uint64{5, 4, 3, 2, 1}, got)
}

// TestNoncePriorityQueuePopEmpty ensures Pop on an empty queue returns nil (guard branch)
// rather than panicking.
func TestNoncePriorityQueuePopEmpty(t *testing.T) {
	r := require.New(t)
	asc := &ascNoncePriorityQueue{}
	r.Nil(asc.Pop())
	desc := &descNoncePriorityQueue{}
	r.Nil(desc.Pop())
}

// TestNoncePriorityQueuePushWrongType exercises the type-assertion guard in Push:
// a non-*nonceWithTTL value must be silently ignored, leaving the queue untouched.
func TestNoncePriorityQueuePushWrongType(t *testing.T) {
	r := require.New(t)
	asc := &ascNoncePriorityQueue{}
	asc.Push(42) // not a *nonceWithTTL
	r.Equal(0, asc.Len())
	desc := &descNoncePriorityQueue{}
	desc.Push("foo")
	r.Equal(0, desc.Len())
}

// TestNoncePriorityQueueRemoveKeepsIndex verifies that heap.Remove of an interior
// element keeps ascIdx/descIdx of the remaining elements aligned with their slot,
// which is what actQueue relies on when it removes by stored index. Both heaps are
// exercised because actQueue removes by ascIdx from one and by descIdx from the other.
func TestNoncePriorityQueueRemoveKeepsIndex(t *testing.T) {
	r := require.New(t)
	// 9 elements so the removed node is an interior node with children, forcing
	// heap.Remove to swap in the last element and percolate, which is the path that
	// rewrites the stored indexes.
	nonces := []uint64{10, 20, 30, 40, 50, 60, 70, 80, 90}
	t.Run("asc heap interior remove keeps ascIdx", func(t *testing.T) {
		asc := &ascNoncePriorityQueue{}
		items := make(map[uint64]*nonceWithTTL)
		for _, n := range nonces {
			it := &nonceWithTTL{nonce: n}
			items[n] = it
			heap.Push(asc, it)
		}
		// nonce 20 sits at an interior slot (index 1) with children
		r.Greater(items[20].ascIdx*2+1, 0)
		r.Less(items[20].ascIdx, asc.Len()-1) // not the last leaf
		heap.Remove(asc, items[20].ascIdx)
		r.Equal(8, asc.Len())
		for i, it := range *asc {
			r.Equal(i, it.ascIdx, "ascIdx must track slice position after Remove")
		}
		var got []uint64
		for asc.Len() > 0 {
			got = append(got, heap.Pop(asc).(*nonceWithTTL).nonce)
		}
		r.Equal([]uint64{10, 30, 40, 50, 60, 70, 80, 90}, got)
	})
	t.Run("desc heap interior remove keeps descIdx", func(t *testing.T) {
		desc := &descNoncePriorityQueue{}
		items := make(map[uint64]*nonceWithTTL)
		for _, n := range nonces {
			it := &nonceWithTTL{nonce: n}
			items[n] = it
			heap.Push(desc, it)
		}
		// nonce 80 sits at an interior slot with children in the max-heap
		r.Less(items[80].descIdx, desc.Len()-1) // not the last leaf
		heap.Remove(desc, items[80].descIdx)
		r.Equal(8, desc.Len())
		for i, it := range *desc {
			r.Equal(i, it.descIdx, "descIdx must track slice position after Remove")
		}
		var got []uint64
		for desc.Len() > 0 {
			got = append(got, heap.Pop(desc).(*nonceWithTTL).nonce)
		}
		r.Equal([]uint64{90, 70, 60, 50, 40, 30, 20, 10}, got)
	})
}
