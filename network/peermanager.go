// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"net"
	"sync"

	"github.com/golang/glog"

	"github.com/iotexproject/iotex-core/common/service"
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
	if lenSyncMap(pm.Peers) >= pm.NumPeersUpperBound {
		glog.Infof("Node already reaches the max number of peers: %d", pm.NumPeersUpperBound)
		return
	}
	if pm.Overlay.PRC.String() == addr {
		glog.Infof("Node at address %s is the current node", addr)
		return
	}
	_, ok := pm.Peers.Load(addr)
	if ok {
		glog.Infof("Node at address %s is already the peer", addr)
		return
	}
	if !pm.Overlay.Config.AllowMultiConnsPerIP {
		nHost, _, err := net.SplitHostPort(addr)
		if err != nil {
			glog.Errorf("Node address %s is invalid", addr)
			return
		}
		found := false
		pm.Peers.Range(func(key, value interface{}) bool {
			host, _, err := net.SplitHostPort(value.(*Peer).String())
			// This should be impossible, otherwise the connection couldn't be established
			if err != nil {
				glog.Errorf("Node address %s is invalid", addr)
				return true
			}
			if host == nHost {
				found = true
				return false
			}
			return true
		})
		if found {
			glog.Infof("Another node on the same IP %s is already the peer", nHost)
			return
		}
	}
	p := NewTCPPeer(addr)
	p.Connect(pm.Overlay.Config)
	pm.Peers.Store(addr, p)
}

// RemovePeer removes an existing peer
func (pm *PeerManager) RemovePeer(addr string) {
	p, found := pm.Peers.Load(addr)
	if !found {
		glog.Infof("Node at address %s is not a peer", addr)
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
	if lenSyncMap(pm.Peers) >= pm.NumPeersUpperBound {
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
