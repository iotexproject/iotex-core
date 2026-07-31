// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"encoding/hex"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// Anomaly kinds. Each names a state the incremental VoterWeightView path can
// reach only if a weight-mutating site upstream is missing its hook or is
// computing the wrong delta.
const (
	// _vwAnomalyUnknownCandidate: a negative delta arrived for a candidate the
	// view has never seen.
	_vwAnomalyUnknownCandidate = "negative_delta_unknown_candidate"
	// _vwAnomalyUnknownVoter: a negative delta arrived for a voter the
	// candidate has no entry for.
	_vwAnomalyUnknownVoter = "negative_delta_unknown_voter"
	// _vwAnomalyUnderflow: a delta drove a voter's weight strictly below zero,
	// i.e. more was subtracted than was ever added.
	_vwAnomalyUnderflow = "weight_underflow"
	// _vwAnomalySeedOnDirtyOverlay: the activation seeding flush listed pairs
	// from an overlay that already held deltas, so the list may not reflect the
	// layer it was asked about. Seeding runs before any action in the block, so
	// this means it was called from the wrong place.
	_vwAnomalySeedOnDirtyOverlay = "seed_on_dirty_overlay"
)

var _voterWeightAnomalyMtc = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "iotex_staking_voter_weight_anomaly_total",
		Help: "Voter weight deltas that could not be applied consistently. " +
			"Any non-zero value means the IIP-59 incremental path has a bug.",
	},
	[]string{"kind"},
)

func init() {
	prometheus.MustRegister(_voterWeightAnomalyMtc)
}

// voterWeightAnomalyFatal makes anomalies crash the process instead of only
// being counted. Tests set it so a mismatch fails the run at its cause;
// production must leave it false, because the metric and log path is purely
// observational and must never change the consensus result.
var voterWeightAnomalyFatal = false

// reportVoterWeightAnomaly records an inconsistency without altering the
// outcome of the operation that hit it. The counter should sit at zero forever;
// alert on any increase.
func reportVoterWeightAnomaly(kind string, candID hash.Hash160, voter address.Address, delta *big.Int) {
	_voterWeightAnomalyMtc.WithLabelValues(kind).Inc()
	voterAddr := "<nil>"
	if voter != nil {
		voterAddr = voter.String()
	}
	deltaStr := "<nil>"
	if delta != nil {
		deltaStr = delta.String()
	}
	log.L().Error("voter weight view anomaly",
		zap.String("kind", kind),
		zap.String("candidate", hex.EncodeToString(candID[:])),
		zap.String("voter", voterAddr),
		zap.String("delta", deltaStr),
	)
	if voterWeightAnomalyFatal {
		panic("voter weight view anomaly: " + kind)
	}
}
