// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package txpool

import (
	"container/heap"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain/action"
)

func TestNoncePriorityQueue(t *testing.T) {
	assert := assert.New(t)
	pq := noncePriorityQueue{}
	// Push four dummy nonce to the queue
	heap.Push(&pq, uint64(1))
	heap.Push(&pq, uint64(3))
	heap.Push(&pq, uint64(2))
	// Test Pop implementation
	i := uint64(1)
	for pq.Len() > 0 {
		nonce := heap.Pop(&pq).(uint64)
		assert.Equal(i, nonce)
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

func TestTsfQueue_Put(t *testing.T) {
	assert := assert.New(t)
	q := NewTsfQueue().(*tsfQueue)
	tsf1 := action.Transfer{Nonce: uint64(2), Amount: big.NewInt(10)}
	q.Put(&tsf1)
	assert.Equal(uint64(2), q.index[0])
	assert.NotNil(q.items[tsf1.Nonce])
	tsf2 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(100)}
	q.Put(&tsf2)
	assert.Equal(uint64(1), heap.Pop(&q.index))
	assert.Equal(&tsf2, q.items[uint64(1)])
	assert.Equal(uint64(2), heap.Pop(&q.index))
	assert.Equal(&tsf1, q.items[uint64(2)])
	// tsf3 is a replacement transfer
	tsf3 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(1000)}
	err := q.Put(&tsf3)
	assert.NotNil(err)
}

func TestTsfQueue_FilterNonce(t *testing.T) {
	assert := assert.New(t)
	q := NewTsfQueue().(*tsfQueue)
	tsf1 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(1)}
	tsf2 := action.Transfer{Nonce: uint64(2), Amount: big.NewInt(100)}
	tsf3 := action.Transfer{Nonce: uint64(3), Amount: big.NewInt(1000)}
	q.Put(&tsf1)
	q.Put(&tsf2)
	q.Put(&tsf3)
	q.FilterNonce(uint64(3))
	assert.Equal(1, len(q.items))
	assert.Equal(uint64(3), q.index[0])
	assert.Equal(&tsf3, q.items[q.index[0]])
}

func TestTsfQueue_UpdateNonce(t *testing.T) {
	assert := assert.New(t)
	q := NewTsfQueue().(*tsfQueue)
	tsf1 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(1)}
	tsf2 := action.Transfer{Nonce: uint64(3), Amount: big.NewInt(1000)}
	tsf3 := action.Transfer{Nonce: uint64(4), Amount: big.NewInt(10000)}
	tsf4 := action.Transfer{Nonce: uint64(6), Amount: big.NewInt(100000)}
	tsf5 := action.Transfer{Nonce: uint64(2), Amount: big.NewInt(100)}
	q.Put(&tsf1)
	q.Put(&tsf2)
	q.Put(&tsf3)
	q.Put(&tsf4)
	q.confirmedNonce = uint64(2)
	q.pendingNonce = uint64(2)
	q.pendingBalance = big.NewInt(1100)
	q.Put(&tsf5)
	q.UpdateNonce(uint64(2))
	assert.Equal(uint64(5), q.pendingNonce)
	assert.Equal(uint64(4), q.confirmedNonce)
}

func TestTsfQueue_ConfirmedTsfs(t *testing.T) {
	assert := assert.New(t)
	q := NewTsfQueue().(*tsfQueue)
	tsf1 := action.Transfer{Nonce: uint64(2), Amount: big.NewInt(1)}
	tsf2 := action.Transfer{Nonce: uint64(3), Amount: big.NewInt(100)}
	tsf3 := action.Transfer{Nonce: uint64(4), Amount: big.NewInt(1000)}
	tsf4 := action.Transfer{Nonce: uint64(6), Amount: big.NewInt(10000)}
	tsf5 := action.Transfer{Nonce: uint64(7), Amount: big.NewInt(100000)}
	q.Put(&tsf1)
	q.Put(&tsf2)
	q.Put(&tsf3)
	q.Put(&tsf4)
	q.Put(&tsf5)
	q.confirmedNonce = 4
	tsfs := q.ConfirmedTsfs()
	assert.Equal([]*action.Transfer{&tsf1, &tsf2}, tsfs)
}
