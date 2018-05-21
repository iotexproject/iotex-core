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

	"github.com/iotexproject/iotex-core-internal/blockchain"
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

func TestTxList_Put(t *testing.T) {
	assert := assert.New(t)
	l := NewTxList()
	tx1 := blockchain.Tx{Nonce: uint64(2), Amount: big.NewInt(10)}
	l.Put(&tx1)
	assert.Equal(uint64(2), l.index[0])
	assert.NotNil(l.items[tx1.Nonce])
	assert.Equal(big.NewInt(10), l.costcap)
	tx2 := blockchain.Tx{Nonce: uint64(1), Amount: big.NewInt(100)}
	l.Put(&tx2)
	assert.Equal(uint64(1), heap.Pop(&l.index))
	assert.Equal(&tx2, l.items[uint64(1)])
	assert.Equal(uint64(2), heap.Pop(&l.index))
	assert.Equal(&tx1, l.items[uint64(2)])
	assert.Equal(big.NewInt(100), l.costcap)
	// tx3 is a replacement transaction
	tx3 := blockchain.Tx{Nonce: uint64(1), Amount: big.NewInt(1000)}
	err := l.Put(&tx3)
	assert.Equal(ErrReplaceTx, err)
}

func TestTxList_FilterNonce(t *testing.T) {
	assert := assert.New(t)
	l := NewTxList()
	tx1 := blockchain.Tx{Nonce: uint64(1), Amount: big.NewInt(1)}
	tx2 := blockchain.Tx{Nonce: uint64(2), Amount: big.NewInt(100)}
	tx3 := blockchain.Tx{Nonce: uint64(3), Amount: big.NewInt(1000)}
	l.Put(&tx1)
	l.Put(&tx2)
	l.Put(&tx3)
	l.FilterNonce(uint64(3))
	assert.Equal(1, len(l.items))
	assert.Equal(uint64(3), l.index[0])
	assert.Equal(&tx3, l.items[l.index[0]])
}

func TestTxList_FilterCost(t *testing.T) {
	// Filter out all the transactions above the account's funds which is 5 in this test case
	assert := assert.New(t)
	l := NewTxList()
	tx1 := blockchain.Tx{Nonce: uint64(1), Amount: big.NewInt(1)}
	tx2 := blockchain.Tx{Nonce: uint64(2), Amount: big.NewInt(10)}
	tx3 := blockchain.Tx{Nonce: uint64(3), Amount: big.NewInt(3)}
	l.Put(&tx1)
	l.Put(&tx2)
	l.Put(&tx3)
	removed := l.FilterCost(big.NewInt(5))
	assert.Equal(2, len(l.items))
	assert.Equal(&tx2, removed[0])
	assert.Equal(uint64(1), heap.Pop(&l.index))
	assert.Equal(&tx1, l.items[uint64(1)])
	assert.Equal(uint64(3), heap.Pop(&l.index))
	assert.Equal(&tx3, l.items[uint64(3)])
}

func TestTxList_UpdatedPendingNonce(t *testing.T) {
	assert := assert.New(t)
	l := NewTxList()
	tx1 := blockchain.Tx{Nonce: uint64(1), Amount: big.NewInt(1)}
	tx2 := blockchain.Tx{Nonce: uint64(2), Amount: big.NewInt(100)}
	tx3 := blockchain.Tx{Nonce: uint64(3), Amount: big.NewInt(1000)}
	tx4 := blockchain.Tx{Nonce: uint64(4), Amount: big.NewInt(10000)}
	tx5 := blockchain.Tx{Nonce: uint64(6), Amount: big.NewInt(100000)}
	l.Put(&tx1)
	l.Put(&tx2)
	l.Put(&tx3)
	l.Put(&tx4)
	l.Put(&tx5)
	newPendingNonce := l.UpdatedPendingNonce(uint64(2))
	assert.Equal(uint64(5), newPendingNonce)
}

func TestTxList_AcceptedTxs(t *testing.T) {
	assert := assert.New(t)
	l := NewTxList()
	tx1 := blockchain.Tx{Nonce: uint64(2), Amount: big.NewInt(1)}
	tx2 := blockchain.Tx{Nonce: uint64(3), Amount: big.NewInt(100)}
	tx3 := blockchain.Tx{Nonce: uint64(4), Amount: big.NewInt(1000)}
	tx4 := blockchain.Tx{Nonce: uint64(6), Amount: big.NewInt(10000)}
	tx5 := blockchain.Tx{Nonce: uint64(7), Amount: big.NewInt(100000)}
	l.Put(&tx1)
	l.Put(&tx2)
	l.Put(&tx3)
	l.Put(&tx4)
	l.Put(&tx5)
	txs := l.AcceptedTxs()
	assert.Equal([]*blockchain.Tx{&tx1, &tx2, &tx3}, txs)
}
