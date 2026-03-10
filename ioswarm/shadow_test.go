package ioswarm

import (
	"testing"

	pb "github.com/iotexproject/iotex-core/ioswarm/proto"
	"go.uber.org/zap"
)

func TestShadowAllMatch(t *testing.T) {
	logger := zap.NewNop()
	shadow := NewShadowComparator(logger)

	// Agent says both valid
	shadow.RecordAgentResults("ant-1", &pb.BatchResult{
		AgentID: "ant-1",
		BatchID: "b1",
		Results: []*pb.TaskResult{
			{TaskID: 1, Valid: true},
			{TaskID: 2, Valid: true},
		},
	})

	// Actual: both valid
	actual := map[uint32]bool{1: true, 2: true}
	mismatches := shadow.CompareWithActual(actual, 100)

	if len(mismatches) != 0 {
		t.Fatalf("expected 0 mismatches, got %d", len(mismatches))
	}

	stats := shadow.Stats()
	if stats.TotalCompared != 2 || stats.TotalMatched != 2 {
		t.Fatalf("expected 2/2 matched, got %d/%d", stats.TotalMatched, stats.TotalCompared)
	}
}

func TestShadowMismatch(t *testing.T) {
	logger := zap.NewNop()
	shadow := NewShadowComparator(logger)

	shadow.RecordAgentResults("ant-1", &pb.BatchResult{
		AgentID: "ant-1",
		Results: []*pb.TaskResult{
			{TaskID: 1, Valid: true},  // agent: valid
			{TaskID: 2, Valid: false, RejectReason: "bad nonce"}, // agent: invalid
		},
	})

	// Actual: task 1 invalid (false positive), task 2 valid (false negative)
	actual := map[uint32]bool{1: false, 2: true}
	mismatches := shadow.CompareWithActual(actual, 200)

	if len(mismatches) != 2 {
		t.Fatalf("expected 2 mismatches, got %d", len(mismatches))
	}

	stats := shadow.Stats()
	if stats.FalsePositives != 1 {
		t.Fatalf("expected 1 false positive, got %d", stats.FalsePositives)
	}
	if stats.FalseNegatives != 1 {
		t.Fatalf("expected 1 false negative, got %d", stats.FalseNegatives)
	}
}

func TestShadowReset(t *testing.T) {
	logger := zap.NewNop()
	shadow := NewShadowComparator(logger)

	shadow.RecordAgentResults("ant-1", &pb.BatchResult{
		Results: []*pb.TaskResult{{TaskID: 1, Valid: true}},
	})
	shadow.CompareWithActual(map[uint32]bool{1: true}, 1)

	shadow.ResetStats()
	stats := shadow.Stats()
	if stats.TotalCompared != 0 {
		t.Fatal("expected stats to be reset")
	}
}
