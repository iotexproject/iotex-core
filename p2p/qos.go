// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"sync/atomic"
	"time"
)

type (
	qos struct {
		broadcastSendCount    uint64
		broadcastSendSuccess  uint64
		broadcastRecvCount    uint64
		lastActiveBroadcastTs int64 // in nano-second
		lastActiveUnicastTs   int64 // in nano-second
		timeout               time.Duration
		metrics               map[string]*transmitMetric
	}

	transmitMetric struct {
		unicastSendCount   uint64
		unicastSendSuccess uint64
		unicastRecvCount   uint64
	}
)

func newQoS(now time.Time, timeout time.Duration) *qos {
	return &qos{
		lastActiveBroadcastTs: now.UnixNano(),
		lastActiveUnicastTs:   now.UnixNano(),
		timeout:               timeout,
		metrics:               make(map[string]*transmitMetric),
	}
}

func (q *qos) lostConnection() bool {
	t := time.Now()
	return q.lastBroadcastTime().Add(q.timeout).Before(t) && q.lastUnicastTime().Add(q.timeout).Before(t)
}

func (q *qos) lastBroadcastTime() time.Time {
	return time.Unix(0, atomic.LoadInt64(&q.lastActiveBroadcastTs))
}

func (q *qos) lastUnicastTime() time.Time {
	return time.Unix(0, atomic.LoadInt64(&q.lastActiveUnicastTs))
}

func (q *qos) updateSendBroadcast(t time.Time, success bool) {
	atomic.AddUint64(&q.broadcastSendCount, 1)
	if success {
		atomic.AddUint64(&q.broadcastSendSuccess, 1)
		atomic.StoreInt64(&q.lastActiveBroadcastTs, t.UnixNano())
	}
}

func (q *qos) updateRecvBroadcast(t time.Time) {
	atomic.AddUint64(&q.broadcastRecvCount, 1)
	atomic.StoreInt64(&q.lastActiveBroadcastTs, t.UnixNano())
}

func (q *qos) updateSendUnicast(peername string, t time.Time, success bool) {
	peer, exist := q.metrics[peername]
	if !exist {
		peer = new(transmitMetric)
		q.metrics[peername] = peer
	}
	atomic.AddUint64(&peer.unicastSendCount, 1)
	if success {
		atomic.AddUint64(&peer.unicastSendSuccess, 1)
		atomic.StoreInt64(&q.lastActiveUnicastTs, t.UnixNano())
	}
}

func (q *qos) updateRecvUnicast(peername string, t time.Time) {
	peer, exist := q.metrics[peername]
	if !exist {
		peer = new(transmitMetric)
		q.metrics[peername] = peer
	}
	atomic.AddUint64(&peer.unicastRecvCount, 1)
	atomic.StoreInt64(&q.lastActiveUnicastTs, t.UnixNano())
}

func (q *qos) broadcastSendTotal() uint64 {
	return atomic.LoadUint64(&q.broadcastSendCount)
}

func (q *qos) broadcastSendSuccessRate() float64 {
	success := atomic.LoadUint64(&q.broadcastSendSuccess)
	total := atomic.LoadUint64(&q.broadcastSendCount)
	return float64(success) / float64(total)
}

func (q *qos) broadcastRecvTotal() uint64 {
	return atomic.LoadUint64(&q.broadcastRecvCount)
}

func (q *qos) unicastSendCount(peername string) (uint64, bool) {
	peer, exist := q.metrics[peername]
	if !exist {
		return 0, false
	}
	return atomic.LoadUint64(&peer.unicastSendCount), true
}

func (q *qos) unicastSendSuccessRate(peername string) (float64, bool) {
	peer, exist := q.metrics[peername]
	if !exist {
		return 0, false
	}
	success := atomic.LoadUint64(&peer.unicastSendSuccess)
	total := atomic.LoadUint64(&peer.unicastSendCount)
	return float64(success) / float64(total), true
}

func (q *qos) unicastRecvCount(peername string) (uint64, bool) {
	peer, exist := q.metrics[peername]
	if !exist {
		return 0, false
	}
	return atomic.LoadUint64(&peer.unicastRecvCount), true
}
