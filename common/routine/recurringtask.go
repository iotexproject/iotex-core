// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine

import (
	"time"

	"github.com/iotexproject/iotex-core/common/service"
)

// IRecurringTaskHandler is the interface to implement the recurring task business logic
type IRecurringTaskHandler interface {
	// Do is called on constant interval
	Do()
}

// RecurringTask represents a recurring task
type RecurringTask struct {
	service.AbstractService
	H        IRecurringTaskHandler
	Interval time.Duration
	ticker   *time.Ticker
}

// NewRecurringTask creates an instance of RecurringTask
func NewRecurringTask(h IRecurringTaskHandler, i time.Duration) *RecurringTask {
	return &RecurringTask{H: h, Interval: i}
}

// Start starts the timer
func (t *RecurringTask) Start() error {
	t.ticker = time.NewTicker(t.Interval)
	go func() {
		for range t.ticker.C {
			t.H.Do()
		}
	}()
	return nil
}

// Stop stops the timer
func (t *RecurringTask) Stop() error {
	t.ticker.Stop()
	return nil
}
