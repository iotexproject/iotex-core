// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import "github.com/prometheus/client_golang/prometheus"

var (
	_iip59DurationMtc = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "iotex_rewarding_iip59_duration_seconds",
			Help: "Time spent in IIP-59 reward distribution operations.",
			Buckets: []float64{
				0.00001, 0.000025, 0.00005, 0.0001, 0.00025, 0.0005,
				0.001, 0.0025, 0.005, 0.01, 0.025, 0.05,
				0.1, 0.25, 0.5, 1, 1.5, 2, 2.5, 5, 10,
			},
		},
		[]string{"operation"},
	)
	_iip59ItemsMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_rewarding_iip59_items_total",
			Help: "Number of items processed by IIP-59 reward distribution operations.",
		},
		[]string{"type"},
	)
	// A failed drain chunk is not a failed block: the block still commits, with
	// a Failure receipt, and the cursor is left exactly where it was. That is
	// deliberate (degrade the item, never abort the block) but it makes the
	// failure invisible from chain data alone -- the next era boundary's
	// writeVoterRewardDistributionState replaces both plan and progress, so a chunk that
	// keeps failing quietly discards an era of voter payouts at the boundary.
	// These two are the only signal an operator gets before that happens.
	_iip59DrainChunkFailureMtc = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iotex_rewarding_iip59_drain_chunk_failures_total",
			Help: "Number of IIP-59 voter-reward chunk actions that failed and settled with a Failure receipt.",
		},
	)
	// Read next to the counter and the resume-voter field in the error log. The
	// phase distinguishes the tail scan, wrapped head scan, and completion.
	_iip59DrainStalledScanPhaseMtc = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "iotex_rewarding_iip59_drain_stalled_scan_phase",
			Help: "Voter address scan phase of the most recent failed voter-reward chunk.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		_iip59DurationMtc, _iip59ItemsMtc,
		_iip59DrainChunkFailureMtc, _iip59DrainStalledScanPhaseMtc,
	)
}

// noteIIP59DrainChunkFailure counts one failed chunk. scanPhase is the cursor
// position it failed at; hasCursor is false when the cursor could not be read,
// in which case the position gauge is left at its previous value rather than
// being reset to a misleading zero.
func noteIIP59DrainChunkFailure(scanPhase voterScanPhase, hasCursor bool) {
	_iip59DrainChunkFailureMtc.Inc()
	if hasCursor {
		_iip59DrainStalledScanPhaseMtc.Set(float64(scanPhase))
	}
}

func startIIP59Duration(operation string) func() {
	timer := prometheus.NewTimer(_iip59DurationMtc.WithLabelValues(operation))
	return func() {
		timer.ObserveDuration()
	}
}

func addIIP59Items(itemType string, count int) {
	if count > 0 {
		_iip59ItemsMtc.WithLabelValues(itemType).Add(float64(count))
	}
}

type iip59RouteDurations struct {
	autoDepositLookup float64
	nativeBucketRead  float64
	compoundDeposit   float64
	destinationLookup float64
	directCredit      float64
}

func (d *iip59RouteDurations) observe() {
	observeIIP59AccumulatedDuration("auto_deposit_lookup", d.autoDepositLookup)
	observeIIP59AccumulatedDuration("native_bucket_read", d.nativeBucketRead)
	observeIIP59AccumulatedDuration("compound_deposit", d.compoundDeposit)
	observeIIP59AccumulatedDuration("reward_destination_lookup", d.destinationLookup)
	observeIIP59AccumulatedDuration("direct_credit", d.directCredit)
}

func observeIIP59AccumulatedDuration(operation string, duration float64) {
	if duration > 0 {
		_iip59DurationMtc.WithLabelValues(operation).Observe(duration)
	}
}

func startIIP59Accumulation(duration *float64) func() {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(elapsed float64) {
		*duration += elapsed
	}))
	return func() {
		timer.ObserveDuration()
	}
}
