// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
)

// TODO: HeartbeatHandler opens encapsulation of a few structs to inspect the internal status, we need to find a better
// approach to do so in the future

var heartbeatMtc = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "iotex_heartbeat_status",
		Help: "Node heartbeat status.",
	},
	[]string{"status_type", "chain_id"},
)

func init() {
	prometheus.MustRegister(heartbeatMtc)
}

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
	p2p, ok := h.s.P2P().(*network.IotxOverlay)
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
	dp, ok := h.s.Dispatcher().(*dispatcher.IotxDispatcher)
	if !ok {
		logger.Error().Msg("dispatcher is not the instance of IotxDispatcher")
		return
	}
	numDPEvts := len(*dp.EventChan())
	dpEvtsAudit, err := json.Marshal(dp.EventAudit())
	if err != nil {
		logger.Error().Msg("error when serializing the dispatcher event audit map")
		return
	}

	logger.Info().
		Uint("numPeers", numPeers).
		Time("lastOut", lastOutTime).
		Time("lastIn", lastInTime).
		Int("pendingDispatcherEvents", numDPEvts).
		Str("pendingDispatcherEventsAudit", string(dpEvtsAudit)).
		Msg("node status")

	heartbeatMtc.WithLabelValues("numPeers").Set(float64(numPeers))
	heartbeatMtc.WithLabelValues("pendingDispatcherEvents").Set(float64(numDPEvts))
	// chain service
	for _, c := range h.s.chainservices {
		// Consensus metrics
		cs, ok := c.Consensus().(*consensus.IotxConsensus)
		if !ok {
			logger.Error().Msg("consensus is not the instance of IotxConsensus")
			return
		}
		rolldpos, ok := cs.Scheme().(*rolldpos.RollDPoS)
		numPendingEvts := 0
		var state fsm.State
		if ok {
			numPendingEvts = rolldpos.NumPendingEvts()
			state = rolldpos.CurrentState()
		} else {
			logger.Debug().Msg("scheme is not the instance of RollDPoS")
		}

		// Block metrics
		height := c.Blockchain().TipHeight()

		actPoolSize := c.ActionPool().GetSize()
		actPoolCapacity := c.ActionPool().GetCapacity()

		logger.Info().
			Int("rolldposEvents", numPendingEvts).
			Str("fsmState", string(state)).
			Uint64("blockchainHeight", height).
			Uint64("actpoolSize", actPoolSize).
			Uint64("actpoolCapacity", actPoolCapacity).
			Uint32("chainID", c.ChainID()).
			Msg("chain service status")

		chainIDStr := strconv.FormatUint(uint64(c.ChainID()), 10)
		heartbeatMtc.WithLabelValues("pendingRolldposEvents", chainIDStr).Set(float64(numPendingEvts))
		heartbeatMtc.WithLabelValues("blockchainHeight", chainIDStr).Set(float64(height))
		heartbeatMtc.WithLabelValues("actpoolSize", chainIDStr).Set(float64(actPoolSize))
		heartbeatMtc.WithLabelValues("actpoolCapacity", chainIDStr).Set(float64(actPoolCapacity))
	}

}
