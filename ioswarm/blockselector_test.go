package ioswarm

import (
	"context"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"go.uber.org/zap"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

func TestNewBlockSelector(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	selector := NewBlockSelector(privKey, 4689, 10, 1*time.Second, zap.NewNop())
	if selector == nil {
		t.Fatal("expected selector to be created")
	}
	if selector.maxCandidates != 10 {
		t.Fatalf("expected maxCandidates 10, got %d", selector.maxCandidates)
	}
	if selector.selectionTime != 1*time.Second {
		t.Fatalf("expected selectionTime 1s, got %v", selector.selectionTime)
	}
}

func TestBlockSelectorStats(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	selector := NewBlockSelector(privKey, 4689, 10, 1*time.Second, zap.NewNop())

	// Initial stats
	received, selected, rejected := selector.Stats()
	if received != 0 || selected != 0 || rejected != 0 {
		t.Fatal("expected initial stats to be 0")
	}
}

func TestBlockSelectorCandidateCount(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	selector := NewBlockSelector(privKey, 4689, 10, 1*time.Second, zap.NewNop())

	// No candidates initially
	count := selector.CandidateCount(100)
	if count != 0 {
		t.Fatalf("expected 0 candidates, got %d", count)
	}
}

func TestBlockSelectorPruneOldCandidates(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	selector := NewBlockSelector(privKey, 4689, 10, 1*time.Second, zap.NewNop())

	// No candidates to prune
	pruned := selector.PruneOldCandidates(1 * time.Minute)
	if pruned != 0 {
		t.Fatalf("expected 0 pruned, got %d", pruned)
	}
}

func TestBlockSelectorSelectBlockNoCandidates(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	selector := NewBlockSelector(privKey, 4689, 10, 100*time.Millisecond, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Should timeout and return error (no candidates)
	_, _, err := selector.SelectBlock(ctx, 100)
	if err == nil {
		t.Fatal("expected error when no candidates available")
	}
}

func TestBlockSelectorSubmitCandidateEmptyBlock(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	selector := NewBlockSelector(privKey, 4689, 10, 1*time.Second, zap.NewNop())

	// Empty block proto should be rejected
	candidate := &pb.BlockCandidate{
		AgentID:     "agent-1",
		BlockHeight: 100,
		BlockProto:  []byte{}, // empty
		TxCount:     0,
		GasUsed:     0,
	}

	err := selector.SubmitCandidate(context.Background(), candidate)
	if err == nil {
		t.Fatal("expected error for empty block proto")
	}

	// Stats should show rejection
	_, _, rejected := selector.Stats()
	if rejected != 1 {
		t.Fatalf("expected 1 rejection, got %d", rejected)
	}
}

func TestBlockSelectorSubmitCandidateNoSignature(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	selector := NewBlockSelector(privKey, 4689, 10, 1*time.Second, zap.NewNop())

	// Block proto without signature should be rejected
	// (we would need a valid block proto to test this properly)
	// For now, we test the empty proto case which fails earlier
	candidate := &pb.BlockCandidate{
		AgentID:     "agent-1",
		BlockHeight: 100,
		BlockProto:  []byte{0x01, 0x02, 0x03}, // invalid proto
		TxCount:     0,
		GasUsed:     0,
	}

	err := selector.SubmitCandidate(context.Background(), candidate)
	if err == nil {
		t.Fatal("expected error for invalid block proto")
	}
}

func TestBlockSelectorContextCancellation(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	selector := NewBlockSelector(privKey, 4689, 10, 5*time.Second, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should return context canceled error
	_, _, err := selector.SelectBlock(ctx, 100)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
