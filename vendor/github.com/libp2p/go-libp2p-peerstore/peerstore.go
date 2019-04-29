package peerstore

import (
	"fmt"
	"io"
	"sync"

	peer "github.com/libp2p/go-libp2p-peer"
)

var _ Peerstore = (*peerstore)(nil)

const maxInternedProtocols = 512
const maxInternedProtocolSize = 256

type peerstore struct {
	Metrics

	KeyBook
	AddrBook
	PeerMetadata

	// lock for protocol information, separate from datastore lock
	protolock         sync.RWMutex
	internedProtocols map[string]string
}

// NewPeerstore creates a data structure that stores peer data, backed by the
// supplied implementations of KeyBook, AddrBook and PeerMetadata.
func NewPeerstore(kb KeyBook, ab AddrBook, md PeerMetadata) Peerstore {
	return &peerstore{
		KeyBook:           kb,
		AddrBook:          ab,
		PeerMetadata:      md,
		Metrics:           NewMetrics(),
		internedProtocols: make(map[string]string),
	}
}

func (ps *peerstore) Close() (err error) {
	var errs []error
	weakClose := func(name string, c interface{}) {
		if cl, ok := c.(io.Closer); ok {
			if err = cl.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s error: %s", name, err))
			}
		}
	}

	weakClose("keybook", ps.KeyBook)
	weakClose("addressbook", ps.AddrBook)
	weakClose("peermetadata", ps.PeerMetadata)

	if len(errs) > 0 {
		return fmt.Errorf("failed while closing peerstore; err(s): %q", errs)
	}
	return nil
}

func (ps *peerstore) Peers() peer.IDSlice {
	set := map[peer.ID]struct{}{}
	for _, p := range ps.PeersWithKeys() {
		set[p] = struct{}{}
	}
	for _, p := range ps.PeersWithAddrs() {
		set[p] = struct{}{}
	}

	pps := make(peer.IDSlice, 0, len(set))
	for p := range set {
		pps = append(pps, p)
	}
	return pps
}

func (ps *peerstore) PeerInfo(p peer.ID) PeerInfo {
	return PeerInfo{
		ID:    p,
		Addrs: ps.AddrBook.Addrs(p),
	}
}

func (ps *peerstore) internProtocol(s string) string {
	if len(s) > maxInternedProtocolSize {
		return s
	}

	if interned, ok := ps.internedProtocols[s]; ok {
		return interned
	}

	if len(ps.internedProtocols) >= maxInternedProtocols {
		ps.internedProtocols = make(map[string]string, maxInternedProtocols)
	}

	ps.internedProtocols[s] = s
	return s
}

func (ps *peerstore) SetProtocols(p peer.ID, protos ...string) error {
	ps.protolock.Lock()
	defer ps.protolock.Unlock()

	protomap := make(map[string]struct{}, len(protos))
	for _, proto := range protos {
		protomap[ps.internProtocol(proto)] = struct{}{}
	}

	return ps.Put(p, "protocols", protomap)
}

func (ps *peerstore) AddProtocols(p peer.ID, protos ...string) error {
	ps.protolock.Lock()
	defer ps.protolock.Unlock()
	protomap, err := ps.getProtocolMap(p)
	if err != nil {
		return err
	}

	for _, proto := range protos {
		protomap[ps.internProtocol(proto)] = struct{}{}
	}

	return ps.Put(p, "protocols", protomap)
}

func (ps *peerstore) getProtocolMap(p peer.ID) (map[string]struct{}, error) {
	iprotomap, err := ps.Get(p, "protocols")
	switch err {
	default:
		return nil, err
	case ErrNotFound:
		return make(map[string]struct{}), nil
	case nil:
		cast, ok := iprotomap.(map[string]struct{})
		if !ok {
			return nil, fmt.Errorf("stored protocol set was not a map")
		}

		return cast, nil
	}
}

func (ps *peerstore) GetProtocols(p peer.ID) ([]string, error) {
	ps.protolock.RLock()
	defer ps.protolock.RUnlock()
	pmap, err := ps.getProtocolMap(p)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(pmap))
	for k := range pmap {
		out = append(out, k)
	}

	return out, nil
}

func (ps *peerstore) SupportsProtocols(p peer.ID, protos ...string) ([]string, error) {
	ps.protolock.RLock()
	defer ps.protolock.RUnlock()
	pmap, err := ps.getProtocolMap(p)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(protos))
	for _, proto := range protos {
		if _, ok := pmap[proto]; ok {
			out = append(out, proto)
		}
	}

	return out, nil
}

func PeerInfos(ps Peerstore, peers peer.IDSlice) []PeerInfo {
	pi := make([]PeerInfo, len(peers))
	for i, p := range peers {
		pi[i] = ps.PeerInfo(p)
	}
	return pi
}

func PeerInfoIDs(pis []PeerInfo) peer.IDSlice {
	ps := make(peer.IDSlice, len(pis))
	for i, pi := range pis {
		ps[i] = pi.ID
	}
	return ps
}
