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
	ch       chan interface{}
	clock    clock.Clock
}

// NewRecurringTask creates an instance of RecurringTask
func NewRecurringTask(t Task, i time.Duration, ops ...RecurringTaskOption) *RecurringTask {
	rt := &RecurringTask{
		t:        t,
		interval: i,
		ch:       make(chan interface{}, 1),
		clock:    clock.New(),
	}
	for _, opt := range ops {
		opt.SetRecurringTaskOption(rt)
	}
	return rt
}

// Start starts the timer
func (t *RecurringTask) Start(ctx context.Context) error {
	t.ticker = t.clock.Ticker(t.interval)
	ready := make(chan struct{})
	go func() {
		close(ready)
		for {
			select {
			// TODO (soy) we can not cancel on ctx.Done, seems there is something cause context timeout of recurring task unexpected
			case <-t.ch:
				return
			case <-t.ticker.C:
				t.t()
			}
		}
	}()

	<-ready
	return nil
}

// Stop stops the timer
func (t *RecurringTask) Stop(_ context.Context) error {
	// TODO: actually this happens when stop is called before init/start. We should prevent this from happening
	if t.ticker != nil {
		t.ticker.Stop()
	}
	t.ch <- struct{}{}
	return nil
}
