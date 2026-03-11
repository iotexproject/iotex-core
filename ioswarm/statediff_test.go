package ioswarm

import (
	"testing"

	"go.uber.org/zap"
)

func TestStateDiffBroadcaster_PublishAndSubscribe(t *testing.T) {
	logger := zap.NewNop()
	b := NewStateDiffBroadcaster(10, logger)

	// Subscribe
	ch := b.Subscribe("agent-1")

	// Publish a diff
	diff := &StateDiff{
		Height: 100,
		Entries: []StateDiffEntry{
			{Type: 0, Namespace: "account", Key: []byte("addr1"), Value: []byte("state1")},
			{Type: 0, Namespace: "evm", Key: []byte("code1"), Value: []byte("bytecode")},
			{Type: 1, Namespace: "account", Key: []byte("addr2")},
		},
		DigestBytes: []byte("digest-hash"),
	}
	b.Publish(diff)

	// Verify subscriber receives it
	received := <-ch
	if received.Height != 100 {
		t.Fatalf("expected height 100, got %d", received.Height)
	}
	if len(received.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(received.Entries))
	}
	if received.Entries[0].Namespace != "account" {
		t.Fatalf("expected namespace 'account', got %s", received.Entries[0].Namespace)
	}

	// Verify latest height
	if h := b.LatestHeight(); h != 100 {
		t.Fatalf("expected latest height 100, got %d", h)
	}

	// Verify subscriber count
	if c := b.SubscriberCount(); c != 1 {
		t.Fatalf("expected 1 subscriber, got %d", c)
	}

	// Unsubscribe
	b.Unsubscribe("agent-1")
	if c := b.SubscriberCount(); c != 0 {
		t.Fatalf("expected 0 subscribers, got %d", c)
	}
}

func TestStateDiffBroadcaster_RingBuffer(t *testing.T) {
	logger := zap.NewNop()
	b := NewStateDiffBroadcaster(5, logger) // small buffer

	// Publish 8 diffs (overflows buffer of 5)
	for i := uint64(1); i <= 8; i++ {
		b.Publish(&StateDiff{Height: i})
	}

	// Buffer should contain last 5
	if c := b.BufferedCount(); c != 5 {
		t.Fatalf("expected 5 buffered, got %d", c)
	}
	if h := b.LatestHeight(); h != 8 {
		t.Fatalf("expected latest height 8, got %d", h)
	}

	// GetRange should return heights 4-8 (the last 5)
	diffs := b.GetRange(4, 8)
	if len(diffs) != 5 {
		t.Fatalf("expected 5 diffs in range, got %d", len(diffs))
	}
	for i, d := range diffs {
		expected := uint64(4 + i)
		if d.Height != expected {
			t.Fatalf("expected height %d, got %d", expected, d.Height)
		}
	}

	// GetRange for heights 1-3 should be empty (evicted)
	diffs = b.GetRange(1, 3)
	if len(diffs) != 0 {
		t.Fatalf("expected 0 diffs for evicted range, got %d", len(diffs))
	}
}

func TestStateDiffBroadcaster_NonBlockingPublish(t *testing.T) {
	logger := zap.NewNop()
	b := NewStateDiffBroadcaster(10, logger)

	// Subscribe but don't read — channel has buffer of 32
	_ = b.Subscribe("slow-agent")

	// Publish more than channel buffer (should not block)
	for i := uint64(1); i <= 50; i++ {
		b.Publish(&StateDiff{Height: i})
	}

	// Should complete without deadlock
	if h := b.LatestHeight(); h != 50 {
		t.Fatalf("expected latest height 50, got %d", h)
	}
}
