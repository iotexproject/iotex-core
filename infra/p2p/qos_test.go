// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQos(t *testing.T) {
	r := require.New(t)

	now := time.Now()
	q := NewQoS(now, time.Second)

	for _, test := range []struct {
		send, broadcast, success bool
	}{
		{true, true, false},
		{false, false, true},
		{false, true, false},
		{true, true, true},
		{false, false, false},
		{true, false, true},
		{false, true, true},
		{true, false, false},
	} {
		t := time.Now()
		if test.send {
			if test.broadcast {
				q.updateSendBroadcast(t, test.success)
			} else {
				q.updateSendUnicast("test", t, test.success)
			}
		} else {
			if test.broadcast {
				q.updateRecvBroadcast(t)
			} else {
				q.updateRecvUnicast("test", t)
			}
		}
	}

	r.EqualValues(2, q.BroadcastSendTotal())
	r.EqualValues(1, q.broadcastSendSuccess)
	r.EqualValues(2, q.BroadcastRecvTotal())
	c, ok := q.UnicastSendTotal("test")
	r.True(ok)
	r.EqualValues(2, c)
	c, ok = q.UnicastRecvTotal("test")
	r.True(ok)
	r.EqualValues(2, c)
	rate, ok := q.UnicastSendSuccessRate("test")
	r.True(ok)
	r.EqualValues(0.5, rate)
	r.EqualValues(0.5, q.BroadcastSendSuccessRate())
	_, ok = q.UnicastSendTotal("noname")
	r.False(ok)
	_, ok = q.UnicastRecvTotal("noname")
	r.False(ok)
	_, ok = q.UnicastSendSuccessRate("noname")
	r.False(ok)
	r.True(q.lastBroadcastTime().After(now))
	r.True(q.lastUnicastTime().After(now))

	r.False(q.lostConnection())
	time.Sleep(750 * time.Millisecond)
	r.False(q.lostConnection())
	q.updateRecvUnicast("test", time.Now())
	time.Sleep(750 * time.Millisecond)
	r.False(q.lostConnection())
	time.Sleep(250 * time.Millisecond)
	r.True(q.lostConnection())
}
