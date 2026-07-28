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
)

func init() {
	prometheus.MustRegister(_iip59DurationMtc, _iip59ItemsMtc)
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
	directCredit      float64
}

func (d *iip59RouteDurations) observe() {
	observeIIP59AccumulatedDuration("auto_deposit_lookup", d.autoDepositLookup)
	observeIIP59AccumulatedDuration("native_bucket_read", d.nativeBucketRead)
	observeIIP59AccumulatedDuration("compound_deposit", d.compoundDeposit)
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
