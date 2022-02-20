// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine

import (
	"context"
	"time"

	"github.com/facebookgo/clock"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var _ lifecycle.StartStopper = (*RecurringTask)(nil)

// RecurringTaskOption is option to RecurringTask.
type RecurringTaskOption interface {
	SetRecurringTaskOption(*RecurringTask)
}

// RecurringTask represents a recurring task
type RecurringTask struct {
	t        Task
	interval time.Duration
	ticker   *clock.Ticker
	done     chan struct{}
	clock    clock.Clock
}

// NewRecurringTask creates an instance of RecurringTask
func NewRecurringTask(t Task, i time.Duration, ops ...RecurringTaskOption) *RecurringTask {
	rt := &RecurringTask{
		t:        t,
		interval: i,
		done:     make(chan struct{}),
		clock:    clock.New(),
	}
	for _, opt := range ops {
		opt.SetRecurringTaskOption(rt)
	}
	return rt
}

// Start starts the timer
func (t *RecurringTask) Start(_ context.Context) error {
	t.ticker = t.clock.Ticker(t.interval)
	go func() {
		for {
			select {
			case <-t.done:
				return
			case <-t.ticker.C:
				t.t()
			}
		}
	}()
	return nil
}

// Stop stops the timer
func (t *RecurringTask) Stop(_ context.Context) error {
	// prevent stop is called before start.
	if t.ticker != nil {
		t.ticker.Stop()
	}
	close(t.done)
	return nil
}
