// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"net"
	"sync"

	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/logger"
)

// PeerManager represents the outgoing neighbor list
// TODO: We should decouple peer address and peer. Node can know more nodes than it connects to
type PeerManager struct {
	service.CompositeService
	// TODO: Need to revisit sync.Map: https://github.com/golang/go/issues/24112
	Peers              sync.Map
	Overlay            *Overlay
	NumPeersLowerBound uint
	NumPeersUpperBound uint
}

// NewPeerManager creates an instance of PeerManager
func NewPeerManager(o *Overlay, lb uint, ub uint) *PeerManager {
	return &PeerManager{Overlay: o, NumPeersLowerBound: lb, NumPeersUpperBound: ub}
}

// AddPeer adds a new peer
func (pm *PeerManager) AddPeer(addr string) {
	if LenSyncMap(pm.Peers) >= pm.NumPeersUpperBound {
		logger.Debug().
			Uint("peers", pm.NumPeersUpperBound).
			Msg("Node already reached the max number of peers")
		return
	}
	if pm.Overlay.PRC.String() == addr {
		logger.Debug().
			Str("addr", addr).
			Msg("Node at address is the current node")
		return
	}
	_, ok := pm.Peers.Load(addr)
	if ok {
		logger.Debug().
			Str("addr", addr).
			Msg("Node at address is already the peer")
		return
	}
	if !pm.Overlay.Config.AllowMultiConnsPerIP {
		nHost, _, err := net.SplitHostPort(addr)
		if err != nil {
			logger.Error().
				Str("addr", addr).
				Msg("Node address is invalid")
			return
		}
		found := false
		pm.Peers.Range(func(key, value interface{}) bool {
			host, _, err := net.SplitHostPort(value.(*Peer).String())
			// This should be impossible, otherwise the connection couldn't be established
			if err != nil {
				logger.Error().
					Str("addr", addr).
					Msg("Node address is invalid")
				return true
			}
			if host == nHost {
				found = true
				return false
			}
			return true
		})
		if found {
			logger.Debug().
				Str("nHost", nHost).
				Msg("Another node on the same IP is already the peer")
			return
		}
	}
	p := NewTCPPeer(addr)
	p.Connect(pm.Overlay.Config)
	pm.Peers.Store(addr, p)
	logger.Debug().
		Str("src", pm.Overlay.PRC.String()).
		Str("dst", addr).
		Msg("establish an outgoing connection")
}

// RemovePeer removes an existing peer
func (pm *PeerManager) RemovePeer(addr string) {
	p, found := pm.Peers.Load(addr)
	if !found {
		logger.Debug().
			Str("addr", addr).
			Msg("Node at address is not a peer")
		return
	}
	pm.Peers.Delete(p.(*Peer).String())
	p.(*Peer).Close()
}

// RemoveLRUPeer removes the least recently used (contacted) peer
func (pm *PeerManager) RemoveLRUPeer() {
	minLastResTime := int64(0)
	addr := ""
	pm.Peers.Range(func(key, value interface{}) bool {
		lastResTime := value.(*Peer).LastResTime.Unix()
		if minLastResTime == 0 || lastResTime < minLastResTime {
			minLastResTime = lastResTime
			addr = key.(string)
		}
		return true
	})
	if addr != "" {
		pm.RemovePeer(addr)
	}
}

// GetOrAddPeer gets a peer. If it is still not in the neighbor list, it will be added first.
func (pm *PeerManager) GetOrAddPeer(addr string) *Peer {
	peer, ok := pm.Peers.Load(addr)
	if ok {
		return peer.(*Peer)
	}
	if LenSyncMap(pm.Peers) >= pm.NumPeersUpperBound {
		pm.RemoveLRUPeer()
	}
	// TODO: there could be race condition that another peer is added first
	pm.AddPeer(addr)
	peer, ok = pm.Peers.Load(addr)
	if ok {
		return peer.(*Peer)
	}
	return nil
}
