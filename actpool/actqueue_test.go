// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.
package actpool

import (
	"container/heap"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

func TestNoncePriorityQueue(t *testing.T) {
	require := require.New(t)
	pq := noncePriorityQueue{}
	// Push four dummy nonce to the queue
	heap.Push(&pq, &nonceWithTTL{nonce: uint64(1)})
	heap.Push(&pq, &nonceWithTTL{nonce: uint64(3)})
	heap.Push(&pq, &nonceWithTTL{nonce: uint64(2)})
	// Test Pop implementation
	i := uint64(1)
	for pq.Len() > 0 {
		nonce := heap.Pop(&pq).(*nonceWithTTL).nonce
		require.Equal(i, nonce)
		i++
	}
	// Repush the four dummy nonce back to the queue
	heap.Push(&pq, &nonceWithTTL{nonce: uint64(3)})
	heap.Push(&pq, &nonceWithTTL{nonce: uint64(2)})
	heap.Push(&pq, &nonceWithTTL{nonce: uint64(1)})
	// Test built-in Remove implementation
	// Remove a random nonce from noncePriorityQueue
	rand.Seed(time.Now().UnixNano())
	heap.Remove(&pq, rand.Intn(pq.Len()))
	t.Log("After randomly removing a dummy nonce, the remaining dummy nonces in the order of popped are as follows:")
	for pq.Len() > 0 {
		nonce := heap.Pop(&pq).(*nonceWithTTL).nonce
		t.Log(nonce)
		t.Log()
	}
}

func TestActQueuePut(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.Equal(uint64(2), q.index[0].nonce)
	require.NotNil(q.items[tsf1.Nonce()])
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	require.NoError(err)
	require.NoError(q.Put(tsf2))
	require.Equal(uint64(1), heap.Pop(&q.index).(*nonceWithTTL).nonce)
	require.Equal(tsf2, q.items[uint64(1)])
	require.Equal(uint64(2), heap.Pop(&q.index).(*nonceWithTTL).nonce)
	require.Equal(tsf1, q.items[uint64(2)])
	// tsf3 is a act which fails to cut in line
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.Error(q.Put(tsf3))
	// tsf4 is a act which succeeds in cutting in line
	tsf4, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1000), nil, uint64(0), big.NewInt(2))
	require.NoError(err)
	require.NoError(q.Put(tsf4))
}

func TestActQueueFilterNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf2))
	require.NoError(q.Put(tsf3))
	q.FilterNonce(uint64(3))
	require.Equal(1, len(q.items))
	require.Equal(uint64(3), q.index[0].nonce)
	require.Equal(tsf3, q.items[q.index[0].nonce])
}

func TestActQueueUpdateNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 4, big.NewInt(10000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr2, _priKey1, 6, big.NewInt(100000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf2))
	require.NoError(q.Put(tsf3))
	require.NoError(q.Put(tsf4))
	q.pendingBalance = big.NewInt(1000)
	require.NoError(q.Put(tsf5))
	removed := q.UpdateQueue(uint64(2))
	require.Equal(uint64(2), q.pendingNonce)
	require.Equal([]action.SealedEnvelope{tsf5, tsf2, tsf3, tsf4}, removed)
}

func TestActQueuePendingActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	cfg := config.Default
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).Do(func(accountState *state.Account, _ protocol.StateOption) {
		require.NoError(accountState.SetNonce(accountState.PendingNonce()))
	}).Return(uint64(0), nil).Times(1)
	ap, err := NewActPool(sf, cfg.ActPool, EnableExperimentalActions())
	require.NoError(err)
	q := NewActQueue(ap.(*actPool), identityset.Address(0).String()).(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 5, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr2, _priKey1, 6, big.NewInt(10000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(_addr2, _priKey1, 7, big.NewInt(100000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf2))
	require.NoError(q.Put(tsf3))
	require.NoError(q.Put(tsf4))
	require.NoError(q.Put(tsf5))
	q.pendingNonce = 4
	actions := q.PendingActs()
	require.Equal([]action.SealedEnvelope{tsf1, tsf2}, actions)
}

func TestActQueueAllActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf3))
	actions := q.AllActs()
	require.Equal([]action.SealedEnvelope{tsf1, tsf3}, actions)
}

func TestActQueueRemoveActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf2))
	require.NoError(q.Put(tsf3))
	removed := q.removeActs(0)
	require.Equal(0, len(q.index))
	require.Equal(0, len(q.items))
	require.Equal([]action.SealedEnvelope{tsf1, tsf2, tsf3}, removed)

	tsf4, err := action.SignedTransfer(_addr2, _priKey1, 4, big.NewInt(10000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(_addr2, _priKey1, 5, big.NewInt(100000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf6, err := action.SignedTransfer(_addr2, _priKey1, 6, big.NewInt(100000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf4))
	require.NoError(q.Put(tsf5))
	require.NoError(q.Put(tsf6))
	removed = q.removeActs(1)
	require.Equal(1, len(q.index))
	require.Equal(1, len(q.items))
	require.Equal([]action.SealedEnvelope{tsf5, tsf6}, removed)
}

func TestActQueueTimeOutAction(t *testing.T) {
	c := clock.NewMock()
	q := NewActQueue(nil, "", WithClock(c), WithTimeOut(3*time.Minute))
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	require.NoError(t, q.Put(tsf1))
	c.Add(2 * time.Minute)

	require.NoError(t, q.Put(tsf2))
	q.(*actQueue).cleanTimeout()
	assert.Equal(t, 2, q.Len())
	c.Add(2 * time.Minute)
	q.(*actQueue).cleanTimeout()
	assert.Equal(t, 1, q.Len())
}

func TestActQueueCleanTimeout(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	q.ttl = 1
	invalidTime := time.Now()
	validTime := time.Now().Add(10 * time.Minute)
	tsf1, _ := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf2, _ := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf3, _ := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf5, _ := action.SignedTransfer(_addr2, _priKey1, 5, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf6, _ := action.SignedTransfer(_addr2, _priKey1, 6, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf7, _ := action.SignedTransfer(_addr2, _priKey1, 7, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	q.items[1] = tsf1
	q.items[2] = tsf2
	q.items[3] = tsf3
	q.items[5] = tsf5
	q.items[6] = tsf6
	q.items[7] = tsf7

	q.index = []*nonceWithTTL{
		{idx: 0, nonce: 1, deadline: validTime},
		{idx: 1, nonce: 5, deadline: validTime},
		{idx: 2, nonce: 2, deadline: validTime},
		{idx: 3, nonce: 6, deadline: validTime},
		{idx: 4, nonce: 7, deadline: invalidTime},
		{idx: 5, nonce: 3, deadline: validTime},
	}
	q.cleanTimeout()
	require.Equal(5, len(q.index))
	expectedHeap := []uint64{1, 3, 2, 6, 5}
	for i := range expectedHeap {
		require.Equal(expectedHeap[i], q.index[i].nonce)
	}

	q.index = []*nonceWithTTL{
		{idx: 0, nonce: 1, deadline: validTime},
		{idx: 1, nonce: 5, deadline: validTime},
		{idx: 2, nonce: 2, deadline: validTime},
		{idx: 3, nonce: 6, deadline: validTime},
		{idx: 4, nonce: 7, deadline: invalidTime},
		{idx: 5, nonce: 3, deadline: invalidTime},
	}
	ret := q.cleanTimeout()
	require.Equal(2, len(ret))
}

// BenchmarkHeapInitAndRemove compare the heap re-establish performance between
// using the heap.Init and the heap.Remove after remove some elements.
// The bench result show that the performance of heap.Init is better than heap.Remove
// in the most cases.
// More detail to see the discusses in https://github.com/iotexproject/iotex-core/pull/3013
func BenchmarkHeapInitAndRemove(b *testing.B) {
	const batch = 20
	testIndex := noncePriorityQueue{}
	index := noncePriorityQueue{}
	invalidTime := time.Now()
	validTime := time.Now().Add(10 * time.Minute)
	for k := uint64(1); k <= batch; k++ {
		for j := uint64(0); j < batch; j++ {
			if j < k {
				heap.Push(&testIndex, &nonceWithTTL{nonce: j, deadline: invalidTime})
			} else {
				heap.Push(&testIndex, &nonceWithTTL{nonce: j, deadline: validTime})
			}
		}
		b.ResetTimer()
		b.Run(fmt.Sprintf("heap.Remove-(%d/%d)", k, batch), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// init
				index = index[:0]
				for _, nonce := range testIndex {
					nonce2 := *nonce
					index = append(index, &nonce2)
				}
				// algo
				removedNonceList := make([]*nonceWithTTL, 0, batch)
				for _, nonce := range index {
					if invalidTime.Equal(nonce.deadline) {
						removedNonceList = append(removedNonceList, nonce)
					}
				}
				for _, removedNonce := range removedNonceList {
					heap.Remove(&index, removedNonce.idx)
				}
			}
		})
		b.ResetTimer()
		b.Run(fmt.Sprintf("heap.Init-(%d/%d)", k, batch), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// init
				index = index[:0]
				for _, nonce := range testIndex {
					nonce2 := *nonce
					index = append(index, &nonce2)
				}
				// algo
				size := index.Len()
				for j := 0; j < size; {
					if invalidTime.Equal(index[j].deadline) {
						index[j] = index[size-1]
						size--
						continue
					}
					j++
				}
				index = index[:size]
				heap.Init(&index)
			}
		})
	}
}
