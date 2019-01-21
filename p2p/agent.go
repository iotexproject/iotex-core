// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	p2p "github.com/iotexproject/go-p2p"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p/pb"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/proto"
)

var (
	p2pMsgCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_p2p_message_counter",
			Help: "P2P message stats",
		},
		[]string{"protocol", "message", "direction", "status"},
	)
)

func init() {
	prometheus.MustRegister(p2pMsgCounter)
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
	broadcastInboundHandler    HandleBroadcastInbound
	unicastInboundAsyncHandler HandleUnicastInboundAsync
	host                       *p2p.Host
}

// NewAgent instantiates a local P2P agent instance
func NewAgent(cfg config.Network, broadcastHandler HandleBroadcastInbound, unicastHandler HandleUnicastInboundAsync) *Agent {
	return &Agent{
		cfg: cfg,
		broadcastInboundHandler:    broadcastHandler,
		unicastInboundAsyncHandler: unicastHandler,
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
	}
	if p.cfg.ExternalHost != "" {
		opts = append(opts, p2p.ExternalHostName(p.cfg.ExternalHost))
		opts = append(opts, p2p.ExternalPort(p.cfg.ExternalPort))
	}
	host, err := p2p.NewHost(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "error when instantiating Agent host")
	}

	if err := host.AddBroadcastPubSub(broadcastTopic, func(ctx context.Context, data []byte) (err error) {
		// Blocking handling the broadcast message until the agent is started
		<-ready
		var broadcast p2ppb.BroadcastMsg
		skip := false
		defer func() {
			// Skip accounting if the broadcast message is not handled
			if skip {
				return
			}
			status := "success"
			if err != nil {
				status = "failure"
			}
			p2pMsgCounter.WithLabelValues("broadcast", strconv.Itoa(int(broadcast.MsgType)), "in", status).Inc()
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
		if p.host.HostIdentity() == rawmsg.GetFrom().Pretty() {
			skip = true
			return
		}
		msg, err := iproto.TypifyProtoMsg(broadcast.MsgType, broadcast.MsgBody)
		if err != nil {
			err = errors.Wrap(err, "error when typifying broadcast message")
			return
		}
		p.broadcastInboundHandler(ctx, broadcast.ChainId, msg)
		return
	}); err != nil {
		return errors.Wrap(err, "error when adding broadcast pubsub")
	}

	if err := host.AddUnicastPubSub(unicastTopic, func(ctx context.Context, _ io.Writer, data []byte) (err error) {
		// Blocking handling the unicast message until the agent is started
		<-ready
		var unicast p2ppb.UnicastMsg
		defer func() {
			status := "success"
			if err != nil {
				status = "failure"
			}
			p2pMsgCounter.WithLabelValues("unicast", strconv.Itoa(int(unicast.MsgType)), "in", status).Inc()
		}()
		if err = proto.Unmarshal(data, &unicast); err != nil {
			err = errors.Wrap(err, "error when marshaling unicast message")
			return
		}
		msg, err := iproto.TypifyProtoMsg(unicast.MsgType, unicast.MsgBody)
		if err != nil {
			err = errors.Wrap(err, "error when typifying unicast message")
			return
		}
		stream, ok := p2p.GetUnicastStream(ctx)
		if !ok {
			err = errors.Wrap(err, "error when typifying unicast message")
			return
		}
		peerInfo := peerstore.PeerInfo{
			ID:    stream.Conn().RemotePeer(),
			Addrs: []multiaddr.Multiaddr{stream.Conn().RemoteMultiaddr()},
		}
		p.unicastInboundAsyncHandler(ctx, unicast.ChainId, peerInfo, msg)
		return
	}); err != nil {
		return errors.Wrap(err, "error when adding unicast pubsub")
	}

	if len(p.cfg.BootstrapNodes) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		randBootstrapNodeAddr := p.cfg.BootstrapNodes[r.Intn(len(p.cfg.BootstrapNodes))]
		bootAddr := multiaddr.StringCast(randBootstrapNodeAddr)
		if !strings.Contains(bootAddr.String(), host.HostIdentity()) {
			if err := exponentialRetry(
				func() error {
					return host.ConnectWithMultiaddr(ctx, bootAddr)
				},
				dialRetryInterval,
				numDialRetries,
			); err != nil {
				return errors.Wrapf(err, "error when connecting bootstrap node %s", randBootstrapNodeAddr)
			}
			log.L().Info("Connected bootstrap node.", zap.String("address", randBootstrapNodeAddr))
		}
	}
	if err := host.JoinOverlay(ctx); err != nil {
		return errors.Wrap(err, "error when joining overlay")
	}
	p.host = host
	close(ready)
	return nil
}

// Stop disconnects from P2P network
func (p *Agent) Stop(ctx context.Context) error {
	if p.host == nil {
		return nil
	}
	if err := p.host.Close(); err != nil {
		return errors.Wrap(err, "error when closing Agent host")
	}
	return nil
}

// BroadcastOutbound sends a broadcast message to the whole network
func (p *Agent) BroadcastOutbound(ctx context.Context, msg proto.Message) (err error) {
	var msgType uint32
	var msgBody []byte
	defer func() {
		status := "success"
		if err != nil {
			status = "failure"
		}
		p2pMsgCounter.WithLabelValues("broadcast", strconv.Itoa(int(msgType)), "out", status).Inc()
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
	broadcast := p2ppb.BroadcastMsg{ChainId: p2pCtx.ChainID, PeerId: p.host.HostIdentity(), MsgType: msgType, MsgBody: msgBody}
	data, err := proto.Marshal(&broadcast)
	if err != nil {
		err = errors.Wrap(err, "error when marshaling broadcast message")
		return
	}
	if err = p.host.Broadcast(broadcastTopic, data); err != nil {
		err = errors.Wrap(err, "error when sending broadcast message")
		return
	}
	return
}

// UnicastOutbound sends a unicast message to the given address
func (p *Agent) UnicastOutbound(ctx context.Context, peer peerstore.PeerInfo, msg proto.Message) (err error) {
	var msgType uint32
	var msgBody []byte
	defer func() {
		status := "success"
		if err != nil {
			status = "failure"
		}
		p2pMsgCounter.WithLabelValues("unicast", strconv.Itoa(int(msgType)), "out", status).Inc()
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
	unicast := p2ppb.UnicastMsg{ChainId: p2pCtx.ChainID, PeerId: p.host.HostIdentity(), MsgType: msgType, MsgBody: msgBody}
	data, err := proto.Marshal(&unicast)
	if err != nil {
		err = errors.Wrap(err, "error when marshaling unicast message")
		return
	}
	if err = p.host.Unicast(ctx, peer, unicastTopic, data); err != nil {
		err = errors.Wrap(err, "error when sending unicast message")
		return
	}
	return
}

// Info returns agents' peer info.
func (p *Agent) Info() peerstore.PeerInfo { return p.host.Info() }

// Self returns the self network address
func (p *Agent) Self() []multiaddr.Multiaddr { return p.host.Addresses() }

// Neighbors returns the neighbors' peer info
func (p *Agent) Neighbors(ctx context.Context) ([]peerstore.PeerInfo, error) {
	return p.host.Neighbors(ctx)
}

func convertAppMsg(msg proto.Message) (uint32, []byte, error) {
	msgType, err := iproto.GetTypeFromProtoMsg(msg)
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
