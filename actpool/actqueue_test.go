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

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"

	"github.com/iotexproject/iotex-core/config"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestNoncePriorityQueue(t *testing.T) {
	require := require.New(t)
	pq := noncePriorityQueue{}
	// Push four dummy nonce to the queue
	heap.Push(&pq, nonceWithTTL{nonce: uint64(1)})
	heap.Push(&pq, nonceWithTTL{nonce: uint64(3)})
	heap.Push(&pq, nonceWithTTL{nonce: uint64(2)})
	// Test Pop implementation
	i := uint64(1)
	for pq.Len() > 0 {
		nonce := heap.Pop(&pq).(nonceWithTTL).nonce
		require.Equal(i, nonce)
		i++
	}
	// Repush the four dummy nonce back to the queue
	heap.Push(&pq, nonceWithTTL{nonce: uint64(3)})
	heap.Push(&pq, nonceWithTTL{nonce: uint64(2)})
	heap.Push(&pq, nonceWithTTL{nonce: uint64(1)})
	// Test built-in Remove implementation
	// Remove a random nonce from noncePriorityQueue
	rand.Seed(time.Now().UnixNano())
	heap.Remove(&pq, rand.Intn(pq.Len()))
	t.Log("After randomly removing a dummy nonce, the remaining dummy nonces in the order of popped are as follows:")
	for pq.Len() > 0 {
		nonce := heap.Pop(&pq).(nonceWithTTL).nonce
		t.Log(nonce)
		t.Log()
	}
}

func TestActQueuePut(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	vote1, err := testutil.SignedVote(addr2, priKey1, 2, 0, big.NewInt(0))
	require.NoError(err)
	err = q.Put(vote1)
	require.NoError(err)
	require.Equal(uint64(2), q.index[0].nonce)
	require.NotNil(q.items[vote1.Nonce()])
	tsf2, err := testutil.SignedTransfer(addr2, priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf2)
	require.NoError(err)
	require.Equal(uint64(1), heap.Pop(&q.index).(nonceWithTTL).nonce)
	require.Equal(tsf2, q.items[uint64(1)])
	require.Equal(uint64(2), heap.Pop(&q.index).(nonceWithTTL).nonce)
	require.Equal(vote1, q.items[uint64(2)])
	// tsf3 is a replacement transfer
	tsf3, err := testutil.SignedTransfer(addr2, priKey1, 1, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf3)
	require.NotNil(err)
}

func TestActQueueFilterNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	tsf1, err := testutil.SignedTransfer(addr2, priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	vote2, err := testutil.SignedVote(addr2, priKey1, 2, 0, big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr2, priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(tsf1)
	require.NoError(err)
	err = q.Put(vote2)
	require.NoError(err)
	err = q.Put(tsf3)
	require.NoError(err)
	q.FilterNonce(uint64(3))
	require.Equal(1, len(q.items))
	require.Equal(uint64(3), q.index[0].nonce)
	require.Equal(tsf3, q.items[q.index[0].nonce])
}

func TestActQueueUpdateNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	tsf1, err := testutil.SignedTransfer(addr2, priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr2, priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr2, priKey1, 4, big.NewInt(10000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf4, err := testutil.SignedTransfer(addr2, priKey1, 6, big.NewInt(100000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	vote5, err := testutil.SignedVote(addr2, priKey1, 2, 0, big.NewInt(0))
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
	require.Equal([]action.SealedEnvelope{tsf3, tsf4}, removed)
}

func TestActQueuePendingActs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	require := require.New(t)
	cfg := config.Default
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().Nonce(gomock.Any()).Return(uint64(1), nil).Times(1)
	ap, err := NewActPool(bc, cfg.ActPool)
	require.NoError(err)
	q := NewActQueue(ap.(*actPool), "").(*actQueue)
	vote1, err := testutil.SignedVote(addr2, priKey1, 2, 0, big.NewInt(0))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(addr2, priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr2, priKey1, 5, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf4, err := testutil.SignedTransfer(addr2, priKey1, 6, big.NewInt(10000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf5, err := testutil.SignedTransfer(addr2, priKey1, 7, big.NewInt(100000), nil, uint64(0), big.NewInt(0))
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
	q.pendingNonce = 4
	actions := q.PendingActs()
	require.Equal([]action.SealedEnvelope{vote1, tsf2}, actions)
}

func TestActQueueAllActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	vote1, err := testutil.SignedVote(addr2, priKey1, 1, 0, big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr2, priKey1, 3, big.NewInt(1000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	err = q.Put(vote1)
	require.NoError(err)
	err = q.Put(tsf3)
	require.NoError(err)
	actions := q.AllActs()
	require.Equal([]action.SealedEnvelope{vote1, tsf3}, actions)
}

func TestActQueueRemoveActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue(nil, "").(*actQueue)
	tsf1, err := testutil.SignedTransfer(addr2, priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	vote2, err := testutil.SignedVote(addr2, priKey1, 2, 0, big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(addr2, priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
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
	require.Equal([]action.SealedEnvelope{tsf1, vote2, tsf3}, removed)

	tsf4, err := testutil.SignedTransfer(addr2, priKey1, 4, big.NewInt(10000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	tsf5, err := testutil.SignedTransfer(addr2, priKey1, 5, big.NewInt(100000), nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	vote6, err := testutil.SignedVote(addr2, priKey1, 6, 0, big.NewInt(0))
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
	require.Equal([]action.SealedEnvelope{tsf5, vote6}, removed)
}

func TestActQueueTimeOutAction(t *testing.T) {
	c := clock.NewMock()
	q := NewActQueue(nil, "", WithClock(c), WithTimeOut(3*time.Minute))
	tsf1, err := testutil.SignedTransfer(addr2, priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	tsf2, err := testutil.SignedTransfer(addr2, priKey1, 3, big.NewInt(100), nil, uint64(0), big.NewInt(0))
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
