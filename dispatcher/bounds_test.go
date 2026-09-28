// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package dispatcher

import (
	"context"
	"testing"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// An oversized Actions message must be rejected before any signature recovery
// runs: every action costs a full secp256k1 recovery, inline in the pubsub
// validation goroutine.
func TestValidateMessageBoundsActionCount(t *testing.T) {
	r := require.New(t)

	verified := 0
	d, err := NewDispatcher(DefaultConfig, func(proto.Message) (string, error) {
		verified++
		return "sender", nil
	})
	r.NoError(err)
	dp := d.(*IotxDispatcher)

	oversized := &iotextypes.Actions{
		Actions: make([]*iotextypes.Action, maxActionsPerMessage+1),
	}
	ignore, err := dp.ValidateMessage(oversized)
	r.True(ignore)
	r.ErrorIs(err, ErrMsgTooLarge)
	r.Zero(verified, "no action may be verified before the count is bounded")

	// a message at the limit is still processed normally
	atLimit := &iotextypes.Actions{
		Actions: make([]*iotextypes.Action, maxActionsPerMessage),
	}
	_, err = dp.ValidateMessage(atLimit)
	r.NoError(err)
	r.Equal(maxActionsPerMessage, verified)
}

// An ActionSync request must not turn one small unicast into an unbounded
// number of pool lookups and reflected sends on the single misc worker.
func TestDispatchMsgBoundsActionSyncHashes(t *testing.T) {
	r := require.New(t)

	d, err := NewDispatcher(DefaultConfig, func(proto.Message) (string, error) { return "", nil })
	r.NoError(err)
	dp := d.(*IotxDispatcher)
	sub := &counterSubscriber{}
	dp.AddSubscriber(defaultChainID, sub)

	hashes := func(n int) [][]byte {
		hs := make([][]byte, n)
		for i := range hs {
			hs[i] = make([]byte, 32)
			hs[i][0] = byte(i)
		}
		return hs
	}
	peerInfo := &peer.AddrInfo{}

	dp.dispatchMsg(&message{
		ctx:      context.Background(),
		chainID:  defaultChainID,
		msg:      &iotexrpc.ActionSync{Hashes: hashes(maxActionSyncHashes + 1)},
		peerInfo: peerInfo,
	})
	r.Zero(sub.actionReq.Load(), "an oversized request must be dropped whole")

	dp.dispatchMsg(&message{
		ctx:      context.Background(),
		chainID:  defaultChainID,
		msg:      &iotexrpc.ActionSync{Hashes: hashes(maxActionSyncHashes)},
		peerInfo: peerInfo,
	})
	r.EqualValues(maxActionSyncHashes, sub.actionReq.Load())
}

// A block-request slot must be released on every path that does not enqueue.
// Otherwise a peer whose requests are dropped leaks a slot per drop and is
// locked out of block sync for good after blockSyncLimit drops.
func TestQueueReleasesBlockRequestSlotOnDrop(t *testing.T) {
	r := require.New(t)

	// blockSync queue of size 1 so the second request is dropped
	m := newMsgQueueMgr(msgQueueConfig{blockSyncSize: 1, blockSyncLimit: 2}, func(*message) {})
	sub := &dummySubscriber{}
	msg := func() *message {
		return &message{msgType: iotexrpc.MessageType_BLOCK_REQUEST, peer: "honest"}
	}

	r.True(m.Queue(msg(), sub))  // enqueued, holds one slot
	r.False(m.Queue(msg(), sub)) // queue full -> dropped, slot must be released
	r.False(m.Queue(msg(), sub)) // still dropped, still released
	r.Equal(1, m.blockRequestPeers.counts["honest"], "only the queued request holds a slot")

	// drain as consume() would, then the peer must still be servable
	<-m.queues[blockSyncQ]
	m.blockRequestPeers.decrement("honest")
	r.True(m.Queue(msg(), sub))
}

// A subscriber-filtered block request must release its slot too.
func TestQueueReleasesBlockRequestSlotOnFilter(t *testing.T) {
	r := require.New(t)

	m := newMsgQueueMgr(msgQueueConfig{blockSyncSize: 10, blockSyncLimit: 2}, func(*message) {})
	msg := &message{msgType: iotexrpc.MessageType_BLOCK_REQUEST, peer: "honest"}

	for range 5 {
		r.False(m.Queue(msg, &filterAllSubscriber{}))
	}
	r.NotContains(m.blockRequestPeers.counts, "honest")
}

// decrement must drop zeroed keys, or the map keeps an entry for every peer
// that ever sent a block request.
func TestQueueLimitDropsZeroedKeys(t *testing.T) {
	r := require.New(t)

	q := &queueLimit{counts: map[string]int{}, limit: 2}
	r.True(q.increment("p"))
	r.True(q.increment("p"))
	r.Equal(2, q.counts["p"])
	q.decrement("p")
	q.decrement("p")
	r.Empty(q.counts)
	// decrementing an absent peer must not resurrect the key
	q.decrement("gone")
	r.Empty(q.counts)
}

// peerLastSync must not grow without bound: entries outside the throttle window
// carry no information.
func TestFilterSweepsPeerLastSync(t *testing.T) {
	r := require.New(t)

	d := &IotxDispatcher{
		peerLastSync: make(map[string]time.Time),
		syncInterval: time.Millisecond,
	}
	stale := time.Now().Add(-time.Hour)
	for i := range maxPeerLastSyncEntries {
		d.peerLastSync[string(rune(i))] = stale
	}
	r.Len(d.peerLastSync, maxPeerLastSyncEntries)

	r.True(d.filter(&message{msgType: iotexrpc.MessageType_BLOCK_REQUEST, peer: "fresh"}))
	r.Len(d.peerLastSync, 1)
}

type filterAllSubscriber struct{ dummySubscriber }

func (f *filterAllSubscriber) Filter(iotexrpc.MessageType, proto.Message, int) bool { return false }
