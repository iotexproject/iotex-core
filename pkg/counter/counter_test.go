// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package counter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlidingWindowCounter1(t *testing.T) {
	c := NewSlidingWindowCounter(time.Second, 100*time.Millisecond)
	for i := 0; i < 10; i++ {
		c.Increment()
	}
	assert.Equal(t, uint64(10), c.Count())
	assert.Equal(t, uint64(10), c.Count())
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, uint64(10), c.Count())
	time.Sleep(1000 * time.Millisecond)
	assert.Equal(t, uint64(0), c.Count())
}

func TestSlidingWindowCounter2(t *testing.T) {
	c := NewSlidingWindowCounter(time.Second, 100*time.Millisecond)
	for i := 0; i < 15; i++ {
		time.Sleep(110 * time.Millisecond)
		c.Increment()
	}
	assert.Equal(t, uint64(10), c.Count())
	for c.Count() > 0 {
		time.Sleep(100 * time.Millisecond)
	}
}

func TestSlidingWindowCounter3(t *testing.T) {
	c := NewSlidingWindowCounter(time.Second, 100*time.Millisecond)
	for i := 0; i < 150; i++ {
		time.Sleep(10 * time.Millisecond)
		c.Increment()
	}
	for _, slot := range c.window {
		assert.True(t, slot > 0)
	}
}
