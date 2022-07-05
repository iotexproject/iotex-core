// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log/zlog"
	"github.com/iotexproject/iotex-core/pkg/version"
	statedb "github.com/iotexproject/iotex-core/state"
)

// TODO: HeartbeatHandler opens encapsulation of a few structs to inspect the internal status, we need to find a better
// approach to do so in the future

var _heartbeatMtc = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "iotex_heartbeat_status",
		Help: "Node heartbeat status.",
	},
	[]string{"status_type", "source"},
)

var _versionMtc = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "iotex_version_status",
		Help: "Node software version status.",
	},
	[]string{"type", "value"},
)

func init() {
	prometheus.MustRegister(_heartbeatMtc)
	prometheus.MustRegister(_versionMtc)
}

// HeartbeatHandler is the handler to periodically log the system key metrics
type HeartbeatHandler struct {
	s *Server
	l zerolog.Logger
}

// NewHeartbeatHandler instantiates a HeartbeatHandler instance
func NewHeartbeatHandler(s *Server, cfg p2p.Network) *HeartbeatHandler {
	return &HeartbeatHandler{
		s: s,
		l: zlog.L().With().Str("networkAddr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)).Logger(),
	}
}

// Log executes the logging logic
func (h *HeartbeatHandler) Log() {
	// Network metrics
	p2pAgent := h.s.P2PAgent()

	// Dispatcher metrics
	dp, ok := h.s.Dispatcher().(*dispatcher.IotxDispatcher)
	if !ok {
		h.l.Error().Msg("dispatcher is not the instance of IotxDispatcher")
		return
	}
	numDPEvts := dp.EventQueueSize()
	totalDPEventNumber := 0
	events := []string{}
	for event, num := range numDPEvts {
		totalDPEventNumber += num
		events = append(events, event+":"+strconv.Itoa(num))
	}
	dpEvtsAudit, err := json.Marshal(dp.EventAudit())
	if err != nil {
		h.l.Err(err).Msg("error when serializing the dispatcher event audit map.")
		return
	}

	ctx := context.Background()
	peers, err := p2pAgent.Neighbors(ctx)
	if err != nil {
		h.l.Debug().Err(err).Msg("error when get neighbors.")
		peers = nil
	}
	numPeers := len(peers)
	h.l.Debug().Fields(map[string]interface{}{
		"numPeers":                     numPeers,
		"pendingDispatcherEvents":      "{" + strings.Join(events, ", ") + "}",
		"pendingDispatcherEventsAudit": string(dpEvtsAudit),
	}).Msg("Node status.")

	_heartbeatMtc.WithLabelValues("numPeers", "node").Set(float64(numPeers))
	_heartbeatMtc.WithLabelValues("pendingDispatcherEvents", "node").Set(float64(totalDPEventNumber))
	// chain service
	for _, c := range h.s.chainservices {
		// Consensus metrics
		cs, ok := c.Consensus().(*consensus.IotxConsensus)
		if !ok {
			h.l.Info().Msg("consensus is not the instance of IotxConsensus.")
			return
		}
		rolldpos, ok := cs.Scheme().(*rolldpos.RollDPoS)
		numPendingEvts := 0
		consensusEpoch := uint64(0)
		consensusHeight := uint64(0)
		height := c.Blockchain().TipHeight()

		var consensusMetrics scheme.ConsensusMetrics
		var state fsm.State
		if ok {
			numPendingEvts = rolldpos.NumPendingEvts()
			state = rolldpos.CurrentState()

			// RollDpos Consensus Metrics
			consensusMetrics, err = rolldpos.Metrics()
			if err != nil {
				if height > 0 || errors.Cause(err) != statedb.ErrStateNotExist {
					h.l.Err(err).Msg("failed to read consensus metrics")
					return
				}
			}
			consensusEpoch = consensusMetrics.LatestEpoch
			consensusHeight = consensusMetrics.LatestHeight
		} else {
			h.l.Debug().Msg("scheme is not the instance of RollDPoS")
		}

		// Block metrics
		actPoolSize := c.ActionPool().GetSize()
		actPoolCapacity := c.ActionPool().GetCapacity()
		targetHeight := c.BlockSync().TargetHeight()

		h.l.Debug().Fields(map[string]interface{}{
			"rolldposEvents":   numPendingEvts,
			"fsmState":         string(state),
			"blockchainHeight": height,
			"actpoolSize":      actPoolSize,
			"actpoolCapacity":  actPoolCapacity,
			"chainID":          c.ChainID(),
			"targetHeight":     targetHeight,
			"concensusEpoch":   consensusEpoch,
			"consensusHeight":  consensusHeight,
		}).Msg("chain service status")

		chainIDStr := strconv.FormatUint(uint64(c.ChainID()), 10)
		_heartbeatMtc.WithLabelValues("consensusEpoch", chainIDStr).Set(float64(consensusHeight))
		_heartbeatMtc.WithLabelValues("consensusRound", chainIDStr).Set(float64(consensusEpoch))
		_heartbeatMtc.WithLabelValues("pendingRolldposEvents", chainIDStr).Set(float64(numPendingEvts))
		_heartbeatMtc.WithLabelValues("blockchainHeight", chainIDStr).Set(float64(height))
		_heartbeatMtc.WithLabelValues("actpoolSize", chainIDStr).Set(float64(actPoolSize))
		_heartbeatMtc.WithLabelValues("actpoolGasInPool", chainIDStr).Set(float64(c.ActionPool().GetGasSize()))
		_heartbeatMtc.WithLabelValues("actpoolCapacity", chainIDStr).Set(float64(actPoolCapacity))
		_heartbeatMtc.WithLabelValues("targetHeight", chainIDStr).Set(float64(targetHeight))
		_heartbeatMtc.WithLabelValues("packageVersion", version.PackageVersion).Set(1)
		_heartbeatMtc.WithLabelValues("packageCommitID", version.PackageCommitID).Set(1)
		_heartbeatMtc.WithLabelValues("goVersion", version.GoVersion).Set(1)
	}

	// Mem metrics
	memMetrics()
}

func memMetrics() {
	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)
	_heartbeatMtc.WithLabelValues("allocatedHeapObjects", "node").Set(float64(bToMb(memStat.Alloc)))
	_heartbeatMtc.WithLabelValues("totalAllocatedHeapObjects", "node").Set(float64(bToMb(memStat.TotalAlloc)))
	_heartbeatMtc.WithLabelValues("stackInUse", "node").Set(float64(bToMb(memStat.StackInuse)))
	_heartbeatMtc.WithLabelValues("stackFromOS", "node").Set(float64(bToMb(memStat.StackSys)))
	_heartbeatMtc.WithLabelValues("totalFromOS", "node").Set(float64(bToMb(memStat.Sys)))
	_heartbeatMtc.WithLabelValues("heapInUse", "node").Set(float64(bToMb(memStat.HeapInuse)))
	_heartbeatMtc.WithLabelValues("heapFromOS", "node").Set(float64(bToMb(memStat.HeapSys)))
	_heartbeatMtc.WithLabelValues("heapIdle", "node").Set(float64(bToMb(memStat.HeapIdle)))
	_heartbeatMtc.WithLabelValues("heapReleased", "node").Set(float64(bToMb(memStat.HeapReleased)))
	_heartbeatMtc.WithLabelValues("numberOfGC", "node").Set(float64(memStat.NumGC))
	_heartbeatMtc.WithLabelValues("numberOfRoutines", "node").Set(float64(runtime.NumGoroutine()))
}
