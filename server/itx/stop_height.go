// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package itx

import (
	"sync"
	"sync/atomic"
)

type stopHeightController struct {
	target  uint64
	done    chan struct{}
	once    sync.Once
	reached atomic.Uint64
}

func newStopHeightController(target uint64) *stopHeightController {
	if target == 0 {
		return nil
	}
	return &stopHeightController{
		target: target,
		done:   make(chan struct{}),
	}
}

func (c *stopHeightController) markReached(height uint64) {
	if c == nil || height < c.target {
		return
	}
	c.once.Do(func() {
		c.reached.Store(height)
		close(c.done)
	})
}

func (c *stopHeightController) doneCh() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.done
}

func (c *stopHeightController) reachedHeight() uint64 {
	if c == nil {
		return 0
	}
	return c.reached.Load()
}
