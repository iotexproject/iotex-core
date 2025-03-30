// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	goproto "github.com/iotexproject/iotex-proto/golang"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	// Config is the config for dispatcher
	Config struct {
		ActionChanSize             uint          `yaml:"actionChanSize"`
		BlockChanSize              uint          `yaml:"blockChanSize"`
		BlockSyncChanSize          uint          `yaml:"blockSyncChanSize"`
		ConsensusChanSize          uint          `yaml:"consensusChanSize"`
		MiscChanSize               uint          `yaml:"miscChanSize"`
		ProcessSyncRequestInterval time.Duration `yaml:"processSyncRequestInterval"`
		AccountRateLimit           uint          `yaml:"accountRateLimit"`
		// TODO: explorer dependency deleted at #1085, need to revive by migrating to api
	}
)

var (
	// DefaultConfig is the default config
	DefaultConfig = Config{
		ActionChanSize:    5000,
		BlockChanSize:     1000,
		BlockSyncChanSize: 400,
		ConsensusChanSize: 1000,
		MiscChanSize:      1000,
		AccountRateLimit:  100,

		ProcessSyncRequestInterval: 0 * time.Second,
	}
)

// Dispatcher is used by peers, handles incoming block and header notifications and relays announcements of new blocks.
type Dispatcher interface {
	lifecycle.StartStopper

	// AddSubscriber adds to dispatcher
	AddSubscriber(uint32, Subscriber)
	// ValidateMessage validates the message
	ValidateMessage(proto.Message) (bool, error)
	// HandleBroadcast handles the incoming broadcast message. The transportation layer semantics is at least once.
	// That said, the handler is likely to receive duplicate messages.
	HandleBroadcast(context.Context, uint32, string, proto.Message)
	// HandleTell handles the incoming tell message. The transportation layer semantics is exact once. The sender is
	// given for the sake of replying the message
	HandleTell(context.Context, uint32, peer.AddrInfo, proto.Message)
}

var (
	requestMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_dispatch_request",
			Help: "Dispatcher request counter.",
		},
		[]string{"method", "succeed"},
	)
)

func init() {
	prometheus.MustRegister(requestMtc)
}

type (
	VerificationFunc func(pm proto.Message) (string, error)

	// IotxDispatcher is the request and event dispatcher for iotx node.
	IotxDispatcher struct {
		lifecycle.Readiness
		lifecycle.Lifecycle
		// queue manager
		queueMgr *msgQueueMgr
		// event stats
		eventAudit     map[iotexrpc.MessageType]int
		eventAuditLock sync.RWMutex
		// subscribers
		subscribers   map[uint32]Subscriber
		subscribersMU sync.RWMutex
		// filter for blocksync message
		peerLastSync     map[string]time.Time
		syncInterval     time.Duration
		peerSyncLock     sync.RWMutex
		ratelimiter      *RateLimiter
		verificationFunc VerificationFunc
	}

	message struct {
		ctx      context.Context
		chainID  uint32
		msg      proto.Message
		msgType  iotexrpc.MessageType
		peerInfo *peer.AddrInfo // peerInfo is only used for unicast message
		peer     string
	}
)

// NewDispatcher creates a new Dispatcher
func NewDispatcher(cfg Config, verificationFunc VerificationFunc) (Dispatcher, error) {
	d := &IotxDispatcher{
		subscribers:      make(map[uint32]Subscriber),
		peerLastSync:     make(map[string]time.Time),
		syncInterval:     cfg.ProcessSyncRequestInterval,
		eventAudit:       make(map[iotexrpc.MessageType]int),
		ratelimiter:      NewRateLimiter(100000, rate.Limit(cfg.AccountRateLimit), int(cfg.AccountRateLimit)),
		verificationFunc: verificationFunc,
	}
	queueMgr := newMsgQueueMgr(msgQueueConfig{
		actionChanSize: cfg.ActionChanSize,
		blockChanSize:  cfg.BlockChanSize,
		blockSyncSize:  cfg.BlockSyncChanSize,
		consensusSize:  cfg.ConsensusChanSize,
		miscSize:       cfg.MiscChanSize,
	}, func(msg *message) {
		if !d.filter(msg) {
			return
		}
		d.dispatchMsg(msg)
	})
	d.queueMgr = queueMgr
	d.Lifecycle.Add(d.queueMgr)
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
	log.L().Info("Starting dispatcher.")
	if err := d.OnStart(ctx); err != nil {
		return err
	}
	return d.TurnOn()
}

// Stop gracefully shuts down the dispatcher by stopping all handlers and waiting for them to finish.
func (d *IotxDispatcher) Stop(ctx context.Context) error {
	if err := d.TurnOff(); err != nil {
		log.L().Warn("Dispatcher already in the process of shutting down.")
		return err
	}
	log.L().Info("Dispatcher is shutting down.")
	return d.OnStop(ctx)
}

// EventQueueSize returns the event queue size
func (d *IotxDispatcher) EventQueueSize() map[string]int {
	res := make(map[string]int)
	for k, v := range d.queueMgr.queues {
		res[k] = len(v)
	}
	return res
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

func (d *IotxDispatcher) subscriber(chainID uint32) Subscriber {
	d.subscribersMU.RLock()
	defer d.subscribersMU.RUnlock()
	subscriber, ok := d.subscribers[chainID]
	if !ok {
		return nil
	}

	return subscriber
}

// ValidateMessage validates the message
func (d *IotxDispatcher) ValidateMessage(pMsg proto.Message) (bool, error) {
	switch pb := pMsg.(type) {
	case *iotextypes.Action:
		sender, err := d.verificationFunc(pb)
		if err != nil {
			return true, err
		}
		return d.ratelimiter.Remainings(sender) == 0, nil
	case *iotextypes.Actions:
		actions := pb.GetActions()
		counters := make(map[string]int, len(actions))
		for _, act := range actions {
			sender, err := d.verificationFunc(act)
			if err != nil {
				return true, err
			}
			counters[sender]++
		}
		for sender, count := range counters {
			if d.ratelimiter.Remainings(sender) < count {
				return true, nil
			}
		}
	}
	return false, nil
}

func (d *IotxDispatcher) queueMessage(msg *message) {
	queue := d.queueMgr.Queue(msg)
	select {
	case queue <- msg:
	default:
		log.L().Warn("Queue is full.", zap.Any("msgType", msg.msgType))
	}
	d.updateMetrics(msg, queue)
}

// HandleBroadcast handles incoming broadcast message
func (d *IotxDispatcher) HandleBroadcast(ctx context.Context, chainID uint32, peer string, msgProto proto.Message) {
	if !d.IsReady() {
		return
	}
	msgType, err := goproto.GetTypeFromRPCMsg(msgProto)
	if err != nil {
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
		return
	}
	switch actions := msgProto.(type) {
	case *iotextypes.Actions:
		for _, act := range actions.Actions {
			d.ratelimiter.Wait(string(act.SenderPubKey))
			msg := &message{
				ctx:     ctx,
				chainID: chainID,
				msg:     act,
				peer:    peer,
				msgType: msgType,
			}
			d.queueMessage(msg)
		}
		return
	case *iotextypes.Action:
		d.ratelimiter.Wait(string(actions.SenderPubKey))
	}
	msg := &message{
		ctx:     ctx,
		chainID: chainID,
		msg:     msgProto,
		peer:    peer,
		msgType: msgType,
	}
	d.queueMessage(msg)
}

// HandleTell handles incoming unicast message
func (d *IotxDispatcher) HandleTell(ctx context.Context, chainID uint32, peer peer.AddrInfo, msgProto proto.Message) {
	if !d.IsReady() {
		return
	}
	msgType, err := goproto.GetTypeFromRPCMsg(msgProto)
	if err != nil {
		log.L().Warn("Unexpected message handled by HandleTell.", zap.Error(err))
	}
	cp := peer
	msg := &message{
		ctx:      ctx,
		chainID:  chainID,
		msg:      msgProto,
		peerInfo: &cp,
		peer:     cp.ID.String(),
		msgType:  msgType,
	}
	d.queueMessage(msg)
}

func (d *IotxDispatcher) updateEventAudit(t iotexrpc.MessageType) {
	d.eventAuditLock.Lock()
	defer d.eventAuditLock.Unlock()
	d.eventAudit[t]++
}

func (d *IotxDispatcher) updateMetrics(msg *message, queue chan *message) {
	d.updateEventAudit(msg.msgType)
	subscriber := d.subscriber(msg.chainID)
	if subscriber != nil {
		subscriber.ReportFullness(msg.ctx, msg.msgType, float32(len(queue))/float32(cap(queue)))
	}
}

func (d *IotxDispatcher) filter(msg *message) bool {
	if msg.msgType != iotexrpc.MessageType_BLOCK_REQUEST {
		return true
	}
	// filter block sync to avoid too frequent sync
	now := time.Now()
	peerID := msg.peer
	d.peerSyncLock.Lock()
	defer d.peerSyncLock.Unlock()
	last, ok := d.peerLastSync[peerID]
	if ok && last.Add(d.syncInterval).After(now) {
		return false
	}
	d.peerLastSync[peerID] = now
	return true
}

func (d *IotxDispatcher) dispatchMsg(message *message) {
	subscriber := d.subscriber(message.chainID)
	if subscriber == nil {
		log.L().Warn("chainID has not been registered in dispatcher.", zap.Uint32("chainID", message.chainID))
		return
	}
	switch msg := message.msg.(type) {
	case *iotextypes.ConsensusMessage:
		if err := subscriber.HandleConsensusMsg(msg); err != nil {
			log.L().Warn("Failed to handle consensus message.", zap.Error(err))
		}
	case *iotextypes.Action:
		if err := subscriber.HandleAction(message.ctx, msg); err != nil {
			requestMtc.WithLabelValues("AddAction", "false").Inc()
			log.L().Warn("Handle action request error.", zap.Error(err))
		}
	case *iotextypes.Actions:
		for i := range msg.Actions {
			if err := subscriber.HandleAction(message.ctx, msg.Actions[i]); err != nil {
				requestMtc.WithLabelValues("AddAction", "false").Inc()
				log.L().Warn("Handle action request error.", zap.Error(err))
			}
		}
	case *iotextypes.Block:
		if err := subscriber.HandleBlock(message.ctx, message.peer, msg); err != nil {
			log.L().Error("Fail to handle the block.", zap.Error(err))
		}
	case *iotextypes.NodeInfo:
		if err := subscriber.HandleNodeInfo(message.ctx, message.peer, msg); err != nil {
			log.L().Warn("Failed to handle node info message.", zap.Error(err))
		}
	case *iotexrpc.BlockSync:
		if message.peerInfo == nil {
			log.L().Warn("BlockSync message must be unicast.")
			return
		}
		if err := subscriber.HandleSyncRequest(message.ctx, *message.peerInfo, msg); err != nil {
			log.L().Debug("Failed to handle consensus message.", zap.Error(err))
		}
	case *iotextypes.NodeInfoRequest:
		if message.peerInfo == nil {
			log.L().Warn("NodeInfoRequest message must be unicast.")
			return
		}
		if err := subscriber.HandleNodeInfoRequest(message.ctx, *message.peerInfo, msg); err != nil {
			log.L().Warn("Failed to handle node info request message.", zap.Error(err))
		}
	case *iotextypes.ActionHash:
		if err := subscriber.HandleActionHash(message.ctx, hash.BytesToHash256(msg.Hash), message.peer); err != nil {
			log.L().Warn("Failed to handle action hash message.", zap.Error(err))
		}
	case *iotexrpc.ActionSync:
		if message.peerInfo == nil {
			log.L().Warn("ActionSync message must be unicast.")
			return
		}
		for _, h := range msg.Hashes {
			if err := subscriber.HandleActionRequest(message.ctx, *message.peerInfo, hash.BytesToHash256(h)); err != nil {
				log.L().Warn("Failed to handle action sync message.", zap.Error(err))
			}
		}
	default:
		msgType, _ := goproto.GetTypeFromRPCMsg(message.msg)
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
	}
}
