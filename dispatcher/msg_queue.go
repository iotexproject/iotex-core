package dispatcher

import (
	"context"
	"sync"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	actionQ    = "action"
	blockQ     = "block"
	blockSyncQ = "blockSync"
	consensusQ = "consensus"
	miscQ      = "misc"
)

type (
	msgQueueMgr struct {
		queues    map[string]msgQueue
		wg        sync.WaitGroup
		handleMsg func(msg *message)
		quit      chan struct{}
	}
	msgQueue       chan *message
	msgQueueConfig struct {
		actionChanSize uint
		blockChanSize  uint
		blockSyncSize  uint
		consensusSize  uint
		miscSize       uint
	}
)

func newMsgQueueMgr(cfg msgQueueConfig, handler func(msg *message)) *msgQueueMgr {
	queues := make(map[string]msgQueue)
	queues[actionQ] = make(chan *message, cfg.actionChanSize)
	queues[blockQ] = make(chan *message, cfg.blockChanSize)
	queues[blockSyncQ] = make(chan *message, cfg.blockSyncSize)
	queues[consensusQ] = make(chan *message, cfg.consensusSize)
	queues[miscQ] = make(chan *message, cfg.miscSize)
	return &msgQueueMgr{
		queues:    queues,
		handleMsg: handler,
		quit:      make(chan struct{}),
	}
}

func (m *msgQueueMgr) Start(ctx context.Context) error {
	for i := 0; i <= 4; i++ {
		m.wg.Add(1)
		go m.consume(actionQ)
	}

	m.wg.Add(1)
	go m.consume(blockQ)

	m.wg.Add(1)
	go m.consume(blockSyncQ)

	m.wg.Add(1)
	go m.consume(consensusQ)

	m.wg.Add(1)
	go m.consume(miscQ)
	return nil
}

func (m *msgQueueMgr) Stop() error {
	close(m.quit)
	m.wg.Wait()
	return nil
}

func (m *msgQueueMgr) consume(q string) {
	defer m.wg.Done()
	for {
		select {
		case msg := <-m.queues[q]:
			m.handleMsg(msg)
		case <-m.quit:
			log.L().Debug("message handler is terminated.")
			return
		}
	}
}

func (m *msgQueueMgr) Queue(msg *message) msgQueue {
	switch msg.msgType {
	case iotexrpc.MessageType_ACTION, iotexrpc.MessageType_ACTIONS, iotexrpc.MessageType_ACTION_HASH, iotexrpc.MessageType_ACTION_REQUEST:
		return m.queues[actionQ]
	case iotexrpc.MessageType_BLOCK:
		return m.queues[blockQ]
	case iotexrpc.MessageType_BLOCK_REQUEST:
		return m.queues[blockSyncQ]
	case iotexrpc.MessageType_CONSENSUS:
		return m.queues[consensusQ]
	default:
		return m.queues[miscQ]
	}
}
