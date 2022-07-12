// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

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

// SendBlockToSubscribers sends block to every subscriber by using buffer channel
func (ps *pubSub) SendBlockToSubscribers(blk *block.Block) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	blocklisteners := ps.blocklisteners
	for _, elem := range blocklisteners {
		elem.pendingBlksBuffer <- blk
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
