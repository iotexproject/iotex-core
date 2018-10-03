package testutil

import "github.com/facebookgo/clock"

func TimestampNow() uint64 {
	c := clock.New()
	return TimestampNowFromClock(c)
}

func TimestampNowFromClock(c clock.Clock) uint64 {
	return uint64(c.Now().Unix())
}
