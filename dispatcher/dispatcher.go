// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	pb "github.com/iotexproject/iotex-core/proto"
)

// Subscriber is the dispatcher subscriber interface
type Subscriber interface {
	HandleAction(*pb.ActionPb) error
	HandleBlock(*pb.BlockPb) error
	HandleBlockSync(*pb.BlockPb) error
	HandleSyncRequest(string, *pb.BlockSync) error
	HandleBlockPropose(*pb.ProposePb) error
	HandleEndorse(*pb.EndorsePb) error
}

// Dispatcher is used by peers, handles incoming block and header notifications and relays announcements of new blocks.
type Dispatcher interface {
	lifecycle.StartStopper

	// AddSubscriber adds to dispatcher
	AddSubscriber(uint32, Subscriber)
	// HandleBroadcast handles the incoming broadcast message. The transportation layer semantics is at least once.
	// That said, the handler is likely to receive duplicate messages.
	HandleBroadcast(uint32, proto.Message, chan bool)
	// HandleTell handles the incoming tell message. The transportation layer semantics is exact once. The sender is
	// given for the sake of replying the message
	HandleTell(uint32, net.Addr, proto.Message, chan bool)
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
	chainID uint32
	block   *pb.BlockPb
	blkType uint32
	done    chan bool
}

func (m blockMsg) ChainID() uint32 {
	return m.chainID
}

// blockSyncMsg packages a proto block sync message.
type blockSyncMsg struct {
	chainID uint32
	sender  string
	sync    *pb.BlockSync
	done    chan bool
}

func (m blockSyncMsg) ChainID() uint32 {
	return m.chainID
}

// actionMsg packages a proto action message.
type actionMsg struct {
	chainID uint32
	action  *pb.ActionPb
	done    chan bool
}

func (m actionMsg) ChainID() uint32 {
	return m.chainID
}

// IotxDispatcher is the request and event dispatcher for iotx node.
type IotxDispatcher struct {
	started        int32
	shutdown       int32
	eventChan      chan interface{}
	eventAudit     map[uint32]int
	eventAuditLock sync.RWMutex
	wg             sync.WaitGroup
	quit           chan struct{}

	subscribers map[uint32]Subscriber
}

// NewDispatcher creates a new Dispatcher
func NewDispatcher(
	cfg config.Config,
) (Dispatcher, error) {
	d := &IotxDispatcher{
		eventChan:   make(chan interface{}, cfg.Dispatcher.EventChanSize),
		eventAudit:  make(map[uint32]int),
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
	d.subscribers[chainID] = subscriber
}

// Start starts the dispatcher.
func (d *IotxDispatcher) Start(ctx context.Context) error {
	if atomic.AddInt32(&d.started, 1) != 1 {
		return errors.New("Dispatcher already started")
	}
	logger.Info().Msg("Starting dispatcher")
	d.wg.Add(1)
	go d.newsHandler()
	return nil
}

// Stop gracefully shuts down the dispatcher by stopping all handlers and waiting for them to finish.
func (d *IotxDispatcher) Stop(ctx context.Context) error {
	if atomic.AddInt32(&d.shutdown, 1) != 1 {
		logger.Warn().Msg("Dispatcher already in the process of shutting down")
		return nil
	}
	logger.Info().Msg("Dispatcher is shutting down")
	close(d.quit)
	d.wg.Wait()
	return nil
}

// EventChan returns the event chan
func (d *IotxDispatcher) EventChan() *chan interface{} {
	return &d.eventChan
}

// EventAudit returns the event audit map
func (d *IotxDispatcher) EventAudit() map[uint32]int {
	d.eventAuditLock.RLock()
	defer d.eventAuditLock.RUnlock()
	snapshot := make(map[uint32]int)
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
				logger.Warn().
					Str("msg", msg.(string)).
					Msg("Invalid message type in block handler")
			}

		case <-d.quit:
			break loop
		}
	}

	d.wg.Done()
	logger.Info().Msg("News handler done")
}

// handleActionMsg handles actionMsg from all peers.
func (d *IotxDispatcher) handleActionMsg(m *actionMsg) {
	d.updateEventAudit(pb.MsgActionType)
	if subscriber, ok := d.subscribers[m.ChainID()]; ok {
		if err := subscriber.HandleAction(m.action); err != nil {
			requestMtc.WithLabelValues("AddAction", "false").Inc()
			logger.Debug().Err(err)
		}
	} else {
		logger.Info().Uint32("ChainID", m.ChainID()).Msg("No subscriber specified in the dispatcher")
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// handleBlockMsg handles blockMsg from peers.
func (d *IotxDispatcher) handleBlockMsg(m *blockMsg) {
	if subscriber, ok := d.subscribers[m.ChainID()]; ok {
		if m.blkType == pb.MsgBlockProtoMsgType {
			d.updateEventAudit(pb.MsgBlockProtoMsgType)
			if err := subscriber.HandleBlock(m.block); err != nil {
				logger.Error().Err(err).Msg("Fail to handle the block")
			}
		} else if m.blkType == pb.MsgBlockSyncDataType {
			d.updateEventAudit(pb.MsgBlockSyncDataType)
			if err := subscriber.HandleBlockSync(m.block); err != nil {
				logger.Error().Err(err).Msg("Fail to sync the block")
			}
		}
	} else {
		logger.Info().Uint32("ChainID", m.ChainID()).Msg("No subscriber specified in the dispatcher")
	}

	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// handleBlockSyncMsg handles block messages from peers.
func (d *IotxDispatcher) handleBlockSyncMsg(m *blockSyncMsg) {
	logger.Info().
		Str("src", m.sender).
		Uint64("start", m.sync.Start).
		Uint64("end", m.sync.End).
		Msg("receive blockSyncMsg")

	d.updateEventAudit(pb.MsgBlockSyncReqType)
	if subscriber, ok := d.subscribers[m.ChainID()]; ok {
		// dispatch to block sync
		if err := subscriber.HandleSyncRequest(m.sender, m.sync); err != nil {
			logger.Error().Err(err)
		}
	} else {
		logger.Info().Uint32("ChainID", m.ChainID()).Msg("No subscriber specified in the dispatcher")
	}
	// signal to let caller know we are done
	if m.done != nil {
		m.done <- true
	}
}

// dispatchAction adds the passed action message to the news handling queue.
func (d *IotxDispatcher) dispatchAction(chainID uint32, msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.enqueueEvent(&actionMsg{chainID, (msg).(*pb.ActionPb), done})
}

// dispatchBlockCommit adds the passed block message to the news handling queue.
func (d *IotxDispatcher) dispatchBlockCommit(chainID uint32, msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.enqueueEvent(&blockMsg{chainID, (msg).(*pb.BlockPb), pb.MsgBlockProtoMsgType, done})
}

// dispatchBlockSyncReq adds the passed block sync request to the news handling queue.
func (d *IotxDispatcher) dispatchBlockSyncReq(chainID uint32, sender string, msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	d.enqueueEvent(&blockSyncMsg{chainID, sender, (msg).(*pb.BlockSync), done})
}

// dispatchBlockSyncData handles block sync data
func (d *IotxDispatcher) dispatchBlockSyncData(chainID uint32, msg proto.Message, done chan bool) {
	if atomic.LoadInt32(&d.shutdown) != 0 {
		if done != nil {
			close(done)
		}
		return
	}
	data := (msg).(*pb.BlockContainer)
	d.enqueueEvent(&blockMsg{chainID, data.Block, pb.MsgBlockSyncDataType, done})
}

// HandleBroadcast handles incoming broadcast message
func (d *IotxDispatcher) HandleBroadcast(chainID uint32, message proto.Message, done chan bool) {
	msgType, err := pb.GetTypeFromProtoMsg(message)
	if err != nil {
		logger.Warn().
			Str("error", err.Error()).
			Msg("unexpected message handled by HandleBroadcast")
	}
	subscriber, ok := d.subscribers[chainID]
	if !ok {
		logger.Warn().
			Uint32("chainID", chainID).
			Msg("chainID has not been registered in dispatcher")
		return
	}

	switch msgType {
	case pb.MsgProposeProtoMsgType:
		err := subscriber.HandleBlockPropose(message.(*pb.ProposePb))
		if err != nil {
			logger.Error().
				Err(err).
				Msg("failed to handle block propose")
		}
		if done != nil {
			done <- true
		}
	case pb.MsgEndorseProtoMsgType:
		err := subscriber.HandleEndorse(message.(*pb.EndorsePb))
		if err != nil {
			logger.Error().
				Err(err).
				Msg("failed to handle endorse")
		}
		if done != nil {
			done <- true
		}
	case pb.MsgActionType:
		d.dispatchAction(chainID, message, done)
	case pb.MsgBlockProtoMsgType:
		d.dispatchBlockCommit(chainID, message, done)
	default:
		logger.Warn().
			Uint32("msgType", msgType).
			Msg("unexpected msgType handled by HandleBroadcast")
	}
}

// HandleTell handles incoming unicast message
func (d *IotxDispatcher) HandleTell(chainID uint32, sender net.Addr, message proto.Message, done chan bool) {
	msgType, err := pb.GetTypeFromProtoMsg(message)
	if err != nil {
		logger.Warn().
			Str("error", err.Error()).
			Msg("unexpected message handled by HandleTell")
	}
	switch msgType {
	case pb.MsgBlockSyncReqType:
		d.dispatchBlockSyncReq(chainID, sender.String(), message, done)
	case pb.MsgBlockSyncDataType:
		d.dispatchBlockSyncData(chainID, message, done)
	default:
		logger.Warn().
			Uint32("msgType", msgType).
			Msg("unexpected msgType handled by HandleTell")
	}
}

func (d *IotxDispatcher) enqueueEvent(event interface{}) {
	go func() {
		if len(d.eventChan) == cap(d.eventChan) {
			logger.Warn().Msg("dispatcher event chan is full, drop an event")
			return
		}
		d.eventChan <- event
	}()
}

func (d *IotxDispatcher) updateEventAudit(t uint32) {
	d.eventAuditLock.Lock()
	defer d.eventAuditLock.Unlock()
	d.eventAudit[t]++
}
