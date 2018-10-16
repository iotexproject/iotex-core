// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package actpool

import (
	"container/heap"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
)

func TestNoncePriorityQueue(t *testing.T) {
	require := require.New(t)
	pq := noncePriorityQueue{}
	// Push four dummy nonce to the queue
	heap.Push(&pq, uint64(1))
	heap.Push(&pq, uint64(3))
	heap.Push(&pq, uint64(2))
	// Test Pop implementation
	i := uint64(1)
	for pq.Len() > 0 {
		nonce := heap.Pop(&pq).(uint64)
		require.Equal(i, nonce)
		i++
	}
	// Repush the four dummy nonce back to the queue
	heap.Push(&pq, uint64(3))
	heap.Push(&pq, uint64(2))
	heap.Push(&pq, uint64(1))
	// Test built-in Remove implementation
	// Remove a random nonce from noncePriorityQueue
	rand.Seed(time.Now().UnixNano())
	heap.Remove(&pq, rand.Intn(pq.Len()))
	t.Log("After randomly removing a dummy nonce, the remaining dummy nonces in the order of popped are as follows:")
	for pq.Len() > 0 {
		nonce := heap.Pop(&pq).(uint64)
		t.Log(nonce)
		t.Log()
	}
}

func TestActQueue_Put(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	vote1, err := action.NewVote(2, "1", "2", 0, big.NewInt(0))
	require.NoError(err)
	err = q.Put(vote1)
	require.NoError(err)
	require.Equal(uint64(2), q.index[0])
	require.NotNil(q.items[vote1.Nonce()])
	tsf2, err := action.NewTransfer(uint64(1), big.NewInt(100), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf2)
	require.NoError(err)
	require.Equal(uint64(1), heap.Pop(&q.index))
	require.Equal(tsf2, q.items[uint64(1)])
	require.Equal(uint64(2), heap.Pop(&q.index))
	require.Equal(vote1, q.items[uint64(2)])
	// tsf3 is a replacement transfer
	tsf3, err := action.NewTransfer(uint64(1), big.NewInt(1000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf3)
	require.NotNil(err)
}

func TestActQueue_FilterNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	tsf1, err := action.NewTransfer(uint64(1), big.NewInt(1), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	vote2, err := action.NewVote(2, "1", "2", 0, big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.NewTransfer(uint64(3), big.NewInt(1000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf1)
	require.NoError(err)
	err = q.Put(vote2)
	require.NoError(err)
	err = q.Put(tsf3)
	require.NoError(err)
	q.FilterNonce(uint64(3))
	require.Equal(1, len(q.items))
	require.Equal(uint64(3), q.index[0])
	require.Equal(tsf3, q.items[q.index[0]])
}

func TestActQueue_UpdateNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	tsf1, err := action.NewTransfer(uint64(1), big.NewInt(1), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.NewTransfer(uint64(3), big.NewInt(1000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.NewTransfer(uint64(4), big.NewInt(10000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.NewTransfer(uint64(6), big.NewInt(100000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	vote5, err := action.NewVote(2, "1", "2", 0, big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf1)
	require.NoError(err)
	err = q.Put(tsf2)
	require.NoError(err)
	err = q.Put(tsf3)
	require.NoError(err)
	err = q.Put(tsf4)
	require.NoError(err)
	q.pendingNonce = uint64(2)
	q.pendingBalance = big.NewInt(1000)
	err = q.Put(vote5)
	require.NoError(err)
	removed := q.UpdateQueue(uint64(2))
	require.Equal(uint64(4), q.pendingNonce)
	require.Equal([]action.Action{tsf3, tsf4}, removed)
}

func TestActQueue_PendingActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	vote1, err := action.NewVote(2, "1", "2", 0, big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.NewTransfer(uint64(3), big.NewInt(100), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.NewTransfer(uint64(5), big.NewInt(1000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.NewTransfer(uint64(6), big.NewInt(10000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.NewTransfer(uint64(7), big.NewInt(100000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(vote1)
	require.NoError(err)
	err = q.Put(tsf2)
	require.NoError(err)
	err = q.Put(tsf3)
	require.NoError(err)
	err = q.Put(tsf4)
	require.NoError(err)
	err = q.Put(tsf5)
	require.NoError(err)
	q.startNonce = 2
	q.pendingNonce = 4
	actions := q.PendingActs()
	require.Equal([]action.Action{vote1, tsf2}, actions)
}

func TestActQueue_AllActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	vote1, err := action.NewVote(1, "1", "2", 0, big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.NewTransfer(uint64(3), big.NewInt(1000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(vote1)
	require.NoError(err)
	err = q.Put(tsf3)
	require.NoError(err)
	actions := q.AllActs()
	require.Equal([]action.Action{vote1, tsf3}, actions)
}

func TestActQueue_removeActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	tsf1, err := action.NewTransfer(uint64(1), big.NewInt(100), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	vote2, err := action.NewVote(2, "1", "2", 0, big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.NewTransfer(uint64(3), big.NewInt(1000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf1)
	require.NoError(err)
	err = q.Put(vote2)
	require.NoError(err)
	err = q.Put(tsf3)
	require.NoError(err)
	removed := q.removeActs(0)
	require.Equal(0, len(q.index))
	require.Equal(0, len(q.items))
	require.Equal([]action.Action{tsf1, vote2, tsf3}, removed)

	tsf4, err := action.NewTransfer(uint64(4), big.NewInt(10000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.NewTransfer(uint64(5), big.NewInt(100000), "1", "2", nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	vote6, err := action.NewVote(6, "1", "2", 0, big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf4)
	require.NoError(err)
	err = q.Put(tsf5)
	require.NoError(err)
	err = q.Put(vote6)
	require.NoError(err)
	removed = q.removeActs(1)
	require.Equal(1, len(q.index))
	require.Equal(1, len(q.items))
	require.Equal([]action.Action{tsf5, vote6}, removed)
}
