// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"context"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network/node"
	pb "github.com/iotexproject/iotex-core/network/proto"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/proto"
)

// ErrPeerNotFound means the peer is not found
var ErrPeerNotFound = errors.New("Peer not found")

// Overlay represents the peer-to-peer network
type Overlay interface {
	lifecycle.StartStopper
	Broadcast(proto.Message) error
	Tell(net.Addr, proto.Message) error
	Self() net.Addr
	GetPeers() []net.Addr
}

// IotxOverlay is the implementation
type IotxOverlay struct {
	PM         *PeerManager
	RPC        *RPCServer
	Gossip     *Gossip
	Tasks      []*routine.RecurringTask
	Config     *config.Network
	Dispatcher dispatcher.Dispatcher

	lifecycle lifecycle.Lifecycle
}

// NewOverlay creates an instance of IotxOverlay
func NewOverlay(config *config.Network) *IotxOverlay {
	o := &IotxOverlay{Config: config}
	o.RPC = NewRPCServer(o)
	o.PM = NewPeerManager(o, config.NumPeersLowerBound, config.NumPeersUpperBound)
	o.Gossip = NewGossip(o)
	o.lifecycle.AddModels(o.RPC, o.PM, o.Gossip)

	o.addPingTask()
	o.addHealthCheckTask()
	if config.PeerDiscovery {
		o.addPeerMaintainer()
	} else {
		o.addConfigBasedPeerMaintainer()
	}
	return o
}

// Start starts IotxOverlay and it's sub-models.
func (o *IotxOverlay) Start(ctx context.Context) error { return o.lifecycle.OnStart(ctx) }

// Stop stops IotxOverlay and it's sub-models.
func (o *IotxOverlay) Stop(ctx context.Context) error { return o.lifecycle.OnStop(ctx) }

// AttachDispatcher attaches to a Dispatcher instance
func (o *IotxOverlay) AttachDispatcher(dispatcher dispatcher.Dispatcher) {
	o.Dispatcher = dispatcher
	o.Gossip.AttachDispatcher(dispatcher)
}

func (o *IotxOverlay) addPingTask() {
	ping := NewPinger(o)
	pingTask := routine.NewRecurringTask(ping.Ping, o.Config.PingInterval)
	o.lifecycle.Add(pingTask)
	o.Tasks = append(o.Tasks, pingTask)
}

func (o *IotxOverlay) addHealthCheckTask() {
	hc := NewHealthChecker(o)
	hcTask := routine.NewRecurringTask(hc.Check, o.Config.HealthCheckInterval)
	o.lifecycle.Add(hcTask)
	o.Tasks = append(o.Tasks, hcTask)
}

func (o *IotxOverlay) addPeerMaintainer() {
	pm := NewPeerMaintainer(o)
	pmTask := routine.NewRecurringTask(pm.Update, o.Config.PeerMaintainerInterval)
	o.lifecycle.Add(pmTask)
	o.Tasks = append(o.Tasks, pmTask)
}

func (o *IotxOverlay) addConfigBasedPeerMaintainer() {
	topology, err := NewTopology(o.Config.TopologyPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Fail to load topology")
	}
	cbpm := NewConfigBasedPeerMaintainer(o, topology)
	cbpmTask := routine.NewRecurringTask(cbpm.Update, o.Config.PeerMaintainerInterval)
	o.lifecycle.Add(cbpmTask)
	o.Tasks = append(o.Tasks, cbpmTask)
}

// Broadcast lets the caller to broadcast the message to all nodes in the P2P network
func (o *IotxOverlay) Broadcast(msg proto.Message) error {
	msgType, err := iproto.GetTypeFromProtoMsg(msg)
	if err != nil {
		return errors.Wrap(err, "failed to convert msg to proto when broadcast")
	}
	msgBody, err := proto.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal msg when broadcast")
	}
	// Source also needs to remember the message sent so that it wouldn't process it again
	o.Gossip.storeBroadcastMsgChecksum(o.Gossip.getBroadcastMsgChecksum(msgBody))
	// Kick off the message
	err = o.Gossip.relayMsg(msgType, msgBody, o.Config.TTL)
	if err != nil {
		return errors.Wrap(err, "failed to relay msg when broadcast")
	}
	return nil
}

// GetPeers returns the current neighbors' network identifiers
func (o *IotxOverlay) GetPeers() []net.Addr {
	var nodes []net.Addr
	o.PM.Peers.Range(func(_, value interface{}) bool {
		nodes = append(nodes, &node.Node{Addr: value.(*Peer).String()})
		return true
	})
	return nodes
}

// Tell tells a given node a proto message
func (o *IotxOverlay) Tell(node net.Addr, msg proto.Message) error {
	peer := o.PM.GetOrAddPeer(node.String())
	if peer == nil {
		return ErrPeerNotFound
	}

	msgType, err := iproto.GetTypeFromProtoMsg(msg)
	if err != nil {
		return errors.Wrap(err, "failed to convert msg to proto when tell msg")
	}
	msgBody, err := proto.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal msg when broadcast")
	}
	go func(p *Peer) {
		_, err := p.Tell(&pb.TellReq{Addr: o.RPC.String(), MsgType: msgType, MsgBody: msgBody})
		if err != nil {
			logger.Error().
				Str("dst", p.String()).
				Str("msg-type", string(msgType)).
				Msg("failed to tell a message")
		}
	}(peer)
	return nil
}

// Self returns the RPC server address to receive messages
func (o *IotxOverlay) Self() net.Addr {
	return o.RPC
}
