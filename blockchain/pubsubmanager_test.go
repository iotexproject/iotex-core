// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type noopSubscriber struct{}

func (noopSubscriber) ReceiveBlock(*block.Block) error { return nil }

// Regression: a concurrent RemoveBlockListener/Stop closes elem.cancel and lets
// the handler goroutine exit, after which nothing drains pendingBlksBuffer. A
// send in SendBlockToSubscribers must then watch elem.cancel, or it blocks
// forever — and because the caller is commitBlock under the chain write lock,
// that wedges the whole node. This pins the cancel-aware send.
func TestPubSub_SendUnblocksOnCancel(t *testing.T) {
	ps := NewPubSub(1).(*pubSub)

	// an element whose buffer is already full and whose handler has gone away
	elem := &pubSubElem{
		listener:          noopSubscriber{},
		pendingBlksBuffer: make(chan *block.Block, 1),
		cancel:            make(chan interface{}),
	}
	elem.pendingBlksBuffer <- &block.Block{} // fill the (cap 1) buffer
	ps.blocklisteners = append(ps.blocklisteners, elem)

	done := make(chan struct{})
	go func() { defer close(done); ps.SendBlockToSubscribers(&block.Block{}) }()

	// nothing drains the buffer, so the send is parked
	select {
	case <-done:
		t.Fatal("SendBlockToSubscribers returned while the buffer was full and no cancel fired")
	case <-time.After(300 * time.Millisecond):
	}

	// RemoveBlockListener/Stop would close cancel; the parked send must then return
	close(elem.cancel)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SendBlockToSubscribers did not return after cancel; commitBlock would wedge")
	}
}
