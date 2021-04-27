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

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/iotexproject/go-p2p"
	goproto "github.com/iotexproject/iotex-proto/golang"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
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
	HandleBroadcastInbound func(context.Context, uint32, proto.Message)

	// HandleUnicastInboundAsync handles unicast message when agent listens it from the network
	HandleUnicastInboundAsync func(context.Context, uint32, peerstore.PeerInfo, proto.Message)
)

// Agent is the agent to help the blockchain node connect into the P2P networks and send/receive messages
type Agent struct {
	cfg                        config.Network
	topicSuffix                string
	broadcastInboundHandler    HandleBroadcastInbound
	unicastInboundAsyncHandler HandleUnicastInboundAsync
	host                       *p2p.Host
	unicastBlocklist           *BlockList
	reconnectTask              *routine.RecurringTask
}

// NewAgent instantiates a local P2P agent instance
func NewAgent(cfg config.Config, broadcastHandler HandleBroadcastInbound, unicastHandler HandleUnicastInboundAsync) *Agent {
	gh := cfg.Genesis.Hash()
	log.L().Info("p2p agent", log.Hex("topicSuffix", gh[22:]))
	return &Agent{
		cfg: cfg.Network,
		// Make sure the honest node only care the messages related the chain from the same genesis
		topicSuffix:                hex.EncodeToString(gh[22:]), // last 10 bytes of genesis hash
		broadcastInboundHandler:    broadcastHandler,
		unicastInboundAsyncHandler: unicastHandler,
		unicastBlocklist:           NewBlockList(blockListLen),
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

	if err := host.AddBroadcastPubSub(broadcastTopic+p.topicSuffix, func(ctx context.Context, data []byte) (err error) {
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

		t, _ := ptypes.Timestamp(broadcast.GetTimestamp())
		latency = time.Since(t).Nanoseconds() / time.Millisecond.Nanoseconds()

		msg, err := goproto.TypifyRPCMsg(broadcast.MsgType, broadcast.MsgBody)
		if err != nil {
			err = errors.Wrap(err, "error when typifying broadcast message")
			return
		}
		p.broadcastInboundHandler(ctx, broadcast.ChainId, msg)
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

		t, _ := ptypes.Timestamp(unicast.GetTimestamp())
		latency = time.Since(t).Nanoseconds() / time.Millisecond.Nanoseconds()

		stream, ok := p2p.GetUnicastStream(ctx)
		if !ok {
			err = errors.Wrap(err, "error when typifying unicast message")
			return
		}
		peerID = stream.Conn().RemotePeer().Pretty()
		peerInfo := peerstore.PeerInfo{
			ID:    stream.Conn().RemotePeer(),
			Addrs: []multiaddr.Multiaddr{stream.Conn().RemoteMultiaddr()},
		}
		p.unicastInboundAsyncHandler(ctx, unicast.ChainId, peerInfo, msg)
		return
	}); err != nil {
		return errors.Wrap(err, "error when adding unicast pubsub")
	}

	// connect to bootstrap nodes
	p.host = host
	if err = p.connect(ctx); err != nil {
		return err
	}
	p.host.JoinOverlay(ctx)
	close(ready)

	// check network connectivity every 2 blocks, and reconnect in case of disconnection
	p.reconnectTask = routine.NewRecurringTask(p.reconnect, 2*config.DardanellesBlockInterval)
	return p.reconnectTask.Start(ctx)
}

// Stop disconnects from P2P network
func (p *Agent) Stop(ctx context.Context) error {
	if p.host == nil {
		return nil
	}
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
	p2pCtx, ok := GetContext(ctx)
	if !ok {
		err = errors.New("P2P context doesn't exist")
		return
	}
	broadcast := iotexrpc.BroadcastMsg{
		ChainId:   p2pCtx.ChainID,
		PeerId:    host.HostIdentity(),
		MsgType:   msgType,
		MsgBody:   msgBody,
		Timestamp: ptypes.TimestampNow(),
	}
	data, err := proto.Marshal(&broadcast)
	if err != nil {
		err = errors.Wrap(err, "error when marshaling broadcast message")
		return err
	}
	if err = host.Broadcast(broadcastTopic+p.topicSuffix, data); err != nil {
		err = errors.Wrap(err, "error when sending broadcast message")
		return err
	}
	return err
}

// UnicastOutbound sends a unicast message to the given address
func (p *Agent) UnicastOutbound(ctx context.Context, peer peerstore.PeerInfo, msg proto.Message) (err error) {
	var (
		peerName = peer.ID.Pretty()
		msgType  iotexrpc.MessageType
		msgBody  []byte
	)
	host := p.host
	if host == nil {
		return ErrAgentNotStarted
	}
	defer func() {
		status := successStr
		if err != nil {
			status = failureStr
		}
		p2pMsgCounter.WithLabelValues("unicast", strconv.Itoa(int(msgType)), "out", peer.ID.Pretty(), status).Inc()
	}()

	if p.unicastBlocklist.Blocked(peerName, time.Now()) {
		err = errors.New("peer is in blocklist at this moment")
		return
	}

	msgType, msgBody, err = convertAppMsg(msg)
	if err != nil {
		return
	}
	p2pCtx, ok := GetContext(ctx)
	if !ok {
		err = errors.New("P2P context doesn't exist")
		return
	}
	unicast := iotexrpc.UnicastMsg{
		ChainId:   p2pCtx.ChainID,
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

	if err = host.Unicast(ctx, peer, unicastTopic+p.topicSuffix, data); err != nil {
		err = errors.Wrap(err, "error when sending unicast message")
		p.unicastBlocklist.Add(peerName, time.Now())
		return
	}

	// remove peer from blocklist upon success
	p.unicastBlocklist.Remove(peerName)
	return
}

// Info returns agents' peer info.
func (p *Agent) Info() (peerstore.PeerInfo, error) {
	host := p.host
	if host == nil {
		return peerstore.PeerInfo{}, ErrAgentNotStarted
	}
	return host.Info(), nil
}

// Self returns the self network address
func (p *Agent) Self() ([]multiaddr.Multiaddr, error) {
	host := p.host
	if host == nil {
		return nil, ErrAgentNotStarted
	}
	return host.Addresses(), nil
}

// Neighbors returns the neighbors' peer info
func (p *Agent) Neighbors(ctx context.Context) ([]peerstore.PeerInfo, error) {
	host := p.host
	if host == nil {
		return nil, ErrAgentNotStarted
	}
	var res []peerstore.PeerInfo
	nbs, err := host.Neighbors(ctx)
	if err != nil {
		return nbs, err
	}

	for i, nb := range nbs {
		if p.unicastBlocklist.Blocked(nb.ID.Pretty(), time.Now()) {
			continue
		}
		res = append(res, nbs[i])
	}
	return res, nil
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
	for _, bootstrapNode := range p.cfg.BootstrapNodes {
		bootAddr := multiaddr.StringCast(bootstrapNode)
		if strings.Contains(bootAddr.String(), p.host.HostIdentity()) {
			continue
		}

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
	desiredConnNum = len(p.cfg.BootstrapNodes)/2 + 1
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
	ctx := context.Background()
	peers, err := p.Neighbors(ctx)
	if err != nil || len(peers) == 0 {
		log.L().Info("Network lost, try re-connecting.", zap.Error(err))
		p.connect(ctx)
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
