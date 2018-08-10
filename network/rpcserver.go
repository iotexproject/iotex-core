// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network/node"
	pb "github.com/iotexproject/iotex-core/network/proto"
	"github.com/iotexproject/iotex-core/pkg/counter"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/proto"
)

var _ lifecycle.StartStopper = (*RPCServer)(nil)

// RPCServer represents the listener at the transportation layer
type RPCServer struct {
	node.Node

	Server  *grpc.Server
	Overlay *IotxOverlay

	counters    sync.Map
	rateLimit   uint64
	lastReqTime time.Time
}

// NewRPCServer creates an instance of RPCServer
func NewRPCServer(o *IotxOverlay) *RPCServer {
	s := &RPCServer{Overlay: o}
	portStr := strconv.Itoa(o.Config.Port)
	s.Addr = strings.Join([]string{o.Config.Host, portStr}, ":")
	s.rateLimit = o.Config.RateLimitPerSec * uint64(o.Config.RateLimitWindowSize) / uint64(time.Second)
	return s
}

// Ping implements the server side RPC logic
func (s *RPCServer) Ping(ctx context.Context, ping *pb.Ping) (*pb.Pong, error) {
	drop, err := s.shouldDropRequest(ctx)
	s.updateLastResTime()
	if err != nil {
		return nil, err
	}
	if drop {
		return nil, fmt.Errorf("sended requests too frequently")
	}
	s.Overlay.PM.AddPeer(ping.Addr)
	return &pb.Pong{AckNonce: ping.Nonce}, nil
}

// GetPeers implements the server side RPC logic
func (s *RPCServer) GetPeers(ctx context.Context, req *pb.GetPeersReq) (*pb.GetPeersRes, error) {
	drop, err := s.shouldDropRequest(ctx)
	s.updateLastResTime()
	if err != nil {
		return nil, err
	}
	if drop {
		return nil, fmt.Errorf("sended requests too frequently")
	}
	var addrs []string
	s.Overlay.PM.Peers.Range(func(key, value interface{}) bool {
		addrs = append(addrs, value.(*Peer).String())
		return true
	})
	stringsAreShuffled(addrs)
	res := &pb.GetPeersRes{}
	if req.Count <= uint32(len(addrs)) {
		res.Addr = addrs[:req.Count]
	} else {
		res.Addr = addrs
	}
	return res, nil
}

// Broadcast implements the server side RPC logic
func (s *RPCServer) Broadcast(ctx context.Context, req *pb.BroadcastReq) (*pb.BroadcastRes, error) {
	drop, err := s.shouldDropRequest(ctx)
	s.updateLastResTime()
	if err != nil {
		return nil, err
	}
	if drop {
		return nil, fmt.Errorf("sended requests too frequently")
	}
	err = s.Overlay.Gossip.OnReceivingMsg(req)
	if err == nil {
		return &pb.BroadcastRes{Header: iproto.MagicBroadcastMsgHeader}, nil
	}
	return nil, err
}

// Tell implements the server side RPC logic
func (s *RPCServer) Tell(ctx context.Context, req *pb.TellReq) (*pb.TellRes, error) {
	drop, err := s.shouldDropRequest(ctx)
	s.updateLastResTime()
	if err != nil {
		return nil, err
	}
	if drop {
		return nil, fmt.Errorf("sended requests too frequently")
	}
	protoMsg, err := iproto.TypifyProtoMsg(req.MsgType, req.MsgBody)
	if err != nil {
		return nil, err
	}
	if s.Overlay.Dispatcher != nil {
		s.Overlay.Dispatcher.HandleTell(node.NewTCPNode(req.Addr), protoMsg, nil)
	}
	return &pb.TellRes{Header: iproto.MagicBroadcastMsgHeader}, nil
}

// Start starts the rpc server
func (s *RPCServer) Start(_ context.Context) error {
	lis, err := net.Listen(s.Network(), s.String())
	if err != nil {
		logger.Error().Err(err).Msg("Node failed to listen")
		return err
	}
	s.Addr = lis.Addr().String()
	// Create the gRPC server with the credentials
	if s.Overlay.Config.TLSEnabled {
		creds, err := generateServerCredentials(s.Overlay.Config)
		if err != nil {
			return err
		}
		s.Server = grpc.NewServer(
			grpc.Creds(creds),
			grpc.KeepaliveEnforcementPolicy(s.Overlay.Config.KLPolicy),
			grpc.KeepaliveParams(s.Overlay.Config.KLServerParams),
			grpc.MaxRecvMsgSize(s.Overlay.Config.MaxMsgSize))
	} else {
		s.Server = grpc.NewServer(
			grpc.KeepaliveEnforcementPolicy(s.Overlay.Config.KLPolicy),
			grpc.KeepaliveParams(s.Overlay.Config.KLServerParams),
			grpc.MaxRecvMsgSize(1024*1024*10))
	}

	pb.RegisterPeerServer(s.Server, s)
	// Register reflection service on gRPC peer.
	reflection.Register(s.Server)
	started := make(chan bool)
	go func(started chan bool) {
		logger.Info().Msg("start RPC server")
		started <- true
		if err := s.Server.Serve(lis); err != nil {
			logger.Fatal().Err(err).Msg("Node failed to serve")
		}
	}(started)
	<-started
	return nil
}

// Stop stops the rpc server
func (s *RPCServer) Stop(_ context.Context) error {
	logger.Info().Msg("stop RPC server")
	s.Server.Stop()
	return nil
}

// LastReqTime returns the timestamp of the last accepted request
func (s *RPCServer) LastReqTime() time.Time {
	return s.lastReqTime
}

func (s *RPCServer) shouldDropRequest(ctx context.Context) (bool, error) {
	if !s.Overlay.Config.RateLimitEnabled {
		return false, nil
	}
	addr, err := s.getClientAddr(ctx)
	if err != nil {
		return false, err
	}
	c, _ := s.counters.LoadOrStore(
		addr,
		counter.NewSlidingWindowCounterWithSecondSlot(s.Overlay.Config.RateLimitWindowSize))
	c.(*counter.SlidingWindowCounter).Increment()
	if c.(*counter.SlidingWindowCounter).Count() > s.rateLimit {
		return true, nil
	}
	return false, nil
}

func (s *RPCServer) getClientAddr(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("failed to get peer from ctx")
	}
	if p.Addr == net.Addr(nil) {
		return "", fmt.Errorf("failed to get peer address")
	}
	return p.Addr.String(), nil
}

// Update the last time when successfully getting an req from the peer
func (s *RPCServer) updateLastResTime() {
	s.lastReqTime = time.Now()
}
