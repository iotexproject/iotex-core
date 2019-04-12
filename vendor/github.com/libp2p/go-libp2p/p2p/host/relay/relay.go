package relay

import (
	"context"
	"time"

	circuit "github.com/libp2p/go-libp2p-circuit"
	discovery "github.com/libp2p/go-libp2p-discovery"
	ma "github.com/multiformats/go-multiaddr"
)

var (
	AdvertiseBootDelay = 30 * time.Second
)

// Advertise advertises this node as a libp2p relay.
func Advertise(ctx context.Context, advertise discovery.Advertiser) {
	go func() {
		select {
		case <-time.After(AdvertiseBootDelay):
			discovery.Advertise(ctx, advertise, RelayRendezvous)
		case <-ctx.Done():
		}
	}()
}

// Filter filters out all relay addresses.
func Filter(addrs []ma.Multiaddr) []ma.Multiaddr {
	raddrs := make([]ma.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		_, err := addr.ValueForProtocol(circuit.P_CIRCUIT)
		if err == nil {
			continue
		}
		raddrs = append(raddrs, addr)
	}
	return raddrs
}
