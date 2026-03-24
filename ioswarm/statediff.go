package ioswarm

import (
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// StateDiffEntry represents a single state mutation (Put or Delete).
type StateDiffEntry struct {
	Type      uint8  // 0=Put, 1=Delete
	Namespace string
	Key       []byte
	Value     []byte
}

// StateDiff contains all state changes for a single block.
type StateDiff struct {
	Height      uint64
	Entries     []StateDiffEntry
	DigestBytes []byte // raw SerializeQueue output hash for verification
}

// StateDiffBroadcaster buffers recent state diffs and fans them out to subscribers.
type StateDiffBroadcaster struct {
	mu          sync.RWMutex
	buffer      []*StateDiff // rolling window
	maxBuffer   int
	head        int // ring buffer write position
	count       int // number of items in buffer
	subscribers map[string]*diffSubscriber
	logger      *zap.Logger
}

type diffSubscriber struct {
	ch      chan *StateDiff
	dropped atomic.Int64
}

// NewStateDiffBroadcaster creates a new broadcaster with the given buffer size.
func NewStateDiffBroadcaster(maxBuffer int, logger *zap.Logger) *StateDiffBroadcaster {
	if maxBuffer <= 0 {
		maxBuffer = 100
	}
	return &StateDiffBroadcaster{
		buffer:      make([]*StateDiff, maxBuffer),
		maxBuffer:   maxBuffer,
		subscribers: make(map[string]*diffSubscriber),
		logger:      logger,
	}
}

// Publish adds a state diff to the ring buffer and fans out to all subscribers.
func (b *StateDiffBroadcaster) Publish(diff *StateDiff) {
	b.mu.Lock()
	// Write to ring buffer
	b.buffer[b.head] = diff
	b.head = (b.head + 1) % b.maxBuffer
	if b.count < b.maxBuffer {
		b.count++
	}
	// Copy subscriber refs under lock
	subs := make([]*diffSubscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subs = append(subs, sub)
	}
	b.mu.Unlock()

	// Fan out to subscribers (non-blocking)
	for _, sub := range subs {
		select {
		case sub.ch <- diff:
		default:
			// subscriber is slow, track the drop
			sub.dropped.Add(1)
		}
	}
}

// Subscribe returns a channel that receives new state diffs.
// The channel has a buffer of 64 to absorb brief bursts.
// If an agent is already subscribed, the old subscription is closed first.
func (b *StateDiffBroadcaster) Subscribe(agentID string) <-chan *StateDiff {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Close existing subscription if any (M3 fix)
	if old, ok := b.subscribers[agentID]; ok {
		close(old.ch)
		b.logger.Debug("closing duplicate subscription", zap.String("agent", agentID))
	}
	sub := &diffSubscriber{
		ch: make(chan *StateDiff, 64),
	}
	b.subscribers[agentID] = sub
	b.logger.Debug("state diff subscriber added", zap.String("agent", agentID))
	return sub.ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *StateDiffBroadcaster) Unsubscribe(agentID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sub, ok := b.subscribers[agentID]; ok {
		close(sub.ch)
		dropped := sub.dropped.Load()
		delete(b.subscribers, agentID)
		if dropped > 0 {
			b.logger.Warn("state diff subscriber removed with drops",
				zap.String("agent", agentID),
				zap.Int64("dropped_diffs", dropped))
		} else {
			b.logger.Debug("state diff subscriber removed", zap.String("agent", agentID))
		}
	}
}

// DroppedCount returns the number of diffs dropped for a subscriber.
func (b *StateDiffBroadcaster) DroppedCount(agentID string) int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if sub, ok := b.subscribers[agentID]; ok {
		return sub.dropped.Load()
	}
	return 0
}

// GetRange returns state diffs for heights [from, to] inclusive.
// Returns nil entries for heights that have been evicted from the buffer.
func (b *StateDiffBroadcaster) GetRange(from, to uint64) []*StateDiff {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*StateDiff
	// Scan the ring buffer for matching heights
	for i := 0; i < b.count; i++ {
		idx := (b.head - b.count + i + b.maxBuffer) % b.maxBuffer
		d := b.buffer[idx]
		if d != nil && d.Height >= from && d.Height <= to {
			result = append(result, d)
		}
	}
	return result
}

// LatestHeight returns the height of the most recent state diff, or 0 if empty.
func (b *StateDiffBroadcaster) LatestHeight() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.count == 0 {
		return 0
	}
	idx := (b.head - 1 + b.maxBuffer) % b.maxBuffer
	if b.buffer[idx] != nil {
		return b.buffer[idx].Height
	}
	return 0
}

// SubscriberCount returns the number of active subscribers.
func (b *StateDiffBroadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// BufferedCount returns the number of diffs currently in the ring buffer.
func (b *StateDiffBroadcaster) BufferedCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}
