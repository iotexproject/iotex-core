// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package counter

import (
	"sync"
	"time"
)

// SlidingWindowCounter is used to count the number of events happened in the last X duration (in terms of a sliding
// window). Interval defines how big the time window is and SlotGranularity defines how fine grained the counter is.
type SlidingWindowCounter struct {
	Interval        time.Duration
	SlotGranularity time.Duration
	window          []uint64
	count           uint64
	headIdx         int
	lastUpdateTime  time.Time
	locker          sync.Mutex
}

// NewSlidingWindowCounter creates an instance of SlidingWindowCounter
func NewSlidingWindowCounter(i time.Duration, sg time.Duration) *SlidingWindowCounter {
	c := &SlidingWindowCounter{Interval: i, SlotGranularity: sg}
	c.window = make([]uint64, i/sg)
	c.count = 0
	c.headIdx = 0
	c.lastUpdateTime = time.Now()
	return c
}

// NewSlidingWindowCounterWithSecondSlot creates an instance of SlidingWindowCounter with the second level slot
func NewSlidingWindowCounterWithSecondSlot(i time.Duration) *SlidingWindowCounter {
	return NewSlidingWindowCounter(i, time.Second)
}

// Increment increase the counter by 1. It's a blocking operation.
func (c *SlidingWindowCounter) Increment() {
	c.locker.Lock()
	defer c.locker.Unlock()

	c.refresh()
	c.window[c.headIdx]++
	c.count++
}

// Count reads the current gauge. It's a blocking operation.
func (c *SlidingWindowCounter) Count() uint64 {
	c.locker.Lock()
	defer c.locker.Unlock()

	c.refresh()
	return c.count
}

func (c *SlidingWindowCounter) refresh() {
	now := time.Now()
	duration := int(now.Sub(c.lastUpdateTime) / c.SlotGranularity)
	if duration >= len(c.window) {
		for i := 0; i < len(c.window); i++ {
			if i == 0 {
				c.window[i] = 1
			} else {
				c.window[i] = 0
			}
		}
		c.headIdx = 0
		c.count = 0

	} else {
		for i := 0; i < duration; i++ {
			c.headIdx++
			if c.headIdx >= len(c.window) {
				c.headIdx = 0
			}
			c.count -= c.window[c.headIdx]
			c.window[c.headIdx] = 0
		}
	}
	// Only change the lastUpdateTime when duration is greater than 0. That said, lastUpdateTime is updated only when
	// the delta is greater than the slog granularity. This is to prevent keep updating the lastUpdateTime if incoming
	// messages is so frequent that now - lastUpdateTime is always smaller than slog granularity, eventually always
	// increasing the counter in the same slot.
	if duration > 0 {
		c.lastUpdateTime = now
	}
}
