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

	trx "github.com/iotexproject/iotex-core/blockchain/trx"
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

func TestTxQueue_Put(t *testing.T) {
	assert := assert.New(t)
	q := NewTxQueue()
	tx1 := trx.Tx{Nonce: uint64(2), Amount: big.NewInt(10)}
	q.Put(&tx1)
	assert.Equal(uint64(2), q.index[0])
	assert.NotNil(q.items[tx1.Nonce])
	tx2 := trx.Tx{Nonce: uint64(1), Amount: big.NewInt(100)}
	q.Put(&tx2)
	assert.Equal(uint64(1), heap.Pop(&q.index))
	assert.Equal(&tx2, q.items[uint64(1)])
	assert.Equal(uint64(2), heap.Pop(&q.index))
	assert.Equal(&tx1, q.items[uint64(2)])
	// tx3 is a replacement transaction
	tx3 := trx.Tx{Nonce: uint64(1), Amount: big.NewInt(1000)}
	err := q.Put(&tx3)
	assert.NotNil(err)
}

func TestTxQueue_FilterNonce(t *testing.T) {
	assert := assert.New(t)
	q := NewTxQueue()
	tx1 := trx.Tx{Nonce: uint64(1), Amount: big.NewInt(1)}
	tx2 := trx.Tx{Nonce: uint64(2), Amount: big.NewInt(100)}
	tx3 := trx.Tx{Nonce: uint64(3), Amount: big.NewInt(1000)}
	q.Put(&tx1)
	q.Put(&tx2)
	q.Put(&tx3)
	q.FilterNonce(uint64(3))
	assert.Equal(1, len(q.items))
	assert.Equal(uint64(3), q.index[0])
	assert.Equal(&tx3, q.items[q.index[0]])
}

func TestTxQueue_UpdatePendingNonce(t *testing.T) {
	assert := assert.New(t)
	q := NewTxQueue()
	tx1 := trx.Tx{Nonce: uint64(1), Amount: big.NewInt(1)}
	tx2 := trx.Tx{Nonce: uint64(3), Amount: big.NewInt(1000)}
	tx3 := trx.Tx{Nonce: uint64(4), Amount: big.NewInt(10000)}
	tx4 := trx.Tx{Nonce: uint64(6), Amount: big.NewInt(100000)}
	tx5 := trx.Tx{Nonce: uint64(2), Amount: big.NewInt(100)}
	q.Put(&tx1)
	q.Put(&tx2)
	q.Put(&tx3)
	q.Put(&tx4)
	q.confirmedNonce = uint64(2)
	q.pendingNonce = uint64(2)
	q.pendingBalance = big.NewInt(1100)
	q.Put(&tx5)
	q.UpdatePendingNonce(uint64(2), true)
	assert.Equal(uint64(5), q.pendingNonce)
	assert.Equal(uint64(4), q.confirmedNonce)
}

func TestTxQueue_ConfirmedTxs(t *testing.T) {
	assert := assert.New(t)
	q := NewTxQueue()
	tx1 := trx.Tx{Nonce: uint64(2), Amount: big.NewInt(1)}
	tx2 := trx.Tx{Nonce: uint64(3), Amount: big.NewInt(100)}
	tx3 := trx.Tx{Nonce: uint64(4), Amount: big.NewInt(1000)}
	tx4 := trx.Tx{Nonce: uint64(6), Amount: big.NewInt(10000)}
	tx5 := trx.Tx{Nonce: uint64(7), Amount: big.NewInt(100000)}
	q.Put(&tx1)
	q.Put(&tx2)
	q.Put(&tx3)
	q.Put(&tx4)
	q.Put(&tx5)
	q.confirmedNonce = 4
	txs := q.ConfirmedTxs()
	assert.Equal([]*trx.Tx{&tx1, &tx2}, txs)
}
