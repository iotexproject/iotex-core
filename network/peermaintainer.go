// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"math/rand"
	"net"

	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	pb "github.com/iotexproject/iotex-core/network/proto"
)

// PeerMaintainer helps maintain enough connections to other peers in the P2P networks
type PeerMaintainer struct {
	Overlay *Overlay
}

// NewPeerMaintainer creates an instance of PeerMaintainer
func NewPeerMaintainer(o *Overlay) *PeerMaintainer {
	return &PeerMaintainer{Overlay: o}
}

// Do maintain peer connection. Current strategy is to get the (upper_bound - count) peer addresses from one of the
// current peer if the count is lower than the lower bound
func (pm *PeerMaintainer) Do() {
	count := LenSyncMap(pm.Overlay.PM.Peers)
	if count == 0 {
		// TODO: Now we simply read the bootstrap nodes from the config. This needs to be changed in the future
		bns1 := pm.Overlay.Config.BootstrapNodes
		bns2 := make([]string, len(bns1))
		copy(bns2, bns1)
		stringsAreShuffled(bns2)
		for i, bn := range bns2 {
			if uint(i) >= pm.Overlay.PM.NumPeersLowerBound {
				break
			}
			pm.Overlay.PM.AddPeer(bn)
		}
	} else if count < pm.Overlay.PM.NumPeersLowerBound {
		targetIdx := rand.Intn(int(count))
		idx := 0
		// It's possible that no target is reached because the peer is deleted concurrently, so that targetIdx is
		// greater than the number of peers
		var target *Peer
		pm.Overlay.PM.Peers.Range(func(_, value interface{}) bool {
			if idx == targetIdx {
				target = value.(*Peer)
				return false
			}
			idx++
			return true
		})
		go func() {
			if target != nil {
				res, error := target.GetPeers(
					&pb.GetPeersReq{Count: uint32(pm.Overlay.PM.NumPeersUpperBound - count)})
				if res != nil && error == nil {
					for _, addr := range res.Addr {
						pm.Overlay.PM.AddPeer(addr)
					}
				}
			}
		}()
	} else if count > pm.Overlay.PM.NumPeersUpperBound {
		for count > pm.Overlay.PM.NumPeersUpperBound {
			pm.Overlay.PM.RemoveLRUPeer()
			count--
		}
	}
}

// ConfigBasedPeerMaintainer maintain the neighbors by reading the topology file
type ConfigBasedPeerMaintainer struct {
	Overlay *Overlay
	Addrs   []net.Addr
}

// NewConfigBasedPeerMaintainer creates an instance of ConfigBasedPeerMaintainer
func NewConfigBasedPeerMaintainer(o *Overlay, t *config.Topology) *ConfigBasedPeerMaintainer {
	cbpm := &ConfigBasedPeerMaintainer{Overlay: o}
	for host, neighbors := range t.NeighborList {
		if host == o.PRC.String() {
			for _, addr := range neighbors {
				cbpm.Addrs = append(cbpm.Addrs, cm.NewTCPNode(addr))
			}
		}
	}
	return cbpm
}

// Do adds the configured addresses into the peer list
func (cbpm *ConfigBasedPeerMaintainer) Do() {
	addrs := make(map[string]bool)
	for _, addr := range cbpm.Addrs {
		addrs[addr.String()] = false
	}
	cbpm.Overlay.PM.Peers.Range(func(key, value interface{}) bool {
		addrs[key.(string)] = true
		return true
	})
	for addr, ok := range addrs {
		if !ok {
			cbpm.Overlay.PM.AddPeer(addr)
		}
	}
}
