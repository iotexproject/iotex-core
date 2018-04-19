// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"time"
)

// HealthChecker will check its peers at constant interval. If a peer is found not reachable for given period, it would
// be removed from the peer list
type HealthChecker struct {
	Overlay        *Overlay
	SilentInterval time.Duration
}

// NewHealthChecker creates an instance of HealthChecker
func NewHealthChecker(o *Overlay) *HealthChecker {
	hc := &HealthChecker{Overlay: o}
	hc.SilentInterval = o.Config.SilentInterval
	return hc
}

// Do check peer health
func (hc *HealthChecker) Do() {
	addrs := []string{}
	hc.Overlay.PM.Peers.Range(func(key, value interface{}) bool {
		if time.Now().Sub(value.(*Peer).LastResTime) > hc.SilentInterval {
			addrs = append(addrs, key.(string))
		}
		return true
	})
	for _, addr := range addrs {
		go hc.Overlay.PM.RemovePeer(addr)
	}
}
