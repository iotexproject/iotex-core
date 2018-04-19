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

// ITimeoutTaskHandler is the interface to implement the timeout task business logic
type ITimeoutTaskHandler interface {
	// Do is called on constant interval
	Do()
}

// TimeoutTask represents a timeout task
type TimeoutTask struct {
	service.AbstractService
	H        ITimeoutTaskHandler
	Duration time.Duration
	ch       chan interface{}
}

// NewTimeoutTask creates an instance of TimeoutTask
func NewTimeoutTask(h ITimeoutTaskHandler, d time.Duration) *TimeoutTask {
	return &TimeoutTask{
		H:        h,
		Duration: d,
		ch:       make(chan interface{}, 1),
	}
}

// Start starts the timeout
func (t *TimeoutTask) Start() error {
	go func() {
		select {
		case <-t.ch:
			return
		case <-time.After(t.Duration):
			t.H.Do()
		}
	}()

	return nil
}

// Stop stops the timeout
func (t *TimeoutTask) Stop() error {
	t.ch <- struct{}{}
	return nil
}
