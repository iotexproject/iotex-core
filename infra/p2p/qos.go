// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import (
	"sync"
	"sync/atomic"
	"time"
)

type (
	// Qos metrics
	Qos struct {
		lock                  sync.RWMutex
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

// NewQoS returns the Qos metrics
func NewQoS(now time.Time, timeout time.Duration) *Qos {
	return &Qos{
		lastActiveBroadcastTs: now.UnixNano(),
		lastActiveUnicastTs:   now.UnixNano(),
		timeout:               timeout,
		metrics:               make(map[string]*transmitMetric),
	}
}

func (q *Qos) lostConnection() bool {
	t := time.Now()
	return q.lastBroadcastTime().Add(q.timeout).Before(t) && q.lastUnicastTime().Add(q.timeout).Before(t)
}

func (q *Qos) lastBroadcastTime() time.Time {
	return time.Unix(0, atomic.LoadInt64(&q.lastActiveBroadcastTs))
}

func (q *Qos) lastUnicastTime() time.Time {
	return time.Unix(0, atomic.LoadInt64(&q.lastActiveUnicastTs))
}

func (q *Qos) updateSendBroadcast(t time.Time, success bool) {
	atomic.AddUint64(&q.broadcastSendCount, 1)
	if success {
		atomic.AddUint64(&q.broadcastSendSuccess, 1)
		atomic.StoreInt64(&q.lastActiveBroadcastTs, t.UnixNano())
	}
}

func (q *Qos) updateRecvBroadcast(t time.Time) {
	atomic.AddUint64(&q.broadcastRecvCount, 1)
	atomic.StoreInt64(&q.lastActiveBroadcastTs, t.UnixNano())
}

func (q *Qos) updateSendUnicast(peername string, t time.Time, success bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	peer, exist := q.metrics[peername]
	if !exist {
		peer = new(transmitMetric)
		q.metrics[peername] = peer
	}
	peer.unicastSendCount++
	if success {
		peer.unicastSendSuccess++
		q.lastActiveUnicastTs = t.UnixNano()
	}
}

func (q *Qos) updateRecvUnicast(peername string, t time.Time) {
	q.lock.Lock()
	defer q.lock.Unlock()
	peer, exist := q.metrics[peername]
	if !exist {
		peer = new(transmitMetric)
		q.metrics[peername] = peer
	}
	peer.unicastRecvCount++
	q.lastActiveUnicastTs = t.UnixNano()
}

// BroadcastSendTotal returns the total amount of broadcast sent
func (q *Qos) BroadcastSendTotal() uint64 {
	return atomic.LoadUint64(&q.broadcastSendCount)
}

// BroadcastSendSuccessRate returns the broadcast send success rate
func (q *Qos) BroadcastSendSuccessRate() float64 {
	success := atomic.LoadUint64(&q.broadcastSendSuccess)
	total := atomic.LoadUint64(&q.broadcastSendCount)
	return float64(success) / float64(total)
}

// BroadcastRecvTotal returns the total amount of broadcast received
func (q *Qos) BroadcastRecvTotal() uint64 {
	return atomic.LoadUint64(&q.broadcastRecvCount)
}

// UnicastSendTotal returns the total amount of unicast sent to peer
func (q *Qos) UnicastSendTotal(peername string) (uint64, bool) {
	q.lock.RLock()
	peer, exist := q.metrics[peername]
	if !exist {
		q.lock.RUnlock()
		return 0, false
	}
	q.lock.RUnlock()
	return atomic.LoadUint64(&peer.unicastSendCount), true
}

// UnicastSendSuccessRate returns the unicast send success rate
func (q *Qos) UnicastSendSuccessRate(peername string) (float64, bool) {
	q.lock.RLock()
	peer, exist := q.metrics[peername]
	if !exist {
		q.lock.RUnlock()
		return 0, false
	}
	q.lock.RUnlock()
	success := atomic.LoadUint64(&peer.unicastSendSuccess)
	total := atomic.LoadUint64(&peer.unicastSendCount)
	return float64(success) / float64(total), true
}

// UnicastRecvTotal returns the total amount of unicast received from peer
func (q *Qos) UnicastRecvTotal(peername string) (uint64, bool) {
	q.lock.RLock()
	peer, exist := q.metrics[peername]
	if !exist {
		q.lock.RUnlock()
		return 0, false
	}
	q.lock.RUnlock()
	return atomic.LoadUint64(&peer.unicastRecvCount), true
}
