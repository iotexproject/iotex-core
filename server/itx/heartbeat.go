// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"time"

	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/dispatch"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
)

// TODO: HeartbeatHandler opens encapsulation of a few structs to inspect the internal status, we need to find a better
// approach to do so in the future

// HeartbeatHandler is the handler to periodically log the system key metrics
type HeartbeatHandler struct {
	s *Server
}

// NewHeartbeatHandler instantiates a HeartbeatHandler instance
func NewHeartbeatHandler(s *Server) *HeartbeatHandler {
	return &HeartbeatHandler{s: s}
}

// Log executes the logging logic
func (h *HeartbeatHandler) Log() {
	// Network metrics
	p2p, ok := h.s.P2p().(*network.IotxOverlay)
	if !ok {
		logger.Error().Msg("value is not the instance of IotxOverlay")
		return
	}
	numPeers := network.LenSyncMap(p2p.PM.Peers)
	lastOutTime := time.Unix(0, 0)
	p2p.PM.Peers.Range(func(_, value interface{}) bool {
		p, ok := value.(*network.Peer)
		if !ok {
			logger.Error().Msg("value is not the instance of Peer")
			return true
		}
		if p.LastResTime.After(lastOutTime) {
			lastOutTime = p.LastResTime
		}
		return true
	})
	lastInTime := p2p.RPC.LastReqTime()

	// Dispatcher metrics
	dp, ok := h.s.Dp().(*dispatch.IotxDispatcher)
	if !ok {
		logger.Error().Msg("dispatcher is not the instance of IotxDispatcher")
		return
	}
	numDPEvts := len(*dp.EventChan())

	// Consensus metrics
	cs, ok := h.s.cs.(*consensus.IotxConsensus)
	if !ok {
		logger.Error().Msg("consensus is not the instance of IotxConsensus")
		return
	}
	rolldpos, ok := cs.Scheme().(*rolldpos.RollDPoS)
	numCSEvts := 0
	var state fsm.State
	if ok {
		numCSEvts = len(*rolldpos.EventChan())
		state = rolldpos.FSM().CurrentState()
	} else {
		logger.Debug().Msg("scheme is not the instance of RollDPoS")
	}

	// Block metrics
	height, err := h.s.Bc().TipHeight()
	if err != nil {
		logger.Error().Err(err).Msg("error one getting the the blockchain height")
		height = 0
	}

	logger.Info().
		Uint("num-peers", numPeers).
		Time("last-out", lastOutTime).
		Time("last-in", lastInTime).
		Int("dispatcher-events", numDPEvts).
		Int("rolldpos-events", numCSEvts).
		Str("fsm-state", string(state)).
		Uint64("height", height).
		Msg("node status")
}
