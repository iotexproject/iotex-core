// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"container/list"
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
		AddSubscriber(BlockCreationSubscriber) error
		RemoveSubscriber(BlockCreationSubscriber) error
		Subscribers() []BlockCreationSubscriber
		PubBlock(blk *block.Block) *pubTask
	}

	pubSub struct {
		lock        sync.RWMutex
		subscribers *list.List
		tasks       chan *pubTask
		ctx         context.Context
		cancel      context.CancelFunc
	}

	pubTask struct {
		block       *block.Block
		subscribers []BlockCreationSubscriber
		result      chan error
	}
)

// NewPubSub creates new pubSub struct with buffersize for pendingBlock buffer channel
func NewPubSub(bufferSize uint64) PubSubManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &pubSub{
		subscribers: list.New(),
		tasks:       make(chan *pubTask, bufferSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start run task handler
func (ps *pubSub) Start(_ context.Context) error {
	go func(ps *pubSub) {
		for {
			select {
			case <-ps.ctx.Done():
				return
			case task := <-ps.tasks:
				task.handle()
			}
		}
	}(ps)
	return nil
}

// AddSubscriber creates new pubSubElem subscriber and append it to blocklisteners
func (ps *pubSub) AddSubscriber(s BlockCreationSubscriber) error {
	if s == nil {
		return errors.New("invalid subscriber")
	}
	ps.lock.Lock()
	defer ps.lock.Unlock()
	ps.subscribers.PushBack(s)
	return nil
}

// RemoveSubscriber looks up blocklisteners and if exists, close the cancel channel and pop out the element
func (ps *pubSub) RemoveSubscriber(s BlockCreationSubscriber) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	for elem := ps.subscribers.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(BlockCreationSubscriber) == s {
			ps.subscribers.Remove(elem)
			log.L().Info("Successfully unsubscribe block creation.")
			return nil
		}
	}
	return errors.New("cannot find subscription")
}

func (ps *pubSub) Subscribers() []BlockCreationSubscriber {
	var subscribers []BlockCreationSubscriber
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	for elem := ps.subscribers.Front(); elem != nil; elem = elem.Next() {
		subscribers = append(subscribers, elem.Value.(BlockCreationSubscriber))
	}
	return subscribers
}

// PubBlock sends block to every subscriber by using buffer channel
func (ps *pubSub) PubBlock(blk *block.Block) *pubTask {
	task := &pubTask{
		block:       blk,
		subscribers: ps.Subscribers(),
		result:      make(chan error),
	}
	ps.tasks <- task
	return task
}

// Stop remove all subscribers and stop task handler
func (ps *pubSub) Stop(_ context.Context) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	for i, elem := 0, ps.subscribers.Front(); elem != nil; elem = elem.Next() {
		log.L().Info("Successfully unsubscribe all block creation.", zap.Int("listener", i))
	}
	ps.cancel()
	return nil
}

// handle task handler
func (task *pubTask) handle() {
	for i := range task.subscribers {
		err := task.subscribers[i].ReceiveBlock(task.block)
		if err != nil {
			log.L().Error("Failed to handle new block.", zap.Error(err))
			task.result <- errors.Errorf("subscriber: #%v, err: %v", task.subscribers[i], err)
			return
		}
	}
	task.result <- nil
}
