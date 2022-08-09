// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/go-p2p"
	"github.com/iotexproject/go-pkgs/hash"
	goproto "github.com/iotexproject/iotex-proto/golang"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/pkg/tracer"
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
	_broadcastTopic    = "broadcast"
	_unicastTopic      = "unicast"
	_numDialRetries    = 8
	_dialRetryInterval = 2 * time.Second
)

type (
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
	}

	// Agent is the agent to help the blockchain node connect into the P2P networks and send/receive messages
	Agent interface {
		lifecycle.StartStopper
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
		broadcastInboundHandler    HandleBroadcastInbound
		unicastInboundAsyncHandler HandleUnicastInboundAsync
		host                       *p2p.Host
		bootNodeAddr               []multiaddr.Multiaddr
		reconnectTimeout           time.Duration
		reconnectTask              *routine.RecurringTask
		qosMetrics                 *Qos
	}
)

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

// NewAgent instantiates a local P2P agent instance
func NewAgent(cfg Config, chainID uint32, genesisHash hash.Hash256, broadcastHandler HandleBroadcastInbound, unicastHandler HandleUnicastInboundAsync) Agent {
	log.L().Info("p2p agent", log.Hex("topicSuffix", genesisHash[22:]))
	return &agent{
		cfg:     cfg,
		chainID: chainID,
		// Make sure the honest node only care the messages related the chain from the same genesis
		topicSuffix:                hex.EncodeToString(genesisHash[22:]), // last 10 bytes of genesis hash
		broadcastInboundHandler:    broadcastHandler,
		unicastInboundAsyncHandler: unicastHandler,
		reconnectTimeout:           cfg.ReconnectInterval,
		qosMetrics:                 NewQoS(time.Now(), 2*cfg.ReconnectInterval),
	}
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

	if err := host.AddBroadcastPubSub(ctx, _broadcastTopic+p.topicSuffix, func(ctx context.Context, data []byte) (err error) {
		// Blocking handling the broadcast message until the agent is started
		<-ready
		var (
			peerID    string
			broadcast iotexrpc.BroadcastMsg
			latency   int64
		)
		skip := false
		defer func() {
			// Skip accounting if the broadcast message is not handled
			if skip {
				return
			}
			status := _successStr
			if err != nil {
				status = _failureStr
			}
			_p2pMsgCounter.WithLabelValues("broadcast", strconv.Itoa(int(broadcast.MsgType)), "in", peerID, status).Inc()
			_p2pMsgLatency.WithLabelValues("broadcast", strconv.Itoa(int(broadcast.MsgType)), status).Observe(float64(latency))
		}()
		if err = proto.Unmarshal(data, &broadcast); err != nil {
			err = errors.Wrap(err, "error when marshaling broadcast message")
			return
		}
		// Skip the broadcast message if it's from the node itself
		rawmsg, ok := p2p.GetBroadcastMsg(ctx)
		if !ok {
			err = errors.New("error when asserting broadcast msg context")
			return
		}
		peerID = rawmsg.GetFrom().Pretty()
		if p.host.HostIdentity() == peerID {
			skip = true
			return
		}
		if broadcast.ChainId != p.chainID {
			err = errors.Errorf("chain ID mismatch, received %d, expecting %d", broadcast.ChainId, p.chainID)
			return
		}

		t := broadcast.GetTimestamp().AsTime()
		latency = time.Since(t).Nanoseconds() / time.Millisecond.Nanoseconds()

		msg, err := goproto.TypifyRPCMsg(broadcast.MsgType, broadcast.MsgBody)
		if err != nil {
			err = errors.Wrap(err, "error when typifying broadcast message")
			return
		}
		p.broadcastInboundHandler(ctx, broadcast.ChainId, peerID, msg)
		p.qosMetrics.updateRecvBroadcast(time.Now())
		return
	}); err != nil {
		return errors.Wrap(err, "error when adding broadcast pubsub")
	}

	if err := host.AddUnicastPubSub(_unicastTopic+p.topicSuffix, func(ctx context.Context, peerInfo peer.AddrInfo, data []byte) (err error) {
		// Blocking handling the unicast message until the agent is started
		<-ready
		var (
			unicast iotexrpc.UnicastMsg
			peerID  = peerInfo.ID.Pretty()
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
	if err = host.Broadcast(ctx, _broadcastTopic+p.topicSuffix, data); err != nil {
		err = errors.Wrap(err, "error when sending broadcast message")
		p.qosMetrics.updateSendBroadcast(t, false)
		return
	}
	p.qosMetrics.updateSendBroadcast(t, true)
	return
}

func (p *agent) UnicastOutbound(ctx context.Context, peer peer.AddrInfo, msg proto.Message) (err error) {
	host := p.host
	if host == nil {
		return ErrAgentNotStarted
	}
	var (
		peerName = peer.ID.Pretty()
		msgType  iotexrpc.MessageType
		msgBody  []byte
	)
	defer func() {
		status := _successStr
		if err != nil {
			status = _failureStr
		}
		_p2pMsgCounter.WithLabelValues("unicast", strconv.Itoa(int(msgType)), "out", peer.ID.Pretty(), status).Inc()
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
