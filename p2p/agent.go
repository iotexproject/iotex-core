// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zjshen14/go-p2p"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/p2p/node"
	"github.com/iotexproject/iotex-core/p2p/pb"
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
	// HandleBroadcast handles broadcast message when agent listens it from the network
	HandleBroadcast func(uint32, proto.Message)

	// HandleUnicast handles unicast message when agent listens it from the network
	HandleUnicast func(uint32, net.Addr, proto.Message)
)

// Agent is the agent to help the blockchain node connect into the P2P networks and send/receive messages
type Agent struct {
	cfg              config.Network
	broadcastHandler HandleBroadcast
	unicastHandler   HandleUnicast
	host             *p2p.Host
}

// NewAgent instantiates a local P2P agent instance
func NewAgent(cfg config.Network, broadcastHandler HandleBroadcast, unicastHandler HandleUnicast) *Agent {
	return &Agent{
		cfg:              cfg,
		broadcastHandler: broadcastHandler,
		unicastHandler:   unicastHandler,
	}
}

// Start connects into P2P network
func (p *Agent) Start(ctx context.Context) error {
	p2p.SetLogger(logger.Logger())
	opts := []p2p.Option{
		p2p.HostName(p.cfg.Host),
		p2p.Port(p.cfg.Port),
		p2p.Gossip(),
		p2p.SecureIO(),
	}
	if p.cfg.ExternalHost != "" {
		opts = append(opts, p2p.ExternalHostName(p.cfg.ExternalHost))
		opts = append(opts, p2p.ExternalPort(p.cfg.ExternalPort))
	}
	host, err := p2p.NewHost(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "error when instantiating Agent host")
	}

	if err := host.AddBroadcastPubSub(broadcastTopic, func(data []byte) (err error) {
		var broadcast p2ppb.BroadcastMsg
		defer func() {
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
		if p.Self().String() == broadcast.Addr {
			return
		}
		msg, err := iproto.TypifyProtoMsg(broadcast.MsgType, broadcast.MsgBody)
		if err != nil {
			err = errors.Wrap(err, "error when typifying broadcast message")
			return
		}
		p.broadcastHandler(broadcast.ChainId, msg)
		return
	}); err != nil {
		return errors.Wrap(err, "error when adding broadcast pubsub")
	}

	if err := host.AddUnicastPubSub(unicastTopic, func(data []byte) (err error) {
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
		p.unicastHandler(unicast.ChainId, node.NewTCPNode(unicast.Addr), msg)
		return
	}); err != nil {
		return errors.Wrap(err, "error when adding unicast pubsub")
	}

	if len(p.cfg.BootstrapNodes) > 0 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		randBootstrapNodeAddr := p.cfg.BootstrapNodes[r.Intn(len(p.cfg.BootstrapNodes))]
		if randBootstrapNodeAddr != host.Address() &&
			randBootstrapNodeAddr != fmt.Sprintf("%s:%d", p.cfg.ExternalHost, p.cfg.ExternalPort) {
			if err := exponentialRetry(
				func() error {
					return host.Connect(randBootstrapNodeAddr)
				},
				dialRetryInterval,
				numDialRetries,
			); err != nil {
				return errors.Wrapf(err, "error when connecting bootstrap node %s", randBootstrapNodeAddr)
			}
			logger.Info().Str("address", randBootstrapNodeAddr).Msg("Connected bootstrap node")
		}
	}
	if err := host.JoinOverlay(); err != nil {
		return errors.Wrap(err, "error when joining overlay")
	}
	p.host = host
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

// Broadcast sends a broadcast message to the whole network
func (p *Agent) Broadcast(ctx context.Context, msg proto.Message) (err error) {
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
		err = fmt.Errorf("P2P context doesn't exist")
		return
	}
	broadcast := p2ppb.BroadcastMsg{ChainId: p2pCtx.ChainID, MsgType: msgType, MsgBody: msgBody}
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

// Unicast sends a unicast message to the given address
func (p *Agent) Unicast(ctx context.Context, addr net.Addr, msg proto.Message) (err error) {
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
		err = fmt.Errorf("P2P context doesn't exist")
		return
	}
	unicast := p2ppb.UnicastMsg{ChainId: p2pCtx.ChainID, Addr: p.Self().String(), MsgType: msgType, MsgBody: msgBody}
	data, err := proto.Marshal(&unicast)
	if err != nil {
		err = errors.Wrap(err, "error when marshaling unicast message")
		return
	}
	if err = p.host.Unicast(addr.String(), unicastTopic, data); err != nil {
		err = errors.Wrap(err, "error when sending unicast message")
		return
	}
	return
}

// Self returns the self network address
func (p *Agent) Self() net.Addr {
	return node.NewTCPNode(p.host.Address())
}

// Neighbors returns the neighbors' addresses
func (p *Agent) Neighbors() []net.Addr {
	neighbors := make([]net.Addr, 0)
	addrs, err := p.host.Neighbors()
	if err != nil {
		logger.Logger().Debug().Err(err).Msg("Error when getting the neighbors")
		// Usually it's because no closest peers
		return neighbors
	}
	for _, addr := range addrs {
		neighbors = append(neighbors, node.NewTCPNode(addr))
	}
	return neighbors
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
		logger.Error().Err(err).Msg("Error happens, will retry")
		time.Sleep(retryInterval)
		retryInterval *= 2
	}
	return
}
