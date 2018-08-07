// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network/node"
	pb "github.com/iotexproject/iotex-core/network/proto"
	"github.com/iotexproject/iotex-core/proto"
)

// Peer represents a node in the peer-to-peer networks
type Peer struct {
	node.Node
	Client      pb.PeerClient
	Conn        *grpc.ClientConn
	Ctx         context.Context
	LastResTime time.Time
}

// NewTCPPeer creates an instance of Peer with tcp transportation
func NewTCPPeer(addr string) *Peer {
	return NewPeer("tcp", addr)
}

// NewPeer creates an instance of Peer
func NewPeer(n string, addr string) *Peer {
	p := &Peer{LastResTime: time.Now()}
	p.NetworkType = n
	p.Addr = addr
	return p
}

// Connect connects the peer
func (p *Peer) Connect(config *config.Network) error {
	// Set up a connection to the peer.
	var conn *grpc.ClientConn
	var err error
	if config.TLSEnabled {
		creds, err := generateClientCredentials(config)
		if err != nil {
			return err
		}
		conn, err = grpc.Dial(
			p.String(),
			grpc.WithTransportCredentials(creds),
			grpc.WithKeepaliveParams(config.KLClientParams),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(config.MaxMsgSize)))
		if err != nil {
			return err
		}
	} else {
		conn, err = grpc.Dial(
			p.String(),
			grpc.WithInsecure(),
			grpc.WithKeepaliveParams(config.KLClientParams))
	}

	if err != nil {
		logger.Error().Err(err).Str("dst", p.String()).Msg("Peer did not connect")
		return err
	}
	p.Conn = conn
	p.Client = pb.NewPeerClient(conn)
	p.Ctx = context.Background()
	return nil
}

// Close terminates the connection
func (p *Peer) Close() error {
	return p.Conn.Close()
}

// Ping implements the client side RPC
func (p *Peer) Ping(ping *pb.Ping) (*pb.Pong, error) {
	pong, err := p.Client.Ping(p.Ctx, ping)
	if err == nil {
		p.updateLastResTime()
	}
	return pong, err
}

// GetPeers implements the client side RPC
func (p *Peer) GetPeers(req *pb.GetPeersReq) (*pb.GetPeersRes, error) {
	res, err := p.Client.GetPeers(p.Ctx, req)
	if err == nil {
		p.updateLastResTime()
	}
	return res, err
}

// BroadcastMsg implements the client side RPC
func (p *Peer) BroadcastMsg(req *pb.BroadcastReq) (*pb.BroadcastRes, error) {
	req.Header = iproto.MagicBroadcastMsgHeader
	res, err := p.Client.Broadcast(p.Ctx, req)
	if err == nil {
		p.updateLastResTime()
	}
	return res, err
}

// Tell implements the client side RPC
func (p *Peer) Tell(req *pb.TellReq) (*pb.TellRes, error) {
	req.Header = iproto.MagicBroadcastMsgHeader
	res, err := p.Client.Tell(p.Ctx, req)
	if err == nil {
		p.updateLastResTime()
	}
	return res, err
}

// Update the last time when successfully getting an response from the peer
func (p *Peer) updateLastResTime() {
	p.LastResTime = time.Now()
}
