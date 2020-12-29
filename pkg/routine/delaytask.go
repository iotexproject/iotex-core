// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package routine

import (
	"context"
	"time"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var _ lifecycle.StartStopper = (*DelayTask)(nil)

// DelayTask represents a timeout task
type DelayTask struct {
	cb       Task
	duration time.Duration
	ch       chan interface{}
}

// NewDelayTask creates an instance of DelayTask
func NewDelayTask(cb Task, d time.Duration) *DelayTask {
	dt := &DelayTask{
		cb:       cb,
		duration: d,
		ch:       make(chan interface{}, 1),
	}
	return dt
}

// Start executes the delayed task after given timeout.
func (t *DelayTask) Start(ctx context.Context) error {
	ready := make(chan struct{})
	go func() {
		close(ready)
		select {
		case <-ctx.Done():
			return
		case <-t.ch:
			return
		case <-time.After(t.duration):
			t.cb()
		}
	}()

	<-ready
	return nil
}

// Stop stops the timeout
func (t *DelayTask) Stop(ctx context.Context) error {
	t.ch <- struct{}{}
	return nil
}
