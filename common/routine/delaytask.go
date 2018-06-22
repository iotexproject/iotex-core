// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine

import (
	"time"

	"github.com/iotexproject/iotex-core/common/service"
)

// DelayTaskCB implements the timeout task business logic
type DelayTaskCB func()

// DelayTask represents a timeout task
type DelayTask struct {
	service.AbstractService
	cb       DelayTaskCB
	Duration time.Duration
	ch       chan interface{}
}

// NewDelayTask creates an instance of DelayTask
func NewDelayTask(cb DelayTaskCB, d time.Duration) *DelayTask {
	return &DelayTask{
		cb:       cb,
		Duration: d,
		ch:       make(chan interface{}, 1),
	}
}

// Start starts the timeout
func (t *DelayTask) Start() error {
	go func() {
		select {
		case <-t.ch:
			return
		case <-time.After(t.Duration):
			t.cb()
		}
	}()

	return nil
}

// Stop stops the timeout
func (t *DelayTask) Stop() error {
	t.ch <- struct{}{}
	return nil
}
