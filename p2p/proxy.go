package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

type (
	// SubnetProxy wrap agent with one specific network
	SubnetProxy interface {
		Agent
		// BroadcastOutbound sends a broadcast message to the network
		BroadcastOutbound(ctx context.Context, msg proto.Message) (err error)
		// UnicastOutbound sends a unicast message to the given address
		UnicastOutbound(ctx context.Context, peer peer.AddrInfo, msg proto.Message) (err error)
	}

	subnetProxy struct {
		*agent
		network string
	}
)

func (ap *subnetProxy) BroadcastOutbound(ctx context.Context, msg proto.Message) error {
	return ap.agent.BroadcastOutbound(ctx, ap.network, msg)
}

func (ap *subnetProxy) UnicastOutbound(ctx context.Context, peer peer.AddrInfo, msg proto.Message) (err error) {
	return ap.agent.UnicastOutbound(ctx, peer, ap.network, msg)
}

func (ap *subnetProxy) ConnectedPeers() (peers []peer.AddrInfo, err error) {
	return ap.agent.connectedPeersByNetwork(ap.network)
}
