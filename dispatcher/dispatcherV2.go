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

	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	dispatcherV2 struct {
		broadcastQueueMap map[iotexrpc.MessageType]chan *broadcastMsg
		unitcastQueueMap  map[iotexrpc.MessageType]chan *unicastMsg

		wg   sync.WaitGroup
		quit chan struct{}

		syncChanLock sync.RWMutex
		peerLastSync map[string]time.Time
		syncInterval time.Duration
	}
	broadcastMsg struct {
		ctx     context.Context
		msg     proto.Message
		chainID uint32
		peer    string
	}
	unicastMsg struct {
		ctx     context.Context
		msg     proto.Message
		chainID uint32
		peer    peer.AddrInfo
	}
)

func (d *dispatcherV2) ConfigureMessage() {
	d.broadcastQueueMap = make(map[iotexrpc.MessageType]chan *broadcastMsg)
	d.unitcastQueueMap = make(map[iotexrpc.MessageType]chan *unicastMsg)

	consensusQueue := make(chan *broadcastMsg, 100)
	actionQueue := make(chan *broadcastMsg, 100)
	blockQueue := make(chan *broadcastMsg, 100)
	actionQueueUnicast := make(chan *unicastMsg, 100)
	d.broadcastQueueMap[iotexrpc.MessageType_BLOCK] = blockQueue
	d.broadcastQueueMap[iotexrpc.MessageType_CONSENSUS] = consensusQueue
	d.broadcastQueueMap[iotexrpc.MessageType_ACTION] = actionQueue

	for i := 0; i < 10; i++ {
		d.wg.Add(1)
		go d.consumeQueue(actionQueue, actionQueueUnicast)
	}
}

func (d *dispatcherV2) consumeQueue(queueBC chan *broadcastMsg, queueUC chan *unicastMsg) {
	defer d.wg.Done()
	for {
		select {
		case msg := <-queueBC:
			d.dispatchBroadcastMsg(msg)
		case msg := <-queueUC:
			d.dispatchUnicastMsg(msg)
		case <-d.quit:
			log.L().Debug("action handler is terminated.")
			return
		}
	}
}

func (d *dispatcherV2) HandleBroadcast(ctx context.Context, chainID uint32, peer string, message proto.Message) {
	msgType, err := goproto.GetTypeFromRPCMsg(message)
	if err != nil {
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
		return
	}
	queue, ok := d.broadcastQueueMap[msgType]
	if !ok {
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
		return
	}
	select {
	case queue <- &broadcastMsg{
		ctx:     ctx,
		chainID: chainID,
		msg:     message,
		peer:    peer,
	}:
	default:
		log.L().Warn("Broadcast queue is full.", zap.Any("msgType", msgType))
	}

	d.updateEventAudit(iotexrpc.MessageType_ACTION)
	subscriber := d.subscriber(chainID)
	if subscriber != nil {
		subscriber.ReportFullness(ctx, msgType, float32(len(queue))/float32(cap(queue)))
	}
}

func (d *dispatcherV2) dispatchBroadcastMsg(message *broadcastMsg) {
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
			log.L().Debug("Failed to handle consensus message.", zap.Error(err))
		}
	case *iotextypes.Action:
		if err := subscriber.HandleAction(message.ctx, msg); err != nil {
			requestMtc.WithLabelValues("AddAction", "false").Inc()
			log.L().Debug("Handle action request error.", zap.Error(err))
		}
	case *iotextypes.Actions:
		for i := range msg.Actions {
			if err := subscriber.HandleAction(message.ctx, msg.Actions[i]); err != nil {
				requestMtc.WithLabelValues("AddAction", "false").Inc()
				log.L().Debug("Handle action request error.", zap.Error(err))
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
	// case *iotextypes.ActionHashes:
	// 	if err := subscriber.HandleActionHash(message.ctx, message.peer, msg); err != nil {
	// 		log.L().Warn("Failed to handle action hash message.", zap.Error(err))
	// 	}
	default:
		msgType, _ := goproto.GetTypeFromRPCMsg(message.msg)
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
	}
}

func (d *dispatcherV2) dispatchUnicastMsg(message *unicastMsg) {
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
		func() {
			now := time.Now()
			peerID := message.peer.ID.Pretty()
			d.syncChanLock.Lock()
			defer d.syncChanLock.Unlock()
			last, ok := d.peerLastSync[peerID]
			if ok && last.Add(d.syncInterval).After(now) {
				return
			}
			d.peerLastSync[peerID] = now
			if err := subscriber.HandleSyncRequest(message.ctx, message.peer, msg); err != nil {
				log.L().Debug("Failed to handle consensus message.", zap.Error(err))
			}
		}()
	case *iotextypes.Block:
		if err := subscriber.HandleBlock(message.ctx, message.peer.ID.String(), msg); err != nil {
			log.L().Error("Fail to handle the block.", zap.Error(err))
		}
	case *iotextypes.NodeInfo:
		if err := subscriber.HandleNodeInfo(message.ctx, message.peer.ID.String(), msg); err != nil {
			log.L().Warn("Failed to handle node info message.", zap.Error(err))
		}
	case *iotextypes.NodeInfoRequest:
		if err := subscriber.HandleNodeInfoRequest(message.ctx, message.peer, msg); err != nil {
			log.L().Warn("Failed to handle node info request message.", zap.Error(err))
		}
	default:
		msgType, _ := goproto.GetTypeFromRPCMsg(message.msg)
		log.L().Warn("Unexpected msgType handled by HandleBroadcast.", zap.Any("msgType", msgType))
	}
}

func (d *dispatcherV2) subscriber(chainID uint32) Subscriber {
	return nil
}

func (d *dispatcherV2) updateEventAudit(t iotexrpc.MessageType) {}

func (r *dispatcherV2) IsReady() bool { return false }
