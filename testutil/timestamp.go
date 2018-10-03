package testutil

import "github.com/facebookgo/clock"

// get now timestamp from new clock
func TimestampNow() uint64 {
	return TimestampNowFromClock(clock.New())
}


// get now timestamp from specific clock
func TimestampNowFromClock(c clock.Clock) uint64 {
	return uint64(c.Now().Unix())
}
