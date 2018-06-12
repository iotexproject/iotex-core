// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"fmt"
	"math/rand"

	pb "github.com/iotexproject/iotex-core/network/proto"
)

// Pinger is the recurring logic to constantly check if the node can talk to its peers
type Pinger struct {
	Overlay *Overlay
}

// NewPinger creates an instance of Pinger
func NewPinger(o *Overlay) *Pinger {
	return &Pinger{Overlay: o}
}

// Do ping the neighbor peers
func (h *Pinger) Do() {
	h.Overlay.PM.Peers.Range(func(_, value interface{}) bool {
		go func() {
			n := rand.Uint64()
			pong, error := value.(*Peer).Ping(&pb.Ping{Nonce: n, Addr: h.Overlay.PRC.String()})
			if error == nil && pong.AckNonce == n {
				// TODO: We need to handle wrong response case too
				fmt.Print("Handle wrong response case")
			}
		}()
		return true
	})
}
