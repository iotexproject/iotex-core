// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen"
	"github.com/iotexproject/iotex-core/protogen/iotexrpc"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// Subscriber is the dispatcher subscriber interface
type Subscriber interface {
	HandleAction(context.Context, *iotextypes.Action) error
	HandleBlock(context.Context, *iotextypes.Block) error
	HandleBlockSync(context.Context, *iotextypes.Block) error
	HandleSyncRequest(context.Context, peerstore.PeerInfo, *iotexrpc.BlockSync) error
	HandleConsensusMsg(*iotextypes.ConsensusMessage) error
}

// Dispatcher is used by peers, handles incoming block and header notifications and relays announcements of new blocks.
type Dispatcher interface {
	lifecycle.StartStopper

	// AddSubscriber adds to dispatcher
	AddSubscriber(uint32, Subscriber)
	// HandleBroadcast handles the incoming broadcast message. The transportation layer semantics is at least once.
	// That said, the handler is likely to receive duplicate messages.
	HandleBroadcast(context.Context, uint32, proto.Message)
	// HandleTell handles the incoming tell message. The transportation layer semantics is exact once. The sender is
	// given for the sake of replying the message
	HandleTell(context.Context, uint32, peerstore.PeerInfo, proto.Message)
}

var requestMtc = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "iotex_dispatch_request",
		Help: "Dispatcher request counter.",
	},
	[]string{"method", "succeed"},
)

func init() {
	prometheus.MustRegister(requestMtc)
}

// blockMsg packages a proto block message.
type blockMsg struct {
	ctx     context.Context
	chainID uint32
	block   *iotextypes.Block
}

func (m blockMsg) ChainID() uint32 {
	return m.chainID
}

// blockSyncMsg packages a proto block sync message.
type blockSyncMsg struct {
	ctx     context.Context
	chainID uint32
	sync    *iotexrpc.BlockSync
	peer    peerstore.PeerInfo
}

func (m blockSyncMsg) ChainID() uint32 {
	return m.chainID
}

// actionMsg packages a proto action message.
type actionMsg struct {
	ctx     context.Context
	chainID uint32
	action  *iotextypes.Action
}

func (m actionMsg) ChainID() uint32 {
	return m.chainID
}

// IotxDispatcher is the request and event dispatcher for iotx node.
type IotxDispatcher struct {
	started        int32
	shutdown       int32
	eventChan      chan interface{}
	eventAudit     map[iotexrpc.MessageType]int
	eventAuditLock sync.RWMutex
	wg             sync.WaitGroup
	quit           chan struct{}

	subscribers   map[uint32]Subscriber
	subscribersMU sync.RWMutex
}

// NewDispatcher creates a new Dispatcher
func NewDispatcher(cfg config.Config) (Dispatcher, error) {
	d := &IotxDispatcher{
		eventChan:   make(chan interface{}, cfg.Dispatcher.EventChanSize),
		eventAudit:  make(map[iotexrpc.MessageType]int),
		quit:        make(chan struct{}),
		subscribers: make(map[uint32]Subscriber),
	}
	return d, nil
}

// AddSubscriber adds a subscriber to dispatcher
func (d *IotxDispatcher) AddSubscriber(
	chainID uint32,
	subscriber Subscriber,
) {
	d.subscribersMU.Lock()
	d.subscribers[chainID] = subscriber
	d.subscribersMU.Unlock()
}

// Start starts the dispatcher.
func (d *IotxDispatcher) Start(ctx context.Context) error {
	if atomic.AddInt32(&d.started, 1) != 1 {
		return errors.New("Dispatcher already started")
	}
	log.L().Info("Starting dispatcher.")
	d.wg.Add(1)
	go d.newsHandler()
	return nil
}

// Stop gracefully shuts down the dispatcher by stopping all handlers and waiting for them to finish.
func (d *IotxDispatcher) Stop(ctx context.Context) error {
	if atomic.AddInt32(&d.shutdown, 1) != 1 {
		log.L().Warn("Dispatcher already in the process of shutting down.")
		return nil
	}
	log.L().Info("Dispatcher is shutting down.")
	close(d.quit)
	d.wg.Wait()
	return nil
}

// EventChan returns the event chan
func (d *IotxDispatcher) EventChan() *chan interface{} {
	return &d.eventChan
}

// EventAudit returns the event audit map
func (d *IotxDispatcher) EventAudit() map[iotexrpc.MessageType]int {
	d.eventAuditLock.RLock()
	defer d.eventAuditLock.RUnlock()
	snapshot := make(map[iotexrpc.MessageType]int)
	for k, v := range d.eventAudit {
		snapshot[k] = v
	}
	return snapshot
}

// newsHandler is the main handler for handling all news from peers.
func (d *IotxDispatcher) newsHandler() {
loop:
	for {
		select {
		case m := <-d.eventChan:
			switch msg := m.(type) {
			case *actionMsg:
				d.handleActionMsg(msg)
			case *blockMsg:
				d.handleBlockMsg(msg)
			case *blockSyncMsg:
				d.handleBlockSyncMsg(msg)

			default:
				log.L().Warn("Invalid message type in block handler.", zap.Any("msg", msg))
			}

		case <-d.quit:
			break loop
		}
	}

	d.wg.Done()
	log.L().Info("News handler done.")
}

// handleActionMsg handles actionMsg from all peers.
func (d *IotxDispatcher) handleActionMsg(m *actionMsg) {
	d.updateEventAudit(iotexrpc.MessageType_ACTION)
	if subscriber, ok := d.subscribers[m.ChainID()]; ok {
		if err := subscriber.HandleAction(m.ctx, m.action); err != nil {
			requestMtc.WithLabelValues("AddAction", "false").Inc()
			log.L().Debug("Handle action request error.", zap.Error(err))
		}
	} else {
		log.L().Info("No subscriber specified in the dispatcher.", zap.Uint32("chainID", m.ChainID()))
	}
}

// handleBlockMsg handles blockMsg from peers.
func (d *IotxDispatcher) handleBlockMsg(m *blockMsg) {
	d.subscribersMU.RLock()
	defer d.subscribersMU.RUnlock()
	if subscriber, ok := d.subscribers[m.ChainID()]; ok {
		d.updateEventAudit(iotexrpc.MessageType_BLOCK)
		if err := subscriber.HandleBlock(m.ctx, m.block); err != nil {
			log.L().Error("Fail to handle the block.", zap.Error(err))
		}
	} else {
		log.L().Info("No subscriber specified in the dispatcher.", zap.Uint32("chainID", m.ChainID()))
	}
}

// handleBlockSyncMsg handles block messages from peers.
func (d *IotxDispatcher) handleBlockSyncMsg(m *blockSyncMsg) {
	log.L().Info("Receive blockSyncMsg.",
		zap.String("src", fmt.Sprintf("%v", m.peer)),
		zap.Uint64("start", m.sync.Start),
		zap.Uint64("end", m.sync.End))

	d.updateEventAudit(iotexrpc.MessageType_BLOCK_REQUEST)
	if subscriber, ok := d.subscribers[m.ChainID()]; ok {
		// dispatch to block sync
		if err := subscriber.HandleSyncRequest(m.ctx, m.peer, m.sync); err != nil {
			log.L().Error("Failed to handle sync request.", zap.Error(err))
		}
	} else {
		log.L().Info("No subscriber specified in the dispatcher.", zap.Uint32("chainID", m.ChainID()))
	}
}

// dispatchAction adds the passed action message to the news handling queue.
func (d *IotxDispatcher) dispatchAction(ctx context.Context, chainID uint32, msg proto.Message) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		return
	}
	d.enqueueEvent(&actionMsg{
		ctx:     ctx,
		chainID: chainID,
		action:  (msg).(*iotextypes.Action),
	})
}

// dispatchBlockCommit adds the passed block message to the news handling queue.
func (d *IotxDispatcher) dispatchBlockCommit(ctx context.Context, chainID uint32, msg proto.Message) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		return
	}
	d.enqueueEvent(&blockMsg{
		ctx:     ctx,
		chainID: chainID,
		block:   (msg).(*iotextypes.Block),
	})
}

// dispatchBlockSyncReq adds the passed block sync request to the news handling queue.
func (d *IotxDispatcher) dispatchBlockSyncReq(ctx context.Context, chainID uint32, peer peerstore.PeerInfo, msg proto.Message) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		return
	}
	d.enqueueEvent(&blockSyncMsg{
		ctx:     ctx,
		chainID: chainID,
		peer:    peer,
		sync:    (msg).(*iotexrpc.BlockSync),
	})
}

// HandleBroadcast handles incoming broadcast message
func (d *IotxDispatcher) HandleBroadcast(ctx context.Context, chainID uint32, message proto.Message) {
	msgType, err := protogen.GetTypeFromRPCMsg(message)
	if err != nil {
		log.L().Warn("Unexpected message handled by HandleBroadcast.", zap.Error(err))
	}
	d.subscribersMU.RLock()
	subscriber, ok := d.subscribers[chainID]
	if !ok {
		log.L().Warn("chainID has not been registered in dispatcher.", zap.Uint32("chainID", chainID))
		d.subscribersMU.RUnlock()
		return
	}
	d.subscribersMU.RUnlock()

	switch msgType {
	case iotexrpc.MessageType_CONSENSUS:
		if err := subscriber.HandleConsensusMsg(message.(*iotextypes.ConsensusMessage)); err != nil {
			log.L().Debug("Failed to handle consensus message.", zap.Error(err))
		}
	case iotexrpc.MessageType_ACTION:
		d.dispatchAction(ctx, chainID, message)
	case iotexrpc.MessageType_BLOCK:
		d.dispatchBlockCommit(ctx, chainID, message)
	default:
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
	}
}

// HandleTell handles incoming unicast message
func (d *IotxDispatcher) HandleTell(ctx context.Context, chainID uint32, peer peerstore.PeerInfo, message proto.Message) {
	msgType, err := protogen.GetTypeFromRPCMsg(message)
	if err != nil {
		log.L().Warn("Unexpected message handled by HandleTell.", zap.Error(err))
	}
	switch msgType {
	case iotexrpc.MessageType_BLOCK_REQUEST:
		d.dispatchBlockSyncReq(ctx, chainID, peer, message)
	case iotexrpc.MessageType_BLOCK:
		d.dispatchBlockCommit(ctx, chainID, message)
	default:
		log.L().Warn("Unexpected msgType handled by HandleTell.", zap.Any("msgType", msgType))
	}
}

func (d *IotxDispatcher) enqueueEvent(event interface{}) {
	go func() {
		if len(d.eventChan) == cap(d.eventChan) {
			log.L().Debug("dispatcher event chan is full, drop an event.")
			return
		}
		d.eventChan <- event
	}()
}

func (d *IotxDispatcher) updateEventAudit(t iotexrpc.MessageType) {
	d.eventAuditLock.Lock()
	defer d.eventAuditLock.Unlock()
	d.eventAudit[t]++
}
