package ioswarm

import (
	"testing"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
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
	mismatches, perAgent := shadow.CompareWithActual(actual, nil, 100)

	if len(mismatches) != 0 {
		t.Fatalf("expected 0 mismatches, got %d", len(mismatches))
	}

	stats := shadow.Stats()
	if stats.TotalCompared != 2 || stats.TotalMatched != 2 {
		t.Fatalf("expected 2/2 matched, got %d/%d", stats.TotalMatched, stats.TotalCompared)
	}

	// Verify per-agent accuracy
	if aa, ok := perAgent["ant-1"]; !ok || aa.Compared != 2 || aa.Matched != 2 {
		t.Fatalf("expected ant-1 accuracy 2/2, got %+v", perAgent["ant-1"])
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
	mismatches, perAgent := shadow.CompareWithActual(actual, nil, 200)

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

	// Verify per-agent accuracy: 0 matched out of 2
	if aa, ok := perAgent["ant-1"]; !ok || aa.Compared != 2 || aa.Matched != 0 {
		t.Fatalf("expected ant-1 accuracy 0/2, got %+v", perAgent["ant-1"])
	}
}

func TestShadowReset(t *testing.T) {
	logger := zap.NewNop()
	shadow := NewShadowComparator(logger)

	shadow.RecordAgentResults("ant-1", &pb.BatchResult{
		Results: []*pb.TaskResult{{TaskID: 1, Valid: true}},
	})
	shadow.CompareWithActual(map[uint32]bool{1: true}, nil, 1) //nolint:dogsled

	shadow.ResetStats()
	stats := shadow.Stats()
	if stats.TotalCompared != 0 {
		t.Fatal("expected stats to be reset")
	}
}
