// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import "github.com/facebookgo/clock"

// TimestampNow get now timestamp from new clock
func TimestampNow() uint64 {
	return TimestampNowFromClock(clock.New())
}

// TimestampNowFromClock get now timestamp from specific clock
func TimestampNowFromClock(c clock.Clock) uint64 {
	return uint64(c.Now().Unix())
}
