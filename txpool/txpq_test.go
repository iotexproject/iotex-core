// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txpool

import (
	"container/heap"
	"math/rand"
	"testing"
	"time"

	trx "github.com/iotexproject/iotex-core/blockchain/trx"
)

func TestTxPq(t *testing.T) {
	// Create four dummy TxDescs
	// TxDesc1
	desc1 := TxDesc{
		Tx:          &trx.Tx{},
		AddedTime:   time.Now(),
		BlockHeight: uint64(1),
		Fee:         int64(0),
		FeePerKB:    int64(0),
		Priority:    float64(100),
	}

	// TxDesc2
	desc2 := TxDesc{
		Tx:          &trx.Tx{},
		AddedTime:   time.Now(),
		BlockHeight: uint64(1),
		Fee:         int64(0),
		FeePerKB:    int64(0),
		Priority:    float64(50),
	}

	// TxDesc3
	desc3 := TxDesc{
		Tx:          &trx.Tx{},
		AddedTime:   time.Now(),
		BlockHeight: uint64(1),
		Fee:         int64(0),
		FeePerKB:    int64(0),
		Priority:    float64(1000),
	}

	// TxDesc4
	desc4 := TxDesc{
		Tx:          &trx.Tx{},
		AddedTime:   time.Now(),
		BlockHeight: uint64(1),
		Fee:         int64(0),
		FeePerKB:    int64(0),
		Priority:    float64(5),
	}

	// Initialize a txDescPriorityQueue
	pq := txDescPriorityQueue{}

	// Test Push implementation
	heap.Push(&pq, &desc1)
	heap.Push(&pq, &desc2)
	heap.Push(&pq, &desc3)
	heap.Push(&pq, &desc4)

	t.Log("The priority of each TxDesc in the order of popped is as follows:")

	// Test Pop implementation
	for pq.Len() > 0 {
		txDesc := heap.Pop(&pq).(*TxDesc)
		t.Log(txDesc.Priority)
		t.Log()
	}

	// Repush the four dummy TxDescs back to the txDescPriorityQueue
	heap.Push(&pq, &desc1)
	heap.Push(&pq, &desc2)
	heap.Push(&pq, &desc3)
	heap.Push(&pq, &desc4)

	// Test built-in Remove implementation
	// Remove a random *txDesc from txDescPriorityQueue
	rand.Seed(time.Now().UnixNano())
	heap.Remove(&pq, rand.Intn(pq.Len()))

	t.Log("After randomly removing a TxDesc, the priority of each remaining TxDesc in the order of popped is as follows:")

	for pq.Len() > 0 {
		txDesc := heap.Pop(&pq).(*TxDesc)
		t.Log(txDesc.Priority)
		t.Log()
	}
}
