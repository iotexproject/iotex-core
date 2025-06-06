// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/go-p2p"
	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	goproto "github.com/iotexproject/iotex-proto/golang"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/routine"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
	"github.com/iotexproject/iotex-core/v2/server/itx/nodestats"
)

const (
	_successStr = "success"
	_failureStr = "failure"
)

var (
	_p2pMsgCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_p2p_message_counter",
			Help: "P2P message stats",
		},
		[]string{"protocol", "message", "direction", "peer", "status"},
	)
	_p2pMsgLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "iotex_p2p_message_latency",
			Help:    "message latency",
			Buckets: prometheus.LinearBuckets(0, 10, 200),
		},
		[]string{"protocol", "message", "status"},
	)
	// ErrAgentNotStarted is the error returned when p2p agent has not been started
	ErrAgentNotStarted = errors.New("p2p agent has not been started")
)

func init() {
	prometheus.MustRegister(_p2pMsgCounter)
	prometheus.MustRegister(_p2pMsgLatency)
}

const (
	// TODO: the topic could be fine tuned
	_broadcastTopic             = "broadcast"
	_broadcastSubTopicConsensus = "_consensus"
	_broadcastSubTopicAction    = "_action"
	_unicastTopic               = "unicast"
	_numDialRetries             = 8
	_dialRetryInterval          = 2 * time.Second
)

type (
	// ValidateBroadcastInbound validates the inbound broadcast message
	ValidateBroadcastInbound func(proto.Message) (bool, error)

	// HandleBroadcastInbound handles broadcast message when agent listens it from the network
	HandleBroadcastInbound func(context.Context, uint32, string, proto.Message)

	// HandleUnicastInboundAsync handles unicast message when agent listens it from the network
	HandleUnicastInboundAsync func(context.Context, uint32, peer.AddrInfo, proto.Message)

	// Config is the config of p2p
	Config struct {
		Host           string   `yaml:"host"`
		Port           int      `yaml:"port"`
		ExternalHost   string   `yaml:"externalHost"`
		ExternalPort   int      `yaml:"externalPort"`
		BootstrapNodes []string `yaml:"bootstrapNodes"`
		MasterKey      string   `yaml:"masterKey"` // master key will be PrivateKey if not set.
		// RelayType is the type of P2P network relay. By default, the value is empty, meaning disabled. Two relay types
		// are supported: active, nat.
		RelayType         string              `yaml:"relayType"`
		ReconnectInterval time.Duration       `yaml:"reconnectInterval"`
		RateLimit         p2p.RateLimitConfig `yaml:"rateLimit"`
		EnableRateLimit   bool                `yaml:"enableRateLimit"`
		PrivateNetworkPSK string              `yaml:"privateNetworkPSK"`
		MaxPeers          int                 `yaml:"maxPeers"`
		MaxMessageSize    int                 `yaml:"maxMessageSize"`
		RpcMsgCacheSize   int                 `yaml:"rpcMsgCacheSize"`
		RpcDedupCacheSize int                 `yaml:"rpcDedupCacheSize"`
	}

	// Agent is the agent to help the blockchain node connect into the P2P networks and send/receive messages
	Agent interface {
		lifecycle.StartStopper
		nodestats.StatsReporter
		// BroadcastOutbound sends a broadcast message to the whole network
		BroadcastOutbound(ctx context.Context, msg proto.Message) (err error)
		// UnicastOutbound sends a unicast message to the given address
		UnicastOutbound(_ context.Context, peer peer.AddrInfo, msg proto.Message) (err error)
		// Info returns agents' peer info.
		Info() (peer.AddrInfo, error)
		// Self returns the self network address
		Self() ([]multiaddr.Multiaddr, error)
		// ConnectedPeers returns the connected peers' info
		ConnectedPeers() ([]peer.AddrInfo, error)
		// BlockPeer blocks the peer in p2p layer
		BlockPeer(string)
	}

	dummyAgent struct{}

	agent struct {
		cfg                        Config
		chainID                    uint32
		topicSuffix                string
		validatorBroadcastInbound  ValidateBroadcastInbound
		caches                     cache.LRUCache
		dedup                      cache.LRUCache
		broadcastInboundHandler    HandleBroadcastInbound
		unicastInboundAsyncHandler HandleUnicastInboundAsync
		host                       *p2p.Host
		bootNodeAddr               []multiaddr.Multiaddr
		reconnectTimeout           time.Duration
		reconnectTask              *routine.RecurringTask
		qosMetrics                 *Qos
		unifiedTopic               atomic.Bool
		isUnifiedTopic             func(height uint64) bool
	}

	cacheValue struct {
		msgType   iotexrpc.MessageType
		message   proto.Message
		peerID    string
		timestamp time.Time
	}

	Option func(*agent)
)

var WithUnifiedTopicHelper = func(isUnifiedTopic func(height uint64) bool) Option {
	return func(a *agent) {
		a.isUnifiedTopic = isUnifiedTopic
	}
}

// DefaultConfig is the default config of p2p
var DefaultConfig = Config{
	Host:              "0.0.0.0",
	Port:              4689,
	ExternalHost:      "",
	ExternalPort:      4689,
	BootstrapNodes:    []string{},
	MasterKey:         "",
	RateLimit:         p2p.DefaultRatelimitConfig,
	ReconnectInterval: 150 * time.Second,
	EnableRateLimit:   true,
	PrivateNetworkPSK: "",
	MaxPeers:          30,
	MaxMessageSize:    p2p.DefaultConfig.MaxMessageSize,
	RpcMsgCacheSize:   10000,
	RpcDedupCacheSize: 40000,
}

// NewDummyAgent creates a dummy p2p agent
func NewDummyAgent() Agent {
	return &dummyAgent{}
}

func (*dummyAgent) Start(context.Context) error {
	return nil
}

func (*dummyAgent) Stop(context.Context) error {
	return nil
}

func (*dummyAgent) BroadcastOutbound(ctx context.Context, msg proto.Message) error {
	return nil
}

func (*dummyAgent) UnicastOutbound(_ context.Context, peer peer.AddrInfo, msg proto.Message) error {
	return nil
}

func (*dummyAgent) Info() (peer.AddrInfo, error) {
	return peer.AddrInfo{}, nil
}

func (*dummyAgent) Self() ([]multiaddr.Multiaddr, error) {
	return nil, nil
}

func (*dummyAgent) ConnectedPeers() ([]peer.AddrInfo, error) {
	return nil, nil
}

func (*dummyAgent) BlockPeer(string) {
	return
}

func (*dummyAgent) BuildReport() string {
	return ""
}

// NewAgent instantiates a local P2P agent instance
func NewAgent(
	cfg Config,
	chainID uint32,
	genesisHash hash.Hash256,
	validateBroadcastInbound ValidateBroadcastInbound,
	broadcastHandler HandleBroadcastInbound,
	unicastHandler HandleUnicastInboundAsync,
	opts ...Option,
) Agent {
	log.L().Info("p2p agent", log.Hex("topicSuffix", genesisHash[22:]))
	a := &agent{
		cfg:     cfg,
		chainID: chainID,
		// Make sure the honest node only care the messages related the chain from the same genesis
		topicSuffix:               hex.EncodeToString(genesisHash[22:]), // last 10 bytes of genesis hash
		validatorBroadcastInbound: validateBroadcastInbound,
		// TODO: make the cache size configurable
		caches:                     cache.NewThreadSafeLruCache(cfg.RpcMsgCacheSize),
		dedup:                      cache.NewThreadSafeLruCache(cfg.RpcDedupCacheSize),
		broadcastInboundHandler:    broadcastHandler,
		unicastInboundAsyncHandler: unicastHandler,
		reconnectTimeout:           cfg.ReconnectInterval,
		qosMetrics:                 NewQoS(time.Now(), 2*cfg.ReconnectInterval),
	}
	a.unifiedTopic.Store(true)
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (p *agent) duplicateActions(msg *iotexrpc.BroadcastMsg) bool {
	if !(msg.MsgType == iotexrpc.MessageType_ACTION || msg.MsgType == iotexrpc.MessageType_ACTIONS) {
		// for MessageType_ACTION and MessageType_ACTIONS:
		// duplicates will either be discarded by actpool, or constitute flooding attack, so stop forwarding
		// for other types of RPC message, could add specific policies later if needed
		return false
	}
	hash := hash.Hash160b(msg.MsgBody)
	if _, ok := p.dedup.Get(hash); ok {
		return true
	}
	p.dedup.Add(hash, 1)
	return false
}

func (p *agent) Start(ctx context.Context) error {
	ready := make(chan interface{})
	p2p.SetLogger(log.L())
	opts := []p2p.Option{
		p2p.HostName(p.cfg.Host),
		p2p.Port(p.cfg.Port),
		p2p.Gossip(),
		p2p.SecureIO(),
		p2p.MasterKey(p.cfg.MasterKey),
		p2p.PrivateNetworkPSK(p.cfg.PrivateNetworkPSK),
		p2p.DHTProtocolID(p.chainID),
		p2p.DHTGroupID(p.chainID),
		p2p.WithMaxPeer(uint32(p.cfg.MaxPeers)),
		p2p.WithMaxMessageSize(p.cfg.MaxMessageSize),
	}
	if p.cfg.EnableRateLimit {
		opts = append(opts, p2p.WithRateLimit(p.cfg.RateLimit))
	}
	if p.cfg.ExternalHost != "" {
		opts = append(opts, p2p.ExternalHostName(p.cfg.ExternalHost))
		opts = append(opts, p2p.ExternalPort(p.cfg.ExternalPort))
	}
	if p.cfg.RelayType != "" {
		opts = append(opts, p2p.WithRelay(p.cfg.RelayType))
	}
	host, err := p2p.NewHost(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "error when instantiating Agent host")
	}

	broadcastValidator := func(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		if pid.String() == host.HostIdentity() {
			return pubsub.ValidationAccept
		}
		var broadcast iotexrpc.BroadcastMsg
		if err := proto.Unmarshal(msg.Data, &broadcast); err != nil {
			log.L().Debug("error when unmarshaling broadcast message", zap.Error(err))
			return pubsub.ValidationReject
		}
		if broadcast.ChainId != p.chainID {
			log.L().Debug("chain ID mismatch", zap.Uint32("received", broadcast.ChainId), zap.Uint32("expecting", p.chainID))
			return pubsub.ValidationReject
		}
		pMsg, err := goproto.TypifyRPCMsg(broadcast.MsgType, broadcast.MsgBody)
		if err != nil {
			log.L().Debug("error when typifying broadcast message", zap.Error(err))
			return pubsub.ValidationReject
		}
		// dedup message
		if p.duplicateActions(&broadcast) {
			log.L().Debug("duplicate msg", zap.Int("type", int(broadcast.MsgType)))
			return pubsub.ValidationIgnore
		}
		ignore, err := p.validatorBroadcastInbound(pMsg)
		if err != nil {
			log.L().Debug("error when validating broadcast message", zap.Error(err))
			return pubsub.ValidationReject
		}
		if ignore {
			log.L().Debug("invalid broadcast message")
			return pubsub.ValidationIgnore
		}
		p.caches.Add(hash.Hash256b(msg.Data), &cacheValue{
			msgType:   broadcast.MsgType,
			message:   pMsg,
			peerID:    pid.String(),
			timestamp: time.Now(),
		})
		return pubsub.ValidationAccept
	}
	broadcastHandler := func(ctx context.Context, pid peer.ID, data []byte) (err error) {
		// Blocking handling the broadcast message until the agent is started
		<-ready
		if pid.String() == host.HostIdentity() {
			return nil
		}
		var (
			peerID  string
			pMsg    proto.Message
			msgType iotexrpc.MessageType
			latency int64
		)
		defer func() {
			status := _successStr
			if err != nil {
				status = _failureStr
			}
			_p2pMsgCounter.WithLabelValues("broadcast", strconv.Itoa(int(msgType)), "in", peerID, status).Inc()
			_p2pMsgLatency.WithLabelValues("broadcast", strconv.Itoa(int(msgType)), status).Observe(float64(latency))
		}()
		// Check the cache first
		cache, ok := p.caches.Get(hash.Hash256b(data))
		if ok {
			value := cache.(*cacheValue)
			pMsg = value.message
			peerID = value.peerID
			msgType = value.msgType
			latency = time.Since(value.timestamp).Nanoseconds() / time.Millisecond.Nanoseconds()
		} else {
			var broadcast iotexrpc.BroadcastMsg
			if err = proto.Unmarshal(data, &broadcast); err != nil {
				// TODO: unexpected error
				err = errors.Wrap(err, "error when marshaling broadcast message")
				return
			}
			t := broadcast.GetTimestamp().AsTime()
			latency = time.Since(t).Nanoseconds() / time.Millisecond.Nanoseconds()
			pMsg, err = goproto.TypifyRPCMsg(broadcast.MsgType, broadcast.MsgBody)
			if err != nil {
				err = errors.Wrap(err, "error when typifying broadcast message")
				return
			}
			peerID = broadcast.PeerId
			msgType = broadcast.MsgType
		}
		// TODO: skip signature verification for actions
		p.broadcastInboundHandler(ctx, p.chainID, peerID, pMsg)
		p.qosMetrics.updateRecvBroadcast(time.Now())
		return
	}

	// default topics
	if err := host.AddBroadcastPubSub(
		ctx,
		_broadcastTopic+p.topicSuffix,
		broadcastValidator, broadcastHandler); err != nil {
		return errors.Wrap(err, "error when adding broadcast pubsub")
	}
	// add consensus topic
	if err := host.AddBroadcastPubSub(
		ctx,
		_broadcastTopic+_broadcastSubTopicConsensus+p.topicSuffix,
		nil, broadcastHandler); err != nil {
		return errors.Wrap(err, "error when adding broadcast pubsub")
	}
	// add action topic
	if err := host.AddBroadcastPubSub(
		ctx,
		_broadcastTopic+_broadcastSubTopicAction+p.topicSuffix,
		broadcastValidator, broadcastHandler); err != nil {
		return errors.Wrap(err, "error when adding broadcast pubsub")
	}
	if err := host.AddUnicastPubSub(_unicastTopic+p.topicSuffix, func(ctx context.Context, peerInfo peer.AddrInfo, data []byte) (err error) {
		// Blocking handling the unicast message until the agent is started
		<-ready
		var (
			unicast iotexrpc.UnicastMsg
			peerID  = peerInfo.ID.String()
			latency int64
		)
		defer func() {
			status := _successStr
			if err != nil {
				status = _failureStr
			}
			_p2pMsgCounter.WithLabelValues("unicast", strconv.Itoa(int(unicast.MsgType)), "in", peerID, status).Inc()
			_p2pMsgLatency.WithLabelValues("unicast", strconv.Itoa(int(unicast.MsgType)), status).Observe(float64(latency))
		}()
		if err = proto.Unmarshal(data, &unicast); err != nil {
			err = errors.Wrap(err, "error when marshaling unicast message")
			return
		}
		msg, err := goproto.TypifyRPCMsg(unicast.MsgType, unicast.MsgBody)
		if err != nil {
			err = errors.Wrap(err, "error when typifying unicast message")
			return
		}
		if unicast.ChainId != p.chainID {
			err = errors.Errorf("chain ID mismatch, received %d, expecting %d", unicast.ChainId, p.chainID)
			return
		}

		t := unicast.GetTimestamp().AsTime()
		latency = time.Since(t).Nanoseconds() / time.Millisecond.Nanoseconds()

		p.unicastInboundAsyncHandler(ctx, unicast.ChainId, peerInfo, msg)
		p.qosMetrics.updateRecvUnicast(peerID, time.Now())
		return
	}); err != nil {
		return errors.Wrap(err, "error when adding unicast pubsub")
	}

	// create boot nodes list except itself
	hostName := host.HostIdentity()
	for _, bootstrapNode := range p.cfg.BootstrapNodes {
		bootAddr := multiaddr.StringCast(bootstrapNode)
		if !strings.Contains(bootAddr.String(), hostName) {
			p.bootNodeAddr = append(p.bootNodeAddr, bootAddr)
		}
	}
	if err := host.AddBootstrap(p.bootNodeAddr); err != nil {
		return err
	}
	host.JoinOverlay()
	p.host = host

	// connect to bootstrap nodes
	if err := p.connectBootNode(ctx); err != nil {
		log.L().Error("fail to connect bootnode", zap.Error(err))
		return err
	}
	if err := p.host.AdvertiseAsync(); err != nil {
		return err
	}
	if err := p.host.FindPeersAsync(); err != nil {
		return err
	}

	close(ready)

	// check network connectivity every 60 blocks, and reconnect in case of disconnection
	p.reconnectTask = routine.NewRecurringTask(p.reconnect, p.reconnectTimeout)
	return p.reconnectTask.Start(ctx)
}

func (p *agent) Stop(ctx context.Context) error {
	if p.host == nil {
		return ErrAgentNotStarted
	}
	log.L().Info("p2p is shutting down.", zap.Error(ctx.Err()))
	if err := p.reconnectTask.Stop(ctx); err != nil {
		return err
	}
	if err := p.host.Close(); err != nil {
		return errors.Wrap(err, "error when closing Agent host")
	}
	return nil
}

func (p *agent) BroadcastOutbound(ctx context.Context, msg proto.Message) (err error) {
	_, span := tracer.NewSpan(ctx, "Agent.BroadcastOutbound")
	defer span.End()

	host := p.host
	if host == nil {
		return ErrAgentNotStarted
	}
	var msgType iotexrpc.MessageType
	var msgBody []byte
	defer func() {
		status := _successStr
		if err != nil {
			status = _failureStr
		}
		_p2pMsgCounter.WithLabelValues(
			"broadcast",
			strconv.Itoa(int(msgType)),
			"out",
			host.HostIdentity(),
			status,
		).Inc()
	}()
	msgType, msgBody, err = convertAppMsg(msg)
	if err != nil {
		return
	}
	broadcast := iotexrpc.BroadcastMsg{
		ChainId:   p.chainID,
		PeerId:    host.HostIdentity(),
		MsgType:   msgType,
		MsgBody:   msgBody,
		Timestamp: timestamppb.Now(),
	}
	data, err := proto.Marshal(&broadcast)
	if err != nil {
		err = errors.Wrap(err, "error when marshaling broadcast message")
		return
	}
	t := time.Now()
	topic := p.messageTopic(msgType)
	if err = host.Broadcast(ctx, topic, data); err != nil {
		err = errors.Wrapf(err, "error when sending broadcast message to topic %s", topic)
		p.qosMetrics.updateSendBroadcast(t, false)
		return
	}
	p.qosMetrics.updateSendBroadcast(t, true)
	return
}

func (p *agent) messageTopic(msgType iotexrpc.MessageType) string {
	defaultTopic := _broadcastTopic + p.topicSuffix
	switch msgType {
	case iotexrpc.MessageType_ACTION, iotexrpc.MessageType_ACTIONS:
		// TODO: can be removed after wake activated
		if p.unifiedTopic.Load() {
			return defaultTopic
		}
		return _broadcastTopic + _broadcastSubTopicAction + p.topicSuffix
	case iotexrpc.MessageType_CONSENSUS:
		// TODO: can be removed after wake activated
		if p.unifiedTopic.Load() {
			return defaultTopic
		}
		return _broadcastTopic + _broadcastSubTopicConsensus + p.topicSuffix
	default:
		return defaultTopic
	}
}

func (p *agent) UnicastOutbound(ctx context.Context, peer peer.AddrInfo, msg proto.Message) (err error) {
	host := p.host
	if host == nil {
		return ErrAgentNotStarted
	}
	var (
		peerName = peer.ID.String()
		msgType  iotexrpc.MessageType
		msgBody  []byte
	)
	defer func() {
		status := _successStr
		if err != nil {
			status = _failureStr
		}
		_p2pMsgCounter.WithLabelValues("unicast", strconv.Itoa(int(msgType)), "out", peer.ID.String(), status).Inc()
	}()

	msgType, msgBody, err = convertAppMsg(msg)
	if err != nil {
		return
	}
	unicast := iotexrpc.UnicastMsg{
		ChainId:   p.chainID,
		PeerId:    host.HostIdentity(),
		MsgType:   msgType,
		MsgBody:   msgBody,
		Timestamp: timestamppb.Now(),
	}
	data, err := proto.Marshal(&unicast)
	if err != nil {
		err = errors.Wrap(err, "error when marshaling unicast message")
		return
	}

	t := time.Now()
	if err = host.Unicast(ctx, peer, _unicastTopic+p.topicSuffix, data); err != nil {
		err = errors.Wrap(err, "error when sending unicast message")
		p.qosMetrics.updateSendUnicast(peerName, t, false)
		return
	}
	p.qosMetrics.updateSendUnicast(peerName, t, true)
	return
}

func (p *agent) Info() (peer.AddrInfo, error) {
	if p.host == nil {
		return peer.AddrInfo{}, ErrAgentNotStarted
	}
	return p.host.Info(), nil
}

func (p *agent) Self() ([]multiaddr.Multiaddr, error) {
	if p.host == nil {
		return nil, ErrAgentNotStarted
	}
	return p.host.Addresses(), nil
}

func (p *agent) ConnectedPeers() ([]peer.AddrInfo, error) {
	if p.host == nil {
		return nil, ErrAgentNotStarted
	}
	return p.host.ConnectedPeers(), nil
}

func (p *agent) BlockPeer(pidStr string) {
	pid, err := peer.Decode(pidStr)
	if err != nil {
		return
	}
	p.host.BlockPeer(pid)
}

// BuildReport builds a report of p2p agent
func (p *agent) BuildReport() string {
	neighbors, err := p.ConnectedPeers()
	if err == nil {
		return fmt.Sprintf("P2P ConnectedPeers: %d", len(neighbors))
	}
	return ""
}

func (p *agent) ReceiveBlock(blk *block.Block) error {
	if p.isUnifiedTopic == nil {
		return nil
	}
	p.unifiedTopic.Store(p.isUnifiedTopic(blk.Height()))
	return nil
}

func (p *agent) connectBootNode(ctx context.Context) error {
	if len(p.cfg.BootstrapNodes) == 0 {
		return nil
	}
	var errNum, connNum, desiredConnNum int
	conn := make(chan struct{}, len(p.cfg.BootstrapNodes))
	connErrChan := make(chan error, len(p.cfg.BootstrapNodes))

	// try to connect to all bootstrap node beside itself.
	for i := range p.bootNodeAddr {
		bootAddr := p.bootNodeAddr[i]
		go func() {
			if err := exponentialRetry(
				func() error { return p.host.ConnectWithMultiaddr(ctx, bootAddr) },
				_dialRetryInterval,
				_numDialRetries,
			); err != nil {
				err := errors.Wrap(err, fmt.Sprintf("error when connecting bootstrap node %s", bootAddr.String()))
				connErrChan <- err
				return
			}
			conn <- struct{}{}
			log.L().Info("Connected bootstrap node.", zap.String("address", bootAddr.String()))
		}()
	}

	// wait until half+1 bootnodes get connected
	desiredConnNum = len(p.bootNodeAddr)/2 + 1
	for {
		select {
		case err := <-connErrChan:
			log.L().Info("Connection failed.", zap.Error(err))
			errNum++
			if errNum == len(p.bootNodeAddr) {
				return errors.New("failed to connect to any bootstrap node")
			}
		case <-conn:
			connNum++
		}
		// can add more condition later
		if connNum >= desiredConnNum {
			break
		}
	}
	return nil
}

func (p *agent) reconnect() {
	if p.host == nil {
		return
	}
	if len(p.host.ConnectedPeers()) == 0 || p.qosMetrics.lostConnection() {
		log.L().Info("network lost, try re-connecting.")
		p.host.ClearBlocklist()
		if err := p.connectBootNode(context.Background()); err != nil {
			log.L().Error("fail to connect bootnode", zap.Error(err))
			return
		}
		if err := p.host.AdvertiseAsync(); err != nil {
			log.L().Error("fail to advertise", zap.Error(err))
			return
		}
	}
	if err := p.host.FindPeersAsync(); err != nil {
		log.L().Error("fail to find peer", zap.Error(err))
	}
}

func convertAppMsg(msg proto.Message) (iotexrpc.MessageType, []byte, error) {
	msgType, err := goproto.GetTypeFromRPCMsg(msg)
	if err != nil {
		return 0, nil, errors.Wrap(err, "error when converting application message to proto")
	}
	msgBody, err := proto.Marshal(msg)
	if err != nil {
		return 0, nil, errors.Wrap(err, "error when marshaling application message")
	}
	return msgType, msgBody, nil
}

func exponentialRetry(f func() error, retryInterval time.Duration, numRetries int) (err error) {
	for i := 0; i < numRetries; i++ {
		if err = f(); err == nil {
			return
		}
		log.L().Error("Error happens, will retry.", zap.Error(err))
		time.Sleep(retryInterval)
		retryInterval *= 2
	}
	return
}
