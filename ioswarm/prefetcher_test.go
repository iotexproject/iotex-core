package ioswarm

import (
	"fmt"
	"testing"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
)

// mockStateReader returns predefined account states.
type mockStateReader struct {
	accounts map[string]*pb.AccountSnapshot
	failAddr string // address that should fail
}

func (m *mockStateReader) AccountState(address string) (*pb.AccountSnapshot, error) {
	if address == m.failAddr {
		return nil, fmt.Errorf("state not found")
	}
	if snap, ok := m.accounts[address]; ok {
		return snap, nil
	}
	return &pb.AccountSnapshot{
		Address: address,
		Balance: "0",
		Nonce:   0,
	}, nil
}

func (m *mockStateReader) GetCode(address string) ([]byte, error)           { return nil, nil }
func (m *mockStateReader) GetStorageAt(address, slot string) (string, error) { return "", nil }

func TestPrefetchBasic(t *testing.T) {
	sr := &mockStateReader{
		accounts: map[string]*pb.AccountSnapshot{
			"io1sender": {Address: "io1sender", Balance: "1000000", Nonce: 5},
			"io1recv":   {Address: "io1recv", Balance: "500", Nonce: 0},
		},
	}
	logger := zap.NewNop()
	pf := NewPrefetcher(sr, logger)

	txs := []*PendingTx{
		{From: "io1sender", To: "io1recv", Nonce: 5},
	}

	result := pf.Prefetch(txs)

	if len(result) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(result))
	}
	if result["io1sender"].Balance != "1000000" {
		t.Fatal("wrong sender balance")
	}
	if result["io1recv"].Nonce != 0 {
		t.Fatal("wrong receiver nonce")
	}
}

func TestPrefetchCacheHit(t *testing.T) {
	callCount := 0
	sr := &mockStateReader{
		accounts: map[string]*pb.AccountSnapshot{
			"io1cached": {Address: "io1cached", Balance: "999", Nonce: 1},
		},
	}
	// Wrap to count calls
	countingSR := &countingStateReader{inner: sr, count: &callCount}

	logger := zap.NewNop()
	pf := NewPrefetcher(countingSR, logger)

	txs := []*PendingTx{{From: "io1cached", To: ""}}

	// First call — cache miss
	pf.Prefetch(txs)
	firstCount := callCount

	// Second call — should be cache hit
	pf.Prefetch(txs)
	if callCount != firstCount {
		t.Fatal("expected cache hit on second prefetch")
	}
}

func TestPrefetchPartialFailure(t *testing.T) {
	sr := &mockStateReader{
		accounts: map[string]*pb.AccountSnapshot{
			"io1ok": {Address: "io1ok", Balance: "100", Nonce: 0},
		},
		failAddr: "io1fail",
	}
	logger := zap.NewNop()
	pf := NewPrefetcher(sr, logger)

	txs := []*PendingTx{
		{From: "io1ok", To: "io1fail"},
	}

	result := pf.Prefetch(txs)
	if _, ok := result["io1ok"]; !ok {
		t.Fatal("expected io1ok to be present")
	}
	if _, ok := result["io1fail"]; ok {
		t.Fatal("expected io1fail to be absent")
	}
}

func TestPrefetchInvalidateCache(t *testing.T) {
	callCount := 0
	sr := &mockStateReader{
		accounts: map[string]*pb.AccountSnapshot{
			"io1x": {Address: "io1x", Balance: "1", Nonce: 0},
		},
	}
	countingSR := &countingStateReader{inner: sr, count: &callCount}

	logger := zap.NewNop()
	pf := NewPrefetcher(countingSR, logger)

	txs := []*PendingTx{{From: "io1x"}}

	pf.Prefetch(txs)
	c1 := callCount

	pf.InvalidateCache()
	pf.Prefetch(txs)

	if callCount <= c1 {
		t.Fatal("expected cache miss after invalidation")
	}
}

type countingStateReader struct {
	inner StateReader
	count *int
}

func (c *countingStateReader) AccountState(address string) (*pb.AccountSnapshot, error) {
	*c.count++
	return c.inner.AccountState(address)
}

func (c *countingStateReader) GetCode(address string) ([]byte, error)           { return nil, nil }
func (c *countingStateReader) GetStorageAt(address, slot string) (string, error) { return "", nil }
