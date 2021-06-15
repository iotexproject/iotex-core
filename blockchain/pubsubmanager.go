// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// PubSubManager is an interface which handles multi-thread publisher and subscribers
type PubSubManager interface {
	AddBlockListener(BlockCreationSubscriber) error
	RemoveBlockListener(BlockCreationSubscriber) error
	SendBlockToSubscribers(*block.Block)
}

// pubSubElem includes Subscriber, buffered channel for storing the pending blocks and cancel channel to end the handler thread
type pubSubElem struct {
	listener          BlockCreationSubscriber
	pendingBlksBuffer chan *block.Block
	cancel            chan interface{}
}

// pubSub defines array of blockListener to handle multi-thread publish/subscribe
type pubSub struct {
	blocklisteners       []*pubSubElem
	pendingBlkBufferSize uint64
}

// NewPubSub creates new pubSub struct with buffersize for pendingBlock buffer channel
func NewPubSub(bufferSize uint64) PubSubManager {
	return &pubSub{
		blocklisteners:       make([]*pubSubElem, 0),
		pendingBlkBufferSize: bufferSize,
	}
}

// AddBlockListener creates new pubSubElem subscriber and append it to blocklisteners
func (ps *pubSub) AddBlockListener(s BlockCreationSubscriber) error {
	pendingBlksChan := make(chan *block.Block, ps.pendingBlkBufferSize)
	cancelChan := make(chan interface{})
	// create subscriber handler thread to handle pending blocks
	go ps.handler(cancelChan, pendingBlksChan, s)

	pubSubElem := &pubSubElem{
		listener:          s,
		pendingBlksBuffer: pendingBlksChan,
		cancel:            cancelChan,
	}
	ps.blocklisteners = append(ps.blocklisteners, pubSubElem)

	return nil
}

// RemoveBlockListener looks up blocklisteners and if exists, close the cancel channel and pop out the element
func (ps *pubSub) RemoveBlockListener(s BlockCreationSubscriber) error {
	for i, elem := range ps.blocklisteners {
		if elem.listener == s {
			close(elem.cancel)
			ps.blocklisteners = append(ps.blocklisteners[:i], ps.blocklisteners[i+1:]...)
			log.L().Info("Successfully unsubscribe block creation.")
			return nil
		}
	}
	return errors.New("cannot find subscription")
}

// SendBlockToSubscribers sends block to every subscriber by using buffer channel
func (ps *pubSub) SendBlockToSubscribers(blk *block.Block) {
	for _, elem := range ps.blocklisteners {
		elem.pendingBlksBuffer <- blk
	}
}

func (ps *pubSub) handler(cancelChan <-chan interface{}, pendingBlks <-chan *block.Block, s BlockCreationSubscriber) {
	for {
		select {
		case <-cancelChan:
			return
		case blk := <-pendingBlks:
			if err := s.ReceiveBlock(blk); err != nil {
				log.L().Error("Failed to handle new block.", zap.Error(err))
			}
		}
	}
}
