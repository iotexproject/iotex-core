// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"time"

	"github.com/pkg/errors"
)

// ErrTimeout is returned when time is up
var ErrTimeout = errors.New("timed out")

// CheckCondition defines a func type that checks whether a certain condition is satisfied
type CheckCondition func() (bool, error)

// SignalChan returns a channel that will be written every interval until timeout
func SignalChan(interval, timeout time.Duration) <-chan struct{} {
	ch := make(chan struct{})

	go func() {
		defer close(ch)
		tick := time.NewTicker(interval)
		defer tick.Stop()

		var after <-chan time.Time
		if timeout != 0 {
			timer := time.NewTimer(timeout)
			after = timer.C
			defer timer.Stop()
		}
		// Signal channel every interval time period
		for {
			select {
			case <-tick.C:
				select {
				case ch <- struct{}{}:
				default:
				}
			// Timeout
			case <-after:
				return
			}
		}
	}()
	return ch
}

// WaitUntil periodically checks whether the condition specified in CheckCondition function is satisfied
// If an error is returned, it either comes from CheckCondition function or time is up before the given
// condition is satisfied
func WaitUntil(interval, timeout time.Duration, f CheckCondition) error {
	ch := SignalChan(interval, timeout)
	for {
		_, open := <-ch
		satisfied, err := f()
		if satisfied {
			return nil
		}
		if err != nil {
			return err
		}
		// If channel is closed, the maximum waiting time is reached and a timeout error is returned
		if !open {
			return ErrTimeout
		}
	}
}
