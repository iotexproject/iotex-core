package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"google.golang.org/protobuf/proto"
)

type (
	// AgentProxy wrap agent with one specific network
	AgentProxy interface {
		Agent
		// BroadcastOutbound sends a broadcast message to the whole network
		BroadcastOutbound(ctx context.Context, msg proto.Message) (err error)
		// UnicastOutbound sends a unicast message to the given address
		UnicastOutbound(ctx context.Context, peer peer.AddrInfo, msg proto.Message) (err error)
		// ConnectedPeers returns the connected peers' info
		ConnectedPeers() ([]peer.AddrInfo, error)
	}

	agentProxy struct {
		*agent
		network string
	}
)

func (ap *agentProxy) BroadcastOutbound(ctx context.Context, msg proto.Message) error {
	return ap.BroadcastOutboundByNetwork(ctx, ap.network, msg)
}

func (ap *agentProxy) UnicastOutbound(ctx context.Context, peer peer.AddrInfo, msg proto.Message) (err error) {
	return ap.UnicastOutboundByNetwork(ctx, peer, ap.network, msg)
}

func (ap *agentProxy) ConnectedPeers() (peers []peer.AddrInfo, err error) {
	return ap.ConnectedPeersByNetwork(ap.network)
}
