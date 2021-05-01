// Copyright (c) 2019 IoTeX Foundation
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
	goproto "github.com/iotexproject/iotex-proto/golang"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// Subscriber is the dispatcher subscriber interface
type Subscriber interface {
	HandleAction(context.Context, *iotextypes.Action) error
	HandleBlock(context.Context, *iotextypes.Block) error
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
	actionChanLock sync.RWMutex
	blockChanLock  sync.RWMutex
	syncChanLock   sync.RWMutex
	actionChan     chan *actionMsg
	blockChan      chan *blockMsg
	syncChan       chan *blockSyncMsg
	eventAudit     map[iotexrpc.MessageType]int
	eventAuditLock sync.RWMutex
	wg             sync.WaitGroup
	quit           chan struct{}
	subscribers    map[uint32]Subscriber
	subscribersMU  sync.RWMutex
}

// NewDispatcher creates a new Dispatcher
func NewDispatcher(cfg config.Config) (Dispatcher, error) {
	d := &IotxDispatcher{
		actionChan:  make(chan *actionMsg, cfg.Dispatcher.EventChanSize),
		blockChan:   make(chan *blockMsg, cfg.Dispatcher.EventChanSize),
		syncChan:    make(chan *blockSyncMsg, cfg.Dispatcher.EventChanSize),
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
	d.wg.Add(3)
	go d.actionHandler()
	go d.blockHandler()
	go d.syncHandler()

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

// EventQueueSize returns the event queue size
func (d *IotxDispatcher) EventQueueSize() int {
	d.eventAuditLock.RLock()
	defer d.eventAuditLock.RUnlock()
	return len(d.actionChan) + len(d.blockChan) + len(d.syncChan)
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

func (d *IotxDispatcher) actionHandler() {
	for {
		select {
		case a := <-d.actionChan:
			d.handleActionMsg(a)
		case <-d.quit:
			d.wg.Done()
			log.L().Info("action handler is terminated.")
			return
		}
	}
}

// blockHandler is the main handler for handling all news from peers.
func (d *IotxDispatcher) blockHandler() {
	for {
		select {
		case b := <-d.blockChan:
			d.handleBlockMsg(b)
		case <-d.quit:
			d.wg.Done()
			log.L().Info("block handler is terminated.")
			return
		}
	}
}

// syncHandler handles incoming block sync requests
func (d *IotxDispatcher) syncHandler() {
loop:
	for {
		select {
		case m := <-d.syncChan:
			d.handleBlockSyncMsg(m)
		case <-d.quit:
			break loop
		}
	}

	d.wg.Done()
	log.L().Info("block sync handler done.")
}

// handleActionMsg handles actionMsg from all peers.
func (d *IotxDispatcher) handleActionMsg(m *actionMsg) {
	log.L().Debug("receive actionMsg.")

	d.subscribersMU.RLock()
	subscriber, ok := d.subscribers[m.ChainID()]
	d.subscribersMU.RUnlock()
	if ok {
		d.updateEventAudit(iotexrpc.MessageType_ACTION)
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
	log.L().Debug("receive blockMsg.", zap.Uint64("height", m.block.GetHeader().GetCore().GetHeight()))

	d.subscribersMU.RLock()
	subscriber, ok := d.subscribers[m.ChainID()]
	d.subscribersMU.RUnlock()
	if ok {
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
	log.L().Debug("Receive blockSyncMsg.",
		zap.String("src", fmt.Sprintf("%v", m.peer)),
		zap.Uint64("start", m.sync.Start),
		zap.Uint64("end", m.sync.End))

	d.subscribersMU.RLock()
	subscriber, ok := d.subscribers[m.ChainID()]
	d.subscribersMU.RUnlock()
	if ok {
		d.updateEventAudit(iotexrpc.MessageType_BLOCK_REQUEST)
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
	d.actionChanLock.Lock()
	defer d.actionChanLock.Unlock()
	if len(d.actionChan) < cap(d.actionChan) {
		d.actionChan <- &actionMsg{
			ctx:     ctx,
			chainID: chainID,
			action:  (msg).(*iotextypes.Action),
		}
		return
	}
	log.L().Warn("dispatcher action chan is full, drop an event.")
}

// dispatchBlock adds the passed block message to the news handling queue.
func (d *IotxDispatcher) dispatchBlock(ctx context.Context, chainID uint32, msg proto.Message) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		return
	}
	d.blockChanLock.Lock()
	defer d.blockChanLock.Unlock()
	if len(d.blockChan) < cap(d.blockChan) {
		d.blockChan <- &blockMsg{
			ctx:     ctx,
			chainID: chainID,
			block:   (msg).(*iotextypes.Block),
		}
		return
	}
	log.L().Warn("dispatcher block chan is full, drop an event.")
}

// dispatchBlockSyncReq adds the passed block sync request to the news handling queue.
func (d *IotxDispatcher) dispatchBlockSyncReq(ctx context.Context, chainID uint32, peer peerstore.PeerInfo, msg proto.Message) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		return
	}
	d.syncChanLock.Lock()
	defer d.syncChanLock.Unlock()
	if len(d.syncChan) < cap(d.syncChan) {
		d.syncChan <- &blockSyncMsg{
			ctx:     ctx,
			chainID: chainID,
			peer:    peer,
			sync:    (msg).(*iotexrpc.BlockSync),
		}
		return
	}
	log.L().Warn("dispatcher sync chan is full, drop an event.")
}

// HandleBroadcast handles incoming broadcast message
func (d *IotxDispatcher) HandleBroadcast(ctx context.Context, chainID uint32, message proto.Message) {
	msgType, err := goproto.GetTypeFromRPCMsg(message)
	if err != nil {
		log.L().Warn("Unexpected message handled by HandleBroadcast.", zap.Error(err))
	}
	d.subscribersMU.RLock()
	subscriber, ok := d.subscribers[chainID]
	d.subscribersMU.RUnlock()
	if !ok {
		log.L().Warn("chainID has not been registered in dispatcher.", zap.Uint32("chainID", chainID))
		return
	}

	switch msgType {
	case iotexrpc.MessageType_CONSENSUS:
		if err := subscriber.HandleConsensusMsg(message.(*iotextypes.ConsensusMessage)); err != nil {
			log.L().Debug("Failed to handle consensus message.", zap.Error(err))
		}
	case iotexrpc.MessageType_ACTION:
		d.dispatchAction(ctx, chainID, message)
	case iotexrpc.MessageType_BLOCK:
		d.dispatchBlock(ctx, chainID, message)
	default:
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
	}
}

// HandleTell handles incoming unicast message
func (d *IotxDispatcher) HandleTell(ctx context.Context, chainID uint32, peer peerstore.PeerInfo, message proto.Message) {
	msgType, err := goproto.GetTypeFromRPCMsg(message)
	if err != nil {
		log.L().Warn("Unexpected message handled by HandleTell.", zap.Error(err))
	}
	switch msgType {
	case iotexrpc.MessageType_BLOCK_REQUEST:
		d.dispatchBlockSyncReq(ctx, chainID, peer, message)
	case iotexrpc.MessageType_BLOCK:
		d.dispatchBlock(ctx, chainID, message)
	default:
		log.L().Warn("Unexpected msgType handled by HandleTell.", zap.Any("msgType", msgType))
	}
}

func (d *IotxDispatcher) updateEventAudit(t iotexrpc.MessageType) {
	d.eventAuditLock.Lock()
	defer d.eventAuditLock.Unlock()
	d.eventAudit[t]++
}
