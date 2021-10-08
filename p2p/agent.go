// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-p2p"
	"github.com/iotexproject/go-pkgs/hash"
	goproto "github.com/iotexproject/iotex-proto/golang"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

const (
	successStr = "success"
	failureStr = "failure"
)

var (
	p2pMsgCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_p2p_message_counter",
			Help: "P2P message stats",
		},
		[]string{"protocol", "message", "direction", "peer", "status"},
	)
	p2pMsgLatency = prometheus.NewHistogramVec(
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
	prometheus.MustRegister(p2pMsgCounter)
	prometheus.MustRegister(p2pMsgLatency)
}

const (
	// TODO: the topic could be fine tuned
	broadcastTopic    = "broadcast"
	unicastTopic      = "unicast"
	numDialRetries    = 8
	dialRetryInterval = 2 * time.Second
)

type (
	// HandleBroadcastInbound handles broadcast message when agent listens it from the network
	HandleBroadcastInbound func(context.Context, uint32, string, proto.Message)

	// HandleUnicastInboundAsync handles unicast message when agent listens it from the network
	HandleUnicastInboundAsync func(context.Context, uint32, peer.AddrInfo, proto.Message)

	// Network is the config of p2p
	Network struct {
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
	}

	// Agent is the agent to help the blockchain node connect into the P2P networks and send/receive messages
	Agent struct {
		ctx                        context.Context
		cfg                        Network
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
var DefaultConfig = Network{
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
}

// NewAgent instantiates a local P2P agent instance
func NewAgent(cfg Network, chainID uint32, genesisHash hash.Hash256, broadcastHandler HandleBroadcastInbound, unicastHandler HandleUnicastInboundAsync) *Agent {
	log.L().Info("p2p agent", log.Hex("topicSuffix", genesisHash[22:]))
	return &Agent{
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

// Start connects into P2P network
func (p *Agent) Start(ctx context.Context) error {
	ready := make(chan interface{})
	p2p.SetLogger(log.L())
	opts := []p2p.Option{
		p2p.HostName(p.cfg.Host),
		p2p.Port(p.cfg.Port),
		p2p.Gossip(),
		p2p.SecureIO(),
		p2p.MasterKey(p.cfg.MasterKey),
		p2p.PrivateNetworkPSK(p.cfg.PrivateNetworkPSK),
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

	p.ctx = ctx
	if err := host.AddBroadcastPubSub(ctx, broadcastTopic+p.topicSuffix, func(ctx context.Context, data []byte) (err error) {
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
			status := successStr
			if err != nil {
				status = failureStr
			}
			p2pMsgCounter.WithLabelValues("broadcast", strconv.Itoa(int(broadcast.MsgType)), "in", peerID, status).Inc()
			p2pMsgLatency.WithLabelValues("broadcast", strconv.Itoa(int(broadcast.MsgType)), status).Observe(float64(latency))
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

		t, _ := ptypes.Timestamp(broadcast.GetTimestamp())
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

	if err := host.AddUnicastPubSub(unicastTopic+p.topicSuffix, func(ctx context.Context, _ io.Writer, data []byte) (err error) {
		// Blocking handling the unicast message until the agent is started
		<-ready
		var (
			unicast iotexrpc.UnicastMsg
			peerID  string
			latency int64
		)
		defer func() {
			status := successStr
			if err != nil {
				status = failureStr
			}
			p2pMsgCounter.WithLabelValues("unicast", strconv.Itoa(int(unicast.MsgType)), "in", peerID, status).Inc()
			p2pMsgLatency.WithLabelValues("unicast", strconv.Itoa(int(unicast.MsgType)), status).Observe(float64(latency))
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

		t, _ := ptypes.Timestamp(unicast.GetTimestamp())
		latency = time.Since(t).Nanoseconds() / time.Millisecond.Nanoseconds()

		stream, ok := p2p.GetUnicastStream(ctx)
		if !ok {
			err = errors.Wrap(err, "error when get unicast stream")
			return
		}
		remote := stream.Conn().RemotePeer()
		peerID = remote.Pretty()
		peerInfo := peer.AddrInfo{
			ID:    remote,
			Addrs: []multiaddr.Multiaddr{stream.Conn().RemoteMultiaddr()},
		}
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

	// connect to bootstrap nodes
	p.host = host
	if err = p.connect(ctx); err != nil {
		return err
	}
	p.host.JoinOverlay(ctx)
	close(ready)

	// check network connectivity every 60 blocks, and reconnect in case of disconnection
	p.reconnectTask = routine.NewRecurringTask(p.reconnect, p.reconnectTimeout)
	return p.reconnectTask.Start(ctx)
}

// Stop disconnects from P2P network
func (p *Agent) Stop(ctx context.Context) error {
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

// BroadcastOutbound sends a broadcast message to the whole network
func (p *Agent) BroadcastOutbound(ctx context.Context, msg proto.Message) (err error) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent("Agent BroadcastOutbound")
	defer span.End()

	host := p.host
	if host == nil {
		return ErrAgentNotStarted
	}
	var msgType iotexrpc.MessageType
	var msgBody []byte
	defer func() {
		status := successStr
		if err != nil {
			status = failureStr
		}
		p2pMsgCounter.WithLabelValues(
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
		Timestamp: ptypes.TimestampNow(),
	}
	data, err := proto.Marshal(&broadcast)
	if err != nil {
		err = errors.Wrap(err, "error when marshaling broadcast message")
		return
	}
	t := time.Now()
	if err = host.Broadcast(p.ctx, broadcastTopic+p.topicSuffix, data); err != nil {
		err = errors.Wrap(err, "error when sending broadcast message")
		p.qosMetrics.updateSendBroadcast(t, false)
		return
	}
	p.qosMetrics.updateSendBroadcast(t, true)
	return
}

// UnicastOutbound sends a unicast message to the given address
func (p *Agent) UnicastOutbound(_ context.Context, peer peer.AddrInfo, msg proto.Message) (err error) {
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
		status := successStr
		if err != nil {
			status = failureStr
		}
		p2pMsgCounter.WithLabelValues("unicast", strconv.Itoa(int(msgType)), "out", peer.ID.Pretty(), status).Inc()
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
		Timestamp: ptypes.TimestampNow(),
	}
	data, err := proto.Marshal(&unicast)
	if err != nil {
		err = errors.Wrap(err, "error when marshaling unicast message")
		return
	}

	t := time.Now()
	if err = host.Unicast(p.ctx, peer, unicastTopic+p.topicSuffix, data); err != nil {
		err = errors.Wrap(err, "error when sending unicast message")
		p.qosMetrics.updateSendUnicast(peerName, t, false)
		return
	}
	p.qosMetrics.updateSendUnicast(peerName, t, true)
	return
}

// Info returns agents' peer info.
func (p *Agent) Info() (peer.AddrInfo, error) {
	if p.host == nil {
		return peer.AddrInfo{}, ErrAgentNotStarted
	}
	return p.host.Info(), nil
}

// Self returns the self network address
func (p *Agent) Self() ([]multiaddr.Multiaddr, error) {
	if p.host == nil {
		return nil, ErrAgentNotStarted
	}
	return p.host.Addresses(), nil
}

// Neighbors returns the neighbors' peer info
func (p *Agent) Neighbors(ctx context.Context) ([]peer.AddrInfo, error) {
	if p.host == nil {
		return nil, ErrAgentNotStarted
	}

	// filter out bootnodes
	var nb []peer.AddrInfo
	for _, peer := range p.host.Neighbors(ctx) {
		isValid := true
		for _, bootNode := range p.bootNodeAddr {
			if strings.Contains(bootNode.String(), peer.ID.Pretty()) {
				isValid = false
				break
			}
		}
		if isValid {
			nb = append(nb, peer)
		}
	}
	return nb, nil
}

// QosMetrics returns the Qos metrics
func (p *Agent) QosMetrics() *Qos {
	return p.qosMetrics
}

// connect connects to bootstrap nodes
func (p *Agent) connect(ctx context.Context) error {
	if len(p.cfg.BootstrapNodes) == 0 {
		return nil
	}

	var tryNum, errNum, connNum, desiredConnNum int
	conn := make(chan struct{}, len(p.cfg.BootstrapNodes))
	connErrChan := make(chan error, len(p.cfg.BootstrapNodes))

	// try to connect to all bootstrap node beside itself.
	for _, bootAddr := range p.bootNodeAddr {
		tryNum++
		go func() {
			if err := exponentialRetry(
				func() error { return p.host.ConnectWithMultiaddr(ctx, bootAddr) },
				dialRetryInterval,
				numDialRetries,
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
			if errNum == tryNum {
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

func (p *Agent) reconnect() {
	peers, err := p.Neighbors(context.Background())
	if err == ErrAgentNotStarted {
		return
	}
	if len(peers) == 0 || p.qosMetrics.lostConnection() {
		log.L().Info("network lost, try re-connecting.")
		p.host.ClearBlocklist()
		p.connect(context.Background())
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
