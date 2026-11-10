// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/stretchr/testify/require"
)

// A panicking handler on a recoverable queue must cost the message, not the
// process: the worker recovers, counts it, and keeps serving.
func TestHandleMsgOnQueueRecovers(t *testing.T) {
	r := require.New(t)

	var handled int
	m := newMsgQueueMgr(msgQueueConfig{}, func(msg *message) {
		handled++
		panic("handler blew up")
	})
	msg := &message{msgType: iotexrpc.MessageType_ACTION, peer: "peer1"}

	for _, q := range []string{actionQ, blockSyncQ, consensusQ, miscQ} {
		r.NotPanics(func() { m.handleMsgOnQueue(q, msg) }, q)
	}
	r.Equal(4, handled)
}

// blockSyncQ panics must be recovered AND the consume worker must survive to
// serve the next request.
func TestConsumeBlockSyncWorkerSurvivesPanic(t *testing.T) {
	r := require.New(t)

	var handled int32
	served := make(chan struct{}, 2)
	m := newMsgQueueMgr(msgQueueConfig{blockSyncSize: 2}, func(msg *message) {
		n := atomic.AddInt32(&handled, 1)
		defer func() { served <- struct{}{} }()
		if n == 1 {
			panic("handler blew up")
		}
	})

	m.wg.Add(1)
	go m.consume(blockSyncQ)
	defer func() { _ = m.Stop() }()

	bad := &message{msgType: iotexrpc.MessageType_BLOCK_REQUEST, peer: "peer1"}
	good := &message{msgType: iotexrpc.MessageType_BLOCK_REQUEST, peer: "peer1"}
	m.queues[blockSyncQ] <- bad
	m.queues[blockSyncQ] <- good

	for i := 0; i < 2; i++ {
		select {
		case <-served:
		case <-time.After(5 * time.Second):
			t.Fatal("blockSync worker did not survive the panic")
		}
	}
	r.Equal(int32(2), atomic.LoadInt32(&handled))
}

// The commit-bearing queue keeps today's fail-stop behaviour: blocksync commits
// inline on the blockQ worker while holding bs.mu, so a panic there must not be
// unwound past half-applied state. blockSyncQ must NOT have been made
// recoverable by accident at blockQ's expense.
func TestHandleMsgOnQueueDoesNotRecoverCommitQueues(t *testing.T) {
	r := require.New(t)

	m := newMsgQueueMgr(msgQueueConfig{}, func(msg *message) {
		panic("handler blew up")
	})
	msg := &message{msgType: iotexrpc.MessageType_BLOCK, peer: "peer1"}

	for _, q := range []string{blockQ} {
		r.Panics(func() { m.handleMsgOnQueue(q, msg) }, q)
	}
}
