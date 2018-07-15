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

	"github.com/iotexproject/iotex-core/blockchain/action"
	pb "github.com/iotexproject/iotex-core/proto"
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
	vote1 := action.Vote{&pb.VotePb{Nonce: uint64(2)}}
	action1 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote1.ConvertToVotePb()}}
	err := q.Put(action1)
	require.NoError(err)
	require.Equal(uint64(2), q.index[0])
	require.NotNil(q.items[vote1.Nonce])
	tsf2 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(100)}
	action2 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf2.ConvertToTransferPb()}}
	err = q.Put(action2)
	require.NoError(err)
	require.Equal(uint64(1), heap.Pop(&q.index))
	require.Equal(action2, q.items[uint64(1)])
	require.Equal(uint64(2), heap.Pop(&q.index))
	require.Equal(action1, q.items[uint64(2)])
	// tsf3 is a replacement transfer
	tsf3 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(1000)}
	action3 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}
	err = q.Put(action3)
	require.NotNil(err)
}

func TestActQueue_FilterNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	tsf1 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(1)}
	action1 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf1.ConvertToTransferPb()}}
	vote2 := action.Vote{&pb.VotePb{Nonce: uint64(2)}}
	action2 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote2.ConvertToVotePb()}}
	tsf3 := action.Transfer{Nonce: uint64(3), Amount: big.NewInt(1000)}
	action3 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}
	err := q.Put(action1)
	require.NoError(err)
	err = q.Put(action2)
	require.NoError(err)
	err = q.Put(action3)
	require.NoError(err)
	q.FilterNonce(uint64(3))
	require.Equal(1, len(q.items))
	require.Equal(uint64(3), q.index[0])
	require.Equal(action3, q.items[q.index[0]])
}

func TestActQueue_UpdateNonce(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	tsf1 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(1)}
	action1 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf1.ConvertToTransferPb()}}
	tsf2 := action.Transfer{Nonce: uint64(3), Amount: big.NewInt(1000)}
	action2 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf2.ConvertToTransferPb()}}
	tsf3 := action.Transfer{Nonce: uint64(4), Amount: big.NewInt(10000)}
	action3 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}
	tsf4 := action.Transfer{Nonce: uint64(6), Amount: big.NewInt(100000)}
	action4 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf4.ConvertToTransferPb()}}
	vote5 := action.Vote{&pb.VotePb{Nonce: uint64(2)}}
	action5 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote5.ConvertToVotePb()}}
	err := q.Put(action1)
	require.NoError(err)
	err = q.Put(action2)
	require.NoError(err)
	err = q.Put(action3)
	require.NoError(err)
	err = q.Put(action4)
	require.NoError(err)
	q.pendingNonce = uint64(2)
	q.pendingBalance = big.NewInt(1000)
	err = q.Put(action5)
	require.NoError(err)
	removed := q.UpdateQueue(uint64(2))
	require.Equal(uint64(4), q.pendingNonce)
	require.Equal([]*pb.ActionPb{action3, action4}, removed)
}

func TestActQueue_PendingActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	vote1 := action.Vote{&pb.VotePb{Nonce: uint64(2)}}
	action1 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote1.ConvertToVotePb()}}
	tsf2 := action.Transfer{Nonce: uint64(3), Amount: big.NewInt(100)}
	action2 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf2.ConvertToTransferPb()}}
	tsf3 := action.Transfer{Nonce: uint64(5), Amount: big.NewInt(1000)}
	action3 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}
	tsf4 := action.Transfer{Nonce: uint64(6), Amount: big.NewInt(10000)}
	action4 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf4.ConvertToTransferPb()}}
	tsf5 := action.Transfer{Nonce: uint64(7), Amount: big.NewInt(100000)}
	action5 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf5.ConvertToTransferPb()}}
	err := q.Put(action1)
	require.NoError(err)
	err = q.Put(action2)
	require.NoError(err)
	err = q.Put(action3)
	require.NoError(err)
	err = q.Put(action4)
	require.NoError(err)
	err = q.Put(action5)
	require.NoError(err)
	q.pendingNonce = 4
	actions := q.PendingActs()
	require.Equal([]*pb.ActionPb{action1, action2}, actions)
}

func TestActQueue_AllActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	vote1 := action.Vote{&pb.VotePb{Nonce: uint64(1)}}
	action1 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote1.ConvertToVotePb()}}
	tsf3 := action.Transfer{Nonce: uint64(3), Amount: big.NewInt(1000)}
	action3 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}
	err := q.Put(action1)
	require.NoError(err)
	err = q.Put(action3)
	require.NoError(err)
	actions := q.AllActs()
	require.Equal([]*pb.ActionPb{action1, action3}, actions)
}

func TestActQueue_removeActs(t *testing.T) {
	require := require.New(t)
	q := NewActQueue().(*actQueue)
	tsf1 := action.Transfer{Nonce: uint64(1), Amount: big.NewInt(100)}
	action1 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf1.ConvertToTransferPb()}}
	vote2 := action.Vote{&pb.VotePb{Nonce: uint64(2)}}
	action2 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote2.ConvertToVotePb()}}
	tsf3 := action.Transfer{Nonce: uint64(3), Amount: big.NewInt(1000)}
	action3 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}
	err := q.Put(action1)
	require.NoError(err)
	err = q.Put(action2)
	require.NoError(err)
	err = q.Put(action3)
	require.NoError(err)
	removed := q.removeActs(0)
	require.Equal(0, len(q.index))
	require.Equal(0, len(q.items))
	require.Equal([]*pb.ActionPb{action1, action2, action3}, removed)

	tsf4 := action.Transfer{Nonce: uint64(4), Amount: big.NewInt(10000)}
	action4 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf4.ConvertToTransferPb()}}
	tsf5 := action.Transfer{Nonce: uint64(5), Amount: big.NewInt(100000)}
	action5 := &pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf5.ConvertToTransferPb()}}
	vote6 := action.Vote{&pb.VotePb{Nonce: uint64(6)}}
	action6 := &pb.ActionPb{Action: &pb.ActionPb_Vote{vote6.ConvertToVotePb()}}
	err = q.Put(action4)
	require.NoError(err)
	err = q.Put(action5)
	require.NoError(err)
	err = q.Put(action6)
	require.NoError(err)
	removed = q.removeActs(1)
	require.Equal(1, len(q.index))
	require.Equal(1, len(q.items))
	require.Equal([]*pb.ActionPb{action5, action6}, removed)
}
