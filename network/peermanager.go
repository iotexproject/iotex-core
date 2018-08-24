// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"net"
	"sync"

	"github.com/iotexproject/iotex-core/logger"
)

// PeerManager represents the outgoing neighbor list
// TODO: We should decouple peer address and peer. Node can know more nodes than it connects to
type PeerManager struct {
	// TODO: Need to revisit sync.Map: https://github.com/golang/go/issues/24112
	Peers              sync.Map
	Overlay            *IotxOverlay
	NumPeersLowerBound uint
	NumPeersUpperBound uint
}

// NewPeerManager creates an instance of PeerManager
func NewPeerManager(o *IotxOverlay, lb uint, ub uint) *PeerManager {
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
	if pm.Overlay.RPC.String() == addr {
		logger.Debug().
			Str("dst", addr).
			Msg("Node at address is the current node")
		return
	}
	_, ok := pm.Peers.Load(addr)
	if ok {
		logger.Debug().
			Str("dst", addr).
			Msg("Node at address is already the peer")
		return
	}
	if !pm.Overlay.Config.AllowMultiConnsPerHost {
		nHost, _, err := net.SplitHostPort(addr)
		if err != nil {
			logger.Error().
				Str("dst", addr).
				Msg("Node address is invalid")
			return
		}
		found := false
		pm.Peers.Range(func(key, value interface{}) bool {
			host, _, err := net.SplitHostPort(value.(*Peer).String())
			// This should be impossible, otherwise the connection couldn't be established
			if err != nil {
				logger.Error().
					Str("dst", addr).
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
				Str("dst-host", nHost).
				Msg("Another node on the same Host is already the peer")
			return
		}
	}
	p := NewTCPPeer(addr)
	err := p.Connect(pm.Overlay.Config)
	if err != nil {
		logger.Error().
			Str("dst", addr).
			Msg("failed to establish an outgoing connection")
	}
	pm.Peers.Store(addr, p)
	logger.Debug().
		Str("dst", addr).
		Msg("establish an outgoing connection")
}

// RemovePeer removes an existing peer
func (pm *PeerManager) RemovePeer(addr string) {
	p, found := pm.Peers.Load(addr)
	if !found {
		logger.Debug().
			Str("dst", addr).
			Msg("Node at address is not a peer")
		return
	}
	pm.Peers.Delete(p.(*Peer).String())
	err := p.(*Peer).Close()
	if err != nil {
		logger.Error().
			Str("dst", addr).
			Msg("failed to terminate an outgoing connection")
	}
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
