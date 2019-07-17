// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package prometheustimer

import (
	"errors"

	"github.com/facebookgo/clock"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	// TimerFactory defines a timer factory to generate timer
	TimerFactory struct {
		labelNames    []string
		defaultLabels []string
		vect          *prometheus.GaugeVec
		clk           clock.Clock
	}
	// Timer defines a timer to measure performance
	Timer struct {
		factory   *TimerFactory
		labels    []string
		startTime int64
	}
)

// New returns a new Timer
func New(name, tip string, labelNames []string, defaultLabels []string) (*TimerFactory, error) {
	if len(labelNames) != len(defaultLabels) {
		return nil, errors.New("label names do not match default labels")
	}
	vect := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: tip,
		},
		labelNames,
	)
	err := prometheus.Register(vect)
	if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
		err = nil
	}

	return &TimerFactory{
		labelNames:    labelNames,
		defaultLabels: defaultLabels,
		vect:          vect,
		clk:           clock.New(),
	}, err
}

// NewTimer returns a timer with start time as now
func (factory *TimerFactory) NewTimer(labels ...string) *Timer {
	if factory == nil {
		return &Timer{}
	}
	if len(labels) > len(factory.labelNames) {
		log.L().Error("Two many timer labels")
		return &Timer{}
	}
	return &Timer{
		factory:   factory,
		labels:    labels,
		startTime: factory.now(),
	}
}

// End ends the timer
func (timer *Timer) End() {
	f := timer.factory
	if f == nil {
		return
	}
	f.log(float64(f.now()-timer.startTime), timer.labels...)
}

func (factory *TimerFactory) log(value float64, labels ...string) {
	factory.vect.WithLabelValues(
		append(labels, factory.defaultLabels[len(labels):]...)...,
	).Set(value)
}

func (factory *TimerFactory) now() int64 {
	return factory.clk.Now().UnixNano()
}
