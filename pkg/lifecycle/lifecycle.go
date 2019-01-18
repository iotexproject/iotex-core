// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// Package lifecycle provides application models' lifecycle management.
package lifecycle

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Model is application model which may require to start and stop in application lifecycle.
type Model interface{}

// Starter is Model has a Start method.
type Starter interface {
	// Start runs on lifecycle start phase.
	Start(context.Context) error
}

// Stopper is Model has a Stop method.
type Stopper interface {
	// Stop runs on lifecycle stop phase.
	Stop(context.Context) error
}

// StartStopper is the interface that groups Start and Stop.
type StartStopper interface {
	Starter
	Stopper
}

// Lifecycle manages lifecycle for models. Currently a Lifecycle has two phases: Start and Stop.
// Currently Lifecycle doesn't support soft dependency models and multi-err, so all models in Lifecycle require to be
// succeed on both phases.
type Lifecycle struct {
	models []Model
}

// Add adds a model into LifeCycle.
func (lc *Lifecycle) Add(m Model) { lc.models = append(lc.models, m) }

// AddModels adds multiple models into LifeCycle.
func (lc *Lifecycle) AddModels(m ...Model) { lc.models = append(lc.models, m...) }

// OnStart runs models OnStart function if models implmented it. All OnStart functions will be run in parallel.
// context passed into models' OnStart method will be canceled on the first time a model's OnStart function return non-nil error.
func (lc *Lifecycle) OnStart(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, m := range lc.models {
		if starter, ok := m.(Starter); ok {
			g.Go(func() error { return starter.Start(ctx) })
		}
	}
	return g.Wait()
}

// OnStop runs models Start function if models implmented it. All OnStop functions will be run in parallel.
// context passed into models' OnStop method will be canceled on the first time a model's OnStop function return non-nil error.
func (lc *Lifecycle) OnStop(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, m := range lc.models {
		if stopper, ok := m.(Stopper); ok {
			g.Go(func() error { return stopper.Stop(ctx) })
		}
	}
	return g.Wait()
}
