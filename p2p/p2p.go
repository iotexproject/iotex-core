// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"context"
	"fmt"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/zjshen14/go-p2p"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/p2p/node"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// TODO: the topic could be fine tuned
	broadcastTopic = "broadcast"
	unicastTopic   = "unicast"
)

// HandleBroadcast handles broadcast message when agent listens it from the network
type HandleBroadcast func(uint32, proto.Message, chan bool)

// HandleUnicast handles unicast message when agent listens it from the network
type HandleUnicast func(uint32, net.Addr, proto.Message, chan bool)

// Agent is the agent to help the blockchain node connect into the P2P networks and send/receive messages
type Agent struct {
	cfg         config.Network
	broadcastCB HandleBroadcast
	unicastCB   HandleUnicast
	host        *p2p.Host
}

// NewAgent instantiates a local P2P agent instance
func NewAgent(cfg config.Network, broadcastCB HandleBroadcast, unicastCB HandleUnicast) *Agent {
	return &Agent{
		cfg:         cfg,
		broadcastCB: broadcastCB,
		unicastCB:   unicastCB,
	}
}

// Start connects into P2P network
func (p *Agent) Start(ctx context.Context) error {
	p2p.SetLogger(logger.Logger())
	host, err := p2p.NewHost(ctx, p2p.HostName(p.cfg.Host), p2p.Port(p.cfg.Port), p2p.Gossip())
	if err != nil {
		return errors.Wrap(err, "error when instantiating Agent host")
	}
	if err := host.AddBroadcastPubSub(broadcastTopic, func(data []byte) error {
		var broadcast BroadcastMsg
		if err := proto.Unmarshal(data, &broadcast); err != nil {
			return errors.Wrap(err, "error when marshaling broadcast message")
		}
		msg, err := iproto.TypifyProtoMsg(broadcast.MsgType, broadcast.MsgBody)
		if err != nil {
			return errors.Wrap(err, "error when typifying broadcast message")
		}
		p.broadcastCB(broadcast.ChainId, msg, nil)
		return nil
	}); err != nil {
		return errors.Wrap(err, "error when adding broadcast pubsub")
	}
	if err := host.AddUnicastPubSub(unicastTopic, func(data []byte) error {
		var unicast UnicastMsg
		if err := proto.Unmarshal(data, &unicast); err != nil {
			return errors.Wrap(err, "error when marshaling unicast message")
		}
		msg, err := iproto.TypifyProtoMsg(unicast.MsgType, unicast.MsgBody)
		if err != nil {
			return errors.Wrap(err, "error when typifying unicast message")
		}
		p.unicastCB(unicast.ChainId, node.NewTCPNode(unicast.Addr), msg, nil)
		return nil

	}); err != nil {
		return errors.Wrap(err, "error when adding unicast pubsub")
	}
	if err := host.JoinOverlay(); err != nil {
		return errors.Wrap(err, "error when joining overlay")
	}
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
func (p *Agent) Broadcast(ctx context.Context, msg proto.Message) error {
	msgType, msgBody, err := covertAppMsg(msg)
	if err != nil {
		return err
	}
	p2pCtx, ok := GetContext(ctx)
	if !ok {
		return fmt.Errorf("agent context doesn't exist")
	}
	broadcast := BroadcastMsg{ChainId: p2pCtx.ChainID, MsgType: msgType, MsgBody: msgBody}
	data, err := proto.Marshal(&broadcast)
	if err != nil {
		return errors.Wrap(err, "error when marshaling broadcast message")
	}
	if err := p.host.Broadcast(broadcastTopic, data); err != nil {
		return errors.Wrap(err, "error when sending broadcast message")
	}
	return nil
}

// Unicast sends a unicast message to the given address
func (p *Agent) Unicast(ctx context.Context, addr net.Addr, msg proto.Message) error {
	msgType, msgBody, err := covertAppMsg(msg)
	if err != nil {
		return err
	}
	p2pCtx, ok := GetContext(ctx)
	if !ok {
		return fmt.Errorf("agent context doesn't exist")
	}
	unicast := UnicastMsg{ChainId: p2pCtx.ChainID, Addr: p.Self().String(), MsgType: msgType, MsgBody: msgBody}
	data, err := proto.Marshal(&unicast)
	if err != nil {
		return errors.Wrap(err, "error when marshaling unicast message")
	}
	if err := p.host.Unicast(addr.String(), unicastTopic, data); err != nil {
		return errors.Wrap(err, "error when sending unicast message")
	}
	return err
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

func covertAppMsg(msg proto.Message) (uint32, []byte, error) {
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
