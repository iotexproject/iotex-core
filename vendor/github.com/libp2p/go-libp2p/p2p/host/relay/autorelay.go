package relay

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	basic "github.com/libp2p/go-libp2p/p2p/host/basic"

	autonat "github.com/libp2p/go-libp2p-autonat"
	_ "github.com/libp2p/go-libp2p-circuit"
	discovery "github.com/libp2p/go-libp2p-discovery"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	routing "github.com/libp2p/go-libp2p-routing"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

const (
	RelayRendezvous = "/libp2p/relay"
)

var (
	DesiredRelays = 3

	BootDelay = 20 * time.Second
)

// AutoRelay is a Host that uses relays for connectivity when a NAT is detected.
type AutoRelay struct {
	host     *basic.BasicHost
	discover discovery.Discoverer
	router   routing.PeerRouting
	autonat  autonat.AutoNAT
	addrsF   basic.AddrsFactory

	disconnect chan struct{}

	mx     sync.Mutex
	relays map[peer.ID]pstore.PeerInfo
	addrs  []ma.Multiaddr
}

func NewAutoRelay(ctx context.Context, bhost *basic.BasicHost, discover discovery.Discoverer, router routing.PeerRouting) *AutoRelay {
	ar := &AutoRelay{
		host:       bhost,
		discover:   discover,
		router:     router,
		addrsF:     bhost.AddrsFactory,
		relays:     make(map[peer.ID]pstore.PeerInfo),
		disconnect: make(chan struct{}, 1),
	}
	ar.autonat = autonat.NewAutoNAT(ctx, bhost, ar.baseAddrs)
	bhost.AddrsFactory = ar.hostAddrs
	bhost.Network().Notify(ar)
	go ar.background(ctx)
	return ar
}

func (ar *AutoRelay) hostAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	ar.mx.Lock()
	defer ar.mx.Unlock()
	if ar.addrs != nil && ar.autonat.Status() == autonat.NATStatusPrivate {
		return ar.addrs
	} else {
		return ar.addrsF(addrs)
	}
}

func (ar *AutoRelay) baseAddrs() []ma.Multiaddr {
	return ar.addrsF(ar.host.AllAddrs())
}

func (ar *AutoRelay) background(ctx context.Context) {
	select {
	case <-time.After(autonat.AutoNATBootDelay + BootDelay):
	case <-ctx.Done():
		return
	}

	// when true, we need to identify push
	push := false

	for {
		wait := autonat.AutoNATRefreshInterval
		switch ar.autonat.Status() {
		case autonat.NATStatusUnknown:
			wait = autonat.AutoNATRetryInterval

		case autonat.NATStatusPublic:
			// invalidate addrs
			ar.mx.Lock()
			if ar.addrs != nil {
				ar.addrs = nil
				push = true
			}
			ar.mx.Unlock()

			// if we had previously announced relay addrs, push our public addrs
			if push {
				push = false
				ar.host.PushIdentify()
			}

		case autonat.NATStatusPrivate:
			push = false // clear, findRelays pushes as needed
			ar.findRelays(ctx)
		}

		select {
		case <-ar.disconnect:
			// invalidate addrs
			ar.mx.Lock()
			if ar.addrs != nil {
				ar.addrs = nil
				push = true
			}
			ar.mx.Unlock()
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}
	}
}

func (ar *AutoRelay) findRelays(ctx context.Context) {
	ar.mx.Lock()
	if len(ar.relays) >= DesiredRelays {
		ar.mx.Unlock()
		return
	}
	need := DesiredRelays - len(ar.relays)
	ar.mx.Unlock()

	limit := 50
	if need > limit/2 {
		limit = 2 * need
	}

	dctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	pis, err := discovery.FindPeers(dctx, ar.discover, RelayRendezvous, limit)
	cancel()
	if err != nil {
		log.Debugf("error discovering relays: %s", err.Error())
		return
	}

	pis = ar.selectRelays(pis)

	update := 0

	for _, pi := range pis {
		ar.mx.Lock()
		if _, ok := ar.relays[pi.ID]; ok {
			ar.mx.Unlock()
			continue
		}
		ar.mx.Unlock()

		cctx, cancel := context.WithTimeout(ctx, 60*time.Second)

		if len(pi.Addrs) == 0 {
			pi, err = ar.router.FindPeer(cctx, pi.ID)
			if err != nil {
				log.Debugf("error finding relay peer %s: %s", pi.ID, err.Error())
				cancel()
				continue
			}
		}

		err = ar.host.Connect(cctx, pi)
		cancel()
		if err != nil {
			log.Debugf("error connecting to relay %s: %s", pi.ID, err.Error())
			continue
		}

		log.Debugf("connected to relay %s", pi.ID)
		ar.mx.Lock()
		ar.relays[pi.ID] = pi
		ar.mx.Unlock()

		// tag the connection as very important
		ar.host.ConnManager().TagPeer(pi.ID, "relay", 42)

		update++
		need--
		if need == 0 {
			break
		}
	}

	if update > 0 || ar.addrs == nil {
		ar.updateAddrs()
	}
}

func (ar *AutoRelay) selectRelays(pis []pstore.PeerInfo) []pstore.PeerInfo {
	// TODO better relay selection strategy; this just selects random relays
	//      but we should probably use ping latency as the selection metric
	shuffleRelays(pis)
	return pis
}

func (ar *AutoRelay) updateAddrs() {
	ar.doUpdateAddrs()
	ar.host.PushIdentify()
}

// This function updates our NATed advertised addrs (ar.addrs)
// The public addrs are rewritten so that they only retain the public IP part; they
// become undialable but are useful as a hint to the dialer to determine whether or not
// to dial private addrs.
// The non-public addrs are included verbatim so that peers behind the same NAT/firewall
// can still dial us directly.
// On top of those, we add the relay-specific addrs for the relays to which we are
// connected. For each non-private relay addr, we encapsulate the p2p-circuit addr
// through which we can be dialed.
func (ar *AutoRelay) doUpdateAddrs() {
	ar.mx.Lock()
	defer ar.mx.Unlock()

	addrs := ar.baseAddrs()
	raddrs := make([]ma.Multiaddr, 0, len(addrs)+len(ar.relays))

	// remove our public addresses from the list
	for _, addr := range addrs {
		if manet.IsPublicAddr(addr) {
			continue
		}
		raddrs = append(raddrs, addr)
	}

	// add relay specific addrs to the list
	for _, pi := range ar.relays {
		circuit, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s/p2p-circuit", pi.ID.Pretty()))
		if err != nil {
			panic(err)
		}

		for _, addr := range pi.Addrs {
			if !manet.IsPrivateAddr(addr) {
				pub := addr.Encapsulate(circuit)
				raddrs = append(raddrs, pub)
			}
		}
	}

	ar.addrs = raddrs
}

func shuffleRelays(pis []pstore.PeerInfo) {
	for i := range pis {
		j := rand.Intn(i + 1)
		pis[i], pis[j] = pis[j], pis[i]
	}
}

func (ar *AutoRelay) Listen(inet.Network, ma.Multiaddr)      {}
func (ar *AutoRelay) ListenClose(inet.Network, ma.Multiaddr) {}
func (ar *AutoRelay) Connected(inet.Network, inet.Conn)      {}

func (ar *AutoRelay) Disconnected(net inet.Network, c inet.Conn) {
	p := c.RemotePeer()

	ar.mx.Lock()
	defer ar.mx.Unlock()

	if ar.host.Network().Connectedness(p) == inet.Connected {
		// We have a second connection.
		return
	}

	if _, ok := ar.relays[p]; ok {
		delete(ar.relays, p)
		select {
		case ar.disconnect <- struct{}{}:
		default:
		}
	}
}

func (ar *AutoRelay) OpenedStream(inet.Network, inet.Stream) {}
func (ar *AutoRelay) ClosedStream(inet.Network, inet.Stream) {}
