// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"net"
	"syscall"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/network/proto"
	"github.com/iotexproject/iotex-core/proto"
)

var (
	// ErrPeerNotFound means the peer is not found
	ErrPeerNotFound = errors.New("Peer not found")
)

// Overlay represents the peer-to-peer network
type Overlay struct {
	service.CompositeService
	PM         *PeerManager
	PRC        *RPCServer
	Gossip     *Gossip
	Tasks      []*routine.RecurringTask
	Config     *config.Network
	Dispatcher dispatcher.Dispatcher
}

// NewOverlay creates an instance of Overlay
func NewOverlay(config *config.Network) *Overlay {
	o := &Overlay{Config: config}
	o.PRC = NewRPCServer(o)
	o.PM = NewPeerManager(o, config.NumPeersLowerBound, config.NumPeersUpperBound)
	o.Gossip = NewGossip(o)
	o.AddService(o.PRC)
	o.AddService(o.PM)
	o.AddService(o.Gossip)

	o.addPingTask()
	o.addHealthCheckTask()
	if config.PeerDiscovery {
		o.addPeerMaintainer()
	} else {
		o.addConfigBasedPeerMaintainer()
	}
	return o
}

// AttachDispatcher attaches to a Dispatcher instance
func (o *Overlay) AttachDispatcher(dispatcher dispatcher.Dispatcher) {
	o.Dispatcher = dispatcher
	o.Gossip.AttachDispatcher(dispatcher)
}

func (o *Overlay) addPingTask() {
	ping := NewPinger(o)
	pingTask := routine.NewRecurringTask(ping, o.Config.PingInterval)
	o.AddService(pingTask)
	o.Tasks = append(o.Tasks, pingTask)
}

func (o *Overlay) addHealthCheckTask() {
	hc := NewHealthChecker(o)
	hcTask := routine.NewRecurringTask(hc, o.Config.HealthCheckInterval)
	o.AddService(hcTask)
	o.Tasks = append(o.Tasks, hcTask)
}

func (o *Overlay) addPeerMaintainer() {
	pm := NewPeerMaintainer(o)
	pmTask := routine.NewRecurringTask(pm, o.Config.PeerMaintainerInterval)
	o.AddService(pmTask)
	o.Tasks = append(o.Tasks, pmTask)
}

func (o *Overlay) addConfigBasedPeerMaintainer() {
	topology, err := config.LoadTopology(o.Config.TopologyPath)
	if err != nil {
		logger.Error().Err(err)
		syscall.Exit(syscall.SYS_EXIT)
	}
	cbpm := NewConfigBasedPeerMaintainer(o, topology)
	cbpmTask := routine.NewRecurringTask(cbpm, o.Config.PeerMaintainerInterval)
	o.AddService(cbpmTask)
	o.Tasks = append(o.Tasks, cbpmTask)
}

// Broadcast lets the caller to broadcast the message to all nodes in the P2P network
func (o *Overlay) Broadcast(msg proto.Message) error {
	msgType, err := iproto.GetTypeFromProtoMsg(msg)
	if err != nil {
		return err
	}
	msgBody, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	// Source also needs to remember the message sent so that it wouldn't process it again
	o.Gossip.storeBroadcastMsgChecksum(o.Gossip.getBroadcastMsgChecksum(msgBody))
	// Kick off the message
	o.Gossip.relayMsg(msgType, msgBody, o.Config.TTL)
	return nil
}

// GetPeers returns the current neighbors' network identifiers
func (o *Overlay) GetPeers() []net.Addr {
	nodes := []net.Addr{}
	o.PM.Peers.Range(func(_, value interface{}) bool {
		nodes = append(nodes, &cm.Node{Addr: value.(*Peer).String()})
		return true
	})
	return nodes
}

// Tell tells a given node a proto message
func (o *Overlay) Tell(node net.Addr, msg proto.Message) error {
	peer := o.PM.GetOrAddPeer(node.String())
	if peer == nil {
		return ErrPeerNotFound
	}

	msgType, err := iproto.GetTypeFromProtoMsg(msg)
	if err != nil {
		return err
	}
	msgBody, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	go peer.Tell(&pb.TellReq{Addr: o.PRC.String(), MsgType: msgType, MsgBody: msgBody})
	return nil
}

// Self returns the PRC server address to receive messages
func (o *Overlay) Self() net.Addr {
	return o.PRC
}
