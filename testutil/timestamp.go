package testutil

import "github.com/facebookgo/clock"

func TimestampNow() uint64 {
	return uint64(clock.New().Now().Unix())
}
