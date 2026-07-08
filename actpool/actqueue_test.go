// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.
package actpool

import (
	"container/heap"
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

const (
	maxBalance = 1e7
)

func TestNoncePriorityQueue(t *testing.T) {
	require := require.New(t)
	pq := ascNoncePriorityQueue{}
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
	ctrl := gomock.NewController(t)
	ap, err := NewActPool(genesis.TestDefault(), mock_chainmanager.NewMockStateReader(ctrl), DefaultConfig)
	require.NoError(err)
	q := NewActQueue(ap.(*actPool), "", 1, big.NewInt(maxBalance)).(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.Equal(uint64(2), q.ascQueue[0].nonce)
	require.NotNil(q.items[tsf1.Nonce()])
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
	require.NoError(err)
	require.NoError(q.Put(tsf2))
	require.Equal(uint64(1), heap.Pop(&q.ascQueue).(*nonceWithTTL).nonce)
	require.Equal(tsf2, q.items[uint64(1)])
	require.Equal(uint64(2), heap.Pop(&q.ascQueue).(*nonceWithTTL).nonce)
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
	q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf2))
	require.NoError(q.Put(tsf3))
	q.UpdateAccountState(3, big.NewInt(maxBalance))
	require.Equal(1, len(q.items))
	require.Equal(uint64(3), q.ascQueue[0].nonce)
	require.Equal(tsf3, q.items[q.ascQueue[0].nonce])
}

func TestActQueueUpdateNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "", 1, big.NewInt(1010)).(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 4, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf4, err := action.SignedTransfer(_addr2, _priKey1, 6, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf5, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf2))
	require.NoError(q.Put(tsf3))
	require.NoError(q.Put(tsf4))
	require.NoError(q.Put(tsf5))
	require.Equal(uint64(3), q.pendingNonce)
}

func TestActQueuePendingActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().State(gomock.Any(), gomock.Any()).Do(func(accountState *state.Account, _ protocol.StateOption) {
		require.NoError(accountState.SetPendingNonce(accountState.PendingNonce() + 1))
		accountState.Balance = big.NewInt(maxBalance)
	}).Return(uint64(0), nil).Times(1)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), genesis.TestDefault()), protocol.BlockCtx{
			BlockHeight: 1,
		}))
	ap, err := NewActPool(genesis.TestDefault(), sf, DefaultConfig)
	require.NoError(err)
	q := NewActQueue(ap.(*actPool), identityset.Address(0).String(), 1, big.NewInt(maxBalance)).(*actQueue)
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
	actions := q.PendingActs(ctx)
	require.Equal([]*action.SealedEnvelope{tsf1, tsf2}, actions)
}

func TestActQueueAllActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf3))
	actions := q.AllActs()
	require.Equal([]*action.SealedEnvelope{tsf1, tsf3}, actions)
}

func TestActQueueTimeOutAction(t *testing.T) {
	c := clock.NewMock()
	q := NewActQueue(nil, "", 1, big.NewInt(maxBalance), WithClock(c), WithTimeOut(3*time.Minute))
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	require.NoError(t, q.Put(tsf1))
	c.Add(2 * time.Minute)

	require.NoError(t, q.Put(tsf2))
	q.(*actQueue).cleanTimeout()
	require.Equal(t, 2, q.Len())
	c.Add(2 * time.Minute)
	q.(*actQueue).cleanTimeout()
	require.Equal(t, 2, q.Len())
	c.Add(2 * time.Minute)
	q.(*actQueue).cleanTimeout()
	require.Equal(t, 1, q.Len())
}

func TestActQueueCleanTimeout(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "", 1, big.NewInt(1000)).(*actQueue)
	mockClock := clock.NewMock()
	q.clock = mockClock
	q.ttl = 2 * time.Minute
	tsf1, _ := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf2, _ := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf3, _ := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf5, _ := action.SignedTransfer(_addr2, _priKey1, 5, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf6, _ := action.SignedTransfer(_addr2, _priKey1, 6, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	tsf7, _ := action.SignedTransfer(_addr2, _priKey1, 7, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(q.Put(tsf7))
	mockClock.Add(10 * time.Minute)
	require.NoError(q.Put(tsf1))
	require.NoError(q.Put(tsf5))
	mockClock.Add(1 * time.Minute)
	require.NoError(q.Put(tsf2))
	require.NoError(q.Put(tsf6))
	require.NoError(q.Put(tsf3))

	q.cleanTimeout()
	require.Equal(5, len(q.ascQueue))
	expectedHeap := []uint64{1, 2, 3, 5, 6}
	for i := range expectedHeap {
		require.Equal(expectedHeap[i], q.ascQueue[i].nonce)
	}
	mockClock.Add(2 * time.Minute)
	ret := q.cleanTimeout()
	require.Equal(1, len(ret))
}

func TestActQueueReset(t *testing.T) {
	r := require.New(t)
	q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	r.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	r.NoError(err)
	r.NoError(q.Put(tsf1))
	r.NoError(q.Put(tsf2))
	r.Equal(2, q.Len())
	r.False(q.Empty())

	q.Reset()
	r.Equal(0, q.Len())
	r.True(q.Empty())
	r.Equal(uint64(0), q.pendingNonce)
	r.Equal(uint64(0), q.accountNonce)
	r.Equal(0, q.ascQueue.Len())
	r.Equal(0, q.descQueue.Len())
	r.Equal(big.NewInt(0), q.accountBalance)
	r.Empty(q.AllActs())
}

func TestActQueueNextAction(t *testing.T) {
	r := require.New(t)
	t.Run("empty queue", func(t *testing.T) {
		q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
		pending, gasFeeCap := q.NextAction()
		r.False(pending)
		r.Nil(gasFeeCap)
	})
	t.Run("action continuous with pending nonce is committable", func(t *testing.T) {
		q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
		// gas fee cap is the last arg
		tsf, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(7))
		r.NoError(err)
		r.NoError(q.Put(tsf))
		// pending nonce advanced past account nonce -> committable
		pending, gasFeeCap := q.NextAction()
		r.True(pending)
		r.Equal(big.NewInt(7), gasFeeCap)
	})
	t.Run("gapped action is not committable but still reports its fee cap", func(t *testing.T) {
		q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
		// nonce 3 leaves a gap above pending nonce 1
		tsf, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(9))
		r.NoError(err)
		r.NoError(q.Put(tsf))
		pending, gasFeeCap := q.NextAction()
		r.False(pending)
		r.Equal(big.NewInt(9), gasFeeCap)
	})
}

func TestActQueuePopActionWithLargestNonce(t *testing.T) {
	r := require.New(t)
	t.Run("empty queue returns nil", func(t *testing.T) {
		q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
		r.Nil(q.PopActionWithLargestNonce())
	})
	t.Run("pops the largest nonce and rewinds pending nonce", func(t *testing.T) {
		q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
		tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
		r.NoError(err)
		tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(1), nil, uint64(0), big.NewInt(0))
		r.NoError(err)
		tsf3, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(1), nil, uint64(0), big.NewInt(0))
		r.NoError(err)
		r.NoError(q.Put(tsf1))
		r.NoError(q.Put(tsf2))
		r.NoError(q.Put(tsf3))
		// three continuous actions => pending nonce is 4
		r.Equal(uint64(4), q.pendingNonce)

		r.Equal(tsf3, q.PopActionWithLargestNonce())
		r.Equal(2, q.Len())
		// pending nonce rewound to reflect that nonce 3 is gone
		r.Equal(uint64(3), q.pendingNonce)
		// remaining actions are the two lowest nonces, still ascending
		r.Equal([]*action.SealedEnvelope{tsf1, tsf2}, q.AllActs())

		r.Equal(tsf2, q.PopActionWithLargestNonce())
		r.Equal(tsf1, q.PopActionWithLargestNonce())
		r.Nil(q.PopActionWithLargestNonce())
		r.True(q.Empty())
	})
}

// TestActQueueCrossIndexIntegrity proves that the twin asc/desc heaps keep their
// cross indexes (ascIdx/descIdx) consistent across mixed mutations: UpdateAccountState
// removes the two lowest nonces (popping the asc heap and removing them from the desc
// heap by descIdx), and PopActionWithLargestNonce then removes the highest nonces
// (popping the desc heap and removing them from the asc heap by ascIdx). If either
// stored index drifted, the wrong element would be removed and the ordering below
// would break.
func TestActQueueCrossIndexIntegrity(t *testing.T) {
	r := require.New(t)
	q := NewActQueue(nil, "", 1, big.NewInt(maxBalance)).(*actQueue)
	tsfs := make([]*action.SealedEnvelope, 0, 5)
	for n := uint64(1); n <= 5; n++ {
		tsf, err := action.SignedTransfer(_addr2, _priKey1, n, big.NewInt(1), nil, uint64(0), big.NewInt(0))
		r.NoError(err)
		r.NoError(q.Put(tsf))
		tsfs = append(tsfs, tsf) // tsfs[i] has nonce i+1
	}
	r.Equal(uint64(6), q.pendingNonce)

	// confirm nonces < 3 (removes nonce 1 and 2 from both heaps)
	removed := q.UpdateAccountState(3, big.NewInt(maxBalance))
	r.ElementsMatch([]*action.SealedEnvelope{tsfs[0], tsfs[1]}, removed)
	r.Equal(3, q.Len())

	// pop remaining in descending nonce order; cross-indexes must stay valid
	r.Equal(tsfs[4], q.PopActionWithLargestNonce()) // nonce 5
	r.Equal(tsfs[3], q.PopActionWithLargestNonce()) // nonce 4
	r.Equal(tsfs[2], q.PopActionWithLargestNonce()) // nonce 3
	r.Nil(q.PopActionWithLargestNonce())
	r.True(q.Empty())
}

// BenchmarkHeapInitAndRemove compare the heap re-establish performance between
// using the heap.Init and the heap.Remove after remove some elements.
// The bench result show that the performance of heap.Init is better than heap.Remove
// in the most cases.
// More detail to see the discusses in https://github.com/iotexproject/iotex-core/v2/pull/3013
func BenchmarkHeapInitAndRemove(b *testing.B) {
	const batch = 20
	testIndex := ascNoncePriorityQueue{}
	index := ascNoncePriorityQueue{}
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
					heap.Remove(&index, removedNonce.ascIdx)
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
