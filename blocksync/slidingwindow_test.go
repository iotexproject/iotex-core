package blocksync

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSlidingWindow(t *testing.T) {
	assert := assert.New(t)

	sw := NewSlidingWindow()
	assert.True(sw.State == Open)

	// close must be < open
	err := sw.SetRange(12, 5)
	assert.NotNil(err)
	err = sw.SetRange(12, 12)
	assert.NotNil(err)
	close := uint64(1)
	open := uint64(12)
	err = sw.SetRange(close, open)
	assert.Nil(err)

	// update with any value in [close, open] and != close+1 should have no effect
	sw.Update(close + 2)
	assert.Equal(close, sw.close)
	assert.Equal(open, sw.open)
	sw.Update(close)
	assert.Equal(close, sw.close)
	assert.Equal(open, sw.open)
	sw.Update(open)
	assert.Equal(close, sw.close)
	assert.Equal(open, sw.open)

	// open - WindowSize + 1 is where the windows turns to closing
	closing := open - WindowSize + 1
	for i := sw.close + 1; i <= closing; i++ {
		sw.Update(i)
		assert.Equal(i, sw.close)
	}
	assert.True(sw.State == Closing)

	// update with value < close should have no effect
	sw.Update(sw.close - 1)
	assert.Equal(closing, sw.close)
	assert.Equal(open, sw.open)
	assert.True(sw.State == Closing)

	// update with value > open should open the window more
	open = sw.open + 1
	sw.Update(open)
	assert.Equal(closing, sw.close)
	assert.Equal(open, sw.open)
	assert.True(sw.State == Open)

	// close the window
	for i := closing + 1; i < sw.open; i++ {
		sw.Update(i)
		assert.Equal(i, sw.close)
	}
	assert.True(sw.State == Closed)

	// after window closes, each close increments both the open and close value
	for i := 0; i < 5; i++ {
		close = sw.close + 1
		sw.Update(close)
		assert.Equal(close, sw.close)
		assert.Equal(close+1, sw.open)
		assert.True(sw.State == Closed)
	}
}
