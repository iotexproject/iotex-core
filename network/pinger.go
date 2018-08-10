// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"math/rand"

	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/network/proto"
)

// Pinger is the recurring logic to constantly check if the node can talk to its peers
type Pinger struct {
	Overlay *IotxOverlay
}

// NewPinger creates an instance of Pinger
func NewPinger(o *IotxOverlay) *Pinger {
	return &Pinger{Overlay: o}
}

// Ping pings the neighbor peers
func (h *Pinger) Ping() {
	h.Overlay.PM.Peers.Range(func(_, value interface{}) bool {
		go func() {
			n := rand.Uint64()
			p, ok := value.(*Peer)
			if !ok {
				logger.Error().Msg("value is not an instance of Peer")
				return
			}
			pong, err := p.Ping(&pb.Ping{Nonce: n, Addr: h.Overlay.RPC.String()})
			if err != nil {
				logger.Error().Err(err).Str("dst", p.String()).Msg("error when getting pong")
				return
			}
			if pong == nil {
				logger.Error().Str("dst", p.String()).Msg("nil pong")
				return
			}
			if pong.AckNonce != n {
				logger.Error().
					Str("dst", p.String()).
					Uint64("out-nonce", n).
					Uint64("in-nonce", pong.AckNonce).
					Msg("pong carries an unmatched nonce")
			}
		}()
		return true
	})
}
