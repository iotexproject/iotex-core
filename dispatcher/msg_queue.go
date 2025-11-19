package dispatcher

import (
	"context"
	"sync"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"go.uber.org/zap"

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
	queueLimit struct {
		counts map[string]int
		limit  int
		mu     sync.RWMutex
	}
	msgQueueMgr struct {
		queues            map[string]msgQueue
		blockRequestPeers queueLimit
		wg                sync.WaitGroup
		handleMsg         func(msg *message)
		quit              chan struct{}
	}
	msgQueue       chan *message
	msgQueueConfig struct {
		actionChanSize uint
		blockChanSize  uint
		blockSyncSize  uint
		blockSyncLimit uint
		consensusSize  uint
		miscSize       uint
	}
)

func (q *queueLimit) increment(peer string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.counts[peer] >= q.limit {
		return false
	}
	q.counts[peer]++
	return true
}

func (q *queueLimit) decrement(peer string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.counts[peer] <= 0 {
		return
	}
	q.counts[peer]--
}

func newMsgQueueMgr(cfg msgQueueConfig, handler func(msg *message)) *msgQueueMgr {
	queues := make(map[string]msgQueue)
	queues[actionQ] = make(chan *message, cfg.actionChanSize)
	queues[blockQ] = make(chan *message, cfg.blockChanSize)
	queues[blockSyncQ] = make(chan *message, cfg.blockSyncSize)
	queues[consensusQ] = make(chan *message, cfg.consensusSize)
	queues[miscQ] = make(chan *message, cfg.miscSize)
	return &msgQueueMgr{
		queues:            queues,
		blockRequestPeers: queueLimit{counts: make(map[string]int), limit: int(cfg.blockSyncLimit), mu: sync.RWMutex{}},
		handleMsg:         handler,
		quit:              make(chan struct{}),
	}
}

func (m *msgQueueMgr) Start(ctx context.Context) error {
	for i := 0; i <= 4; i++ {
		m.wg.Add(1)
		go m.consume(actionQ)
	}

	m.wg.Add(1)
	go m.consume(blockQ)

	for i := 0; i <= 2; i++ {
		m.wg.Add(1)
		go m.consume(blockSyncQ)
	}
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
			if q == blockSyncQ {
				m.blockRequestPeers.decrement(msg.peer)
			}
			m.handleMsg(msg)
		case <-m.quit:
			log.L().Debug("message handler is terminated.")
			return
		}
	}
}

func (m *msgQueueMgr) Queue(msg *message, subscriber Subscriber) bool {
	var queue msgQueue
	switch msg.msgType {
	case iotexrpc.MessageType_ACTION, iotexrpc.MessageType_ACTIONS, iotexrpc.MessageType_ACTION_HASH, iotexrpc.MessageType_ACTION_REQUEST:
		queue = m.queues[actionQ]
	case iotexrpc.MessageType_BLOCK:
		queue = m.queues[blockQ]
	case iotexrpc.MessageType_BLOCK_REQUEST:
		if !m.blockRequestPeers.increment(msg.peer) {
			log.L().Warn("Peer has reached the block sync request limit.", zap.String("peer", msg.peer), zap.Int("limit", m.blockRequestPeers.limit))
			return false
		}
		queue = m.queues[blockSyncQ]
	case iotexrpc.MessageType_CONSENSUS:
		queue = m.queues[consensusQ]
	default:
		queue = m.queues[miscQ]
	}
	if !subscriber.Filter(msg.msgType, msg.msg, cap(queue)) {
		log.L().Debug("Message filtered by subscriber.", zap.Uint32("chainID", msg.chainID), zap.String("msgType", msg.msgType.String()))
		return false
	}
	select {
	case queue <- msg:
	default:
		log.L().Warn("Queue is full.", zap.Any("msgType", msg.msgType))
		return false
	}
	subscriber.ReportFullness(msg.ctx, msg.msgType, msg.msg, float32(len(queue))/float32(cap(queue)))
	return true
}
