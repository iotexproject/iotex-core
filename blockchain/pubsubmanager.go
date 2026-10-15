// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// _slowSubscriberWarnInterval is how often to report that a block subscriber
// has stopped draining. commitBlock is blocked for the whole duration, holding
// the chain write lock, so this must be visible well before an operator
// notices the node has stopped following the chain.
const _slowSubscriberWarnInterval = 5 * time.Second

type (
	// PubSubManager is an interface which handles multi-thread publisher and subscribers
	PubSubManager interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		AddBlockListener(BlockCreationSubscriber) error
		RemoveBlockListener(BlockCreationSubscriber) error
		SendBlockToSubscribers(*block.Block)
	}

	pubSubElem struct {
		listener          BlockCreationSubscriber
		pendingBlksBuffer chan *block.Block // buffered channel for storing the pending blocks
		cancel            chan interface{}  // cancel channel to end the handler thread
	}

	pubSub struct {
		lock                 sync.RWMutex
		blocklisteners       []*pubSubElem
		pendingBlkBufferSize uint64
	}
)

// NewPubSub creates new pubSub struct with buffersize for pendingBlock buffer channel
func NewPubSub(bufferSize uint64) PubSubManager {
	return &pubSub{
		blocklisteners:       make([]*pubSubElem, 0),
		pendingBlkBufferSize: bufferSize,
	}
}

func (ps *pubSub) newSubscriber(s BlockCreationSubscriber) *pubSubElem {
	pendingBlksChan := make(chan *block.Block, ps.pendingBlkBufferSize)
	cancelChan := make(chan interface{})
	return &pubSubElem{
		listener:          s,
		pendingBlksBuffer: pendingBlksChan,
		cancel:            cancelChan,
	}
}

// Start starts the pubsub manager
func (ps *pubSub) Start(_ context.Context) error {
	return nil
}

// AddBlockListener creates new pubSubElem subscriber and append it to blocklisteners
func (ps *pubSub) AddBlockListener(s BlockCreationSubscriber) error {
	sub := ps.newSubscriber(s)
	// create subscriber handler thread to handle pending blocks
	go ps.handler(sub)

	ps.lock.Lock()
	ps.blocklisteners = append(ps.blocklisteners, sub)
	ps.lock.Unlock()
	return nil
}

// RemoveBlockListener looks up blocklisteners and if exists, close the cancel channel and pop out the element
func (ps *pubSub) RemoveBlockListener(s BlockCreationSubscriber) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	for i, elem := range ps.blocklisteners {
		if elem.listener == s {
			close(elem.cancel)
			ps.blocklisteners[i] = nil
			ps.blocklisteners = append(ps.blocklisteners[:i], ps.blocklisteners[i+1:]...)
			log.L().Info("Successfully unsubscribe block creation.")
			return nil
		}
	}
	return errors.New("cannot find subscription")
}

// SendBlockToSubscribers sends block to every subscriber by using buffer channel.
//
// The send stays blocking on purpose: subscribers such as the indexer and the
// action pool must not miss a block. But the caller is commitBlock, running
// under the chain write lock, so a subscriber that stops draining halts the
// node -- make that loud instead of silent. The listener slice is snapshotted
// first so a stalled subscriber cannot also starve AddBlockListener.
func (ps *pubSub) SendBlockToSubscribers(blk *block.Block) {
	ps.lock.RLock()
	listeners := make([]*pubSubElem, len(ps.blocklisteners))
	copy(listeners, ps.blocklisteners)
	ps.lock.RUnlock()

	for _, elem := range listeners {
		// Every send also watches elem.cancel: a concurrent RemoveBlockListener
		// or Stop closes it and lets the handler goroutine exit, after which
		// nothing drains pendingBlksBuffer. Without the cancel case a send to a
		// removed subscriber's full buffer would block commitBlock forever.
		select {
		case elem.pendingBlksBuffer <- blk:
			continue
		case <-elem.cancel:
			continue
		default:
		}
		start := time.Now()
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(_slowSubscriberWarnInterval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					log.L().Error("block subscriber is not draining; block commit is stalled",
						zap.Uint64("height", blk.Height()),
						zap.Duration("stalled", time.Since(start)))
				}
			}
		}()
		select {
		case elem.pendingBlksBuffer <- blk:
		case <-elem.cancel:
		}
		close(done)
		if waited := time.Since(start); waited > _slowSubscriberWarnInterval {
			log.L().Warn("block subscriber resumed draining",
				zap.Uint64("height", blk.Height()), zap.Duration("stalled", waited))
		}
	}
}

// Stop stops the pubsub manager
func (ps *pubSub) Stop(_ context.Context) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	for i, elem := range ps.blocklisteners {
		close(elem.cancel)
		log.L().Info("Successfully unsubscribe block creation.", zap.Int("listener", i))
	}
	ps.blocklisteners = nil
	return nil
}

func (ps *pubSub) handler(sub *pubSubElem) {
	for {
		select {
		case <-sub.cancel:
			return
		case blk := <-sub.pendingBlksBuffer:
			if err := sub.listener.ReceiveBlock(blk); err != nil {
				log.L().Error("Failed to handle new block.", zap.Error(err))
			}
		}
	}
}
