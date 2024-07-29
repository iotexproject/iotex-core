package dispatcher

import (
	"context"
	"sync"
	"time"

	goproto "github.com/iotexproject/iotex-proto/golang"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	dispatcherV2 struct {
		lifecycle.Readiness
		lifecycle.Lifecycle
		// queue manager
		queueMgr *msgQueueMgr
		// subscribers
		subscribers   map[uint32]Subscriber
		subscribersMU sync.RWMutex
		// filter for blocksync message
		peerLastSync map[string]time.Time
		peerSyncLock sync.RWMutex
		syncInterval time.Duration
	}

	message struct {
		ctx         context.Context
		chainID     uint32
		msg         proto.Message
		msgType     iotexrpc.MessageType
		isBroadcast bool
		peerInfo    peer.AddrInfo // peerInfo is only used for unicast message
		peer        string        // peer is only used for broadcast message
	}
)

// NewDispatcherV2 creates a new dispatcherV2
func NewDispatcherV2(cfg Config) Dispatcher {
	d := &dispatcherV2{
		subscribers:  make(map[uint32]Subscriber),
		peerLastSync: make(map[string]time.Time),
		syncInterval: cfg.ProcessSyncRequestInterval,
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
		if msg.isBroadcast {
			d.dispatchBroadcastMsg(msg)
		} else {
			d.dispatchUnicastMsg(msg)
		}
	})
	d.queueMgr = queueMgr
	d.Lifecycle.Add(d.queueMgr)
	return d
}

func (d *dispatcherV2) Start(ctx context.Context) error {
	log.L().Info("Starting dispatcherV2.")
	if err := d.OnStart(ctx); err != nil {
		return err
	}
	return d.TurnOn()
}

func (d *dispatcherV2) Stop(ctx context.Context) error {
	if err := d.TurnOff(); err != nil {
		log.L().Warn("DispatcherV2 already in the process of shutting down.")
		return err
	}
	log.L().Info("DispatcherV2 is shutting down.")
	return d.OnStop(ctx)
}

func (d *dispatcherV2) queueForMsg(msg *message) msgQueue {
	return d.queueMgr.Queue(msg)
}

func (d *dispatcherV2) filter(msg *message) bool {
	if msg.msgType != iotexrpc.MessageType_BLOCK_REQUEST {
		return true
	}
	// filter block sync to avoid too frequent sync
	now := time.Now()
	peerID := msg.peerInfo.ID.Pretty()
	d.peerSyncLock.Lock()
	defer d.peerSyncLock.Unlock()
	last, ok := d.peerLastSync[peerID]
	if ok && last.Add(d.syncInterval).After(now) {
		return false
	}
	d.peerLastSync[peerID] = now
	return true
}

func (d *dispatcherV2) HandleBroadcast(ctx context.Context, chainID uint32, peer string, msgProto proto.Message) {
	msgType, err := goproto.GetTypeFromRPCMsg(msgProto)
	if err != nil {
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
		return
	}
	msg := &message{
		ctx:         ctx,
		chainID:     chainID,
		msg:         msgProto,
		peer:        peer,
		msgType:     msgType,
		isBroadcast: true,
	}
	queue := d.queueForMsg(msg)
	select {
	case queue <- msg:
	default:
		log.L().Warn("Broadcast queue is full.", zap.Any("msgType", msgType))
	}

	d.updateMetrics(msg, queue)
}

func (d *dispatcherV2) HandleTell(ctx context.Context, chainID uint32, peer peer.AddrInfo, msgProto proto.Message) {
	msgType, err := goproto.GetTypeFromRPCMsg(msgProto)
	if err != nil {
		log.L().Warn("Unexpected message handled by HandleTell.", zap.Error(err))
	}
	msg := &message{
		ctx:      ctx,
		chainID:  chainID,
		msg:      msgProto,
		peerInfo: peer,
		msgType:  msgType,
	}
	queue := d.queueForMsg(msg)
	select {
	case queue <- msg:
	default:
		log.L().Warn("Unicast queue is full.", zap.Any("msgType", msgType))
	}

	d.updateMetrics(msg, queue)
}

func (d *dispatcherV2) AddSubscriber(chainID uint32, subscriber Subscriber) {
	d.subscribersMU.Lock()
	d.subscribers[chainID] = subscriber
	d.subscribersMU.Unlock()
}

func (d *dispatcherV2) updateMetrics(msg *message, queue chan *message) {
	subscriber := d.subscriber(msg.chainID)
	if subscriber != nil {
		subscriber.ReportFullness(msg.ctx, msg.msgType, float32(len(queue))/float32(cap(queue)))
	}
}

func (d *dispatcherV2) dispatchBroadcastMsg(message *message) {
	if !d.IsReady() {
		return
	}
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
	default:
		msgType, _ := goproto.GetTypeFromRPCMsg(message.msg)
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
	}
}

func (d *dispatcherV2) dispatchUnicastMsg(message *message) {
	if !d.IsReady() {
		return
	}
	subscriber := d.subscriber(message.chainID)
	if subscriber == nil {
		log.L().Warn("chainID has not been registered in dispatcher.", zap.Uint32("chainID", message.chainID))
		return
	}
	switch msg := message.msg.(type) {
	case *iotexrpc.BlockSync:
		if err := subscriber.HandleSyncRequest(message.ctx, message.peerInfo, msg); err != nil {
			log.L().Debug("Failed to handle consensus message.", zap.Error(err))
		}
	case *iotextypes.Block:
		if err := subscriber.HandleBlock(message.ctx, message.peerInfo.ID.String(), msg); err != nil {
			log.L().Error("Fail to handle the block.", zap.Error(err))
		}
	case *iotextypes.NodeInfo:
		if err := subscriber.HandleNodeInfo(message.ctx, message.peerInfo.ID.String(), msg); err != nil {
			log.L().Warn("Failed to handle node info message.", zap.Error(err))
		}
	case *iotextypes.NodeInfoRequest:
		if err := subscriber.HandleNodeInfoRequest(message.ctx, message.peerInfo, msg); err != nil {
			log.L().Warn("Failed to handle node info request message.", zap.Error(err))
		}
	default:
		msgType, _ := goproto.GetTypeFromRPCMsg(message.msg)
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
	}
}

func (d *dispatcherV2) subscriber(chainID uint32) Subscriber {
	d.subscribersMU.RLock()
	defer d.subscribersMU.RUnlock()
	subscriber, ok := d.subscribers[chainID]
	if !ok {
		return nil
	}
	return subscriber
}
