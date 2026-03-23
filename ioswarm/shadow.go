package ioswarm

import (
	"sync"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
)

// ShadowResult holds the comparison between agent and actual results.
type ShadowResult struct {
	TaskID       uint32
	AgentResult  *pb.TaskResult
	ActualValid  bool
	Match        bool
	AgentID      string
	BlockHeight  uint64
}

// ShadowComparator compares agent validation results against iotex-core's
// actual block execution. In shadow mode, agent results never affect
// block production — they're purely observational.
type ShadowComparator struct {
	mu      sync.Mutex
	logger  *zap.Logger
	results []ShadowResult
	stats   ShadowStats
}

// ShadowStats tracks shadow mode accuracy metrics.
type ShadowStats struct {
	TotalCompared   uint64
	TotalMatched    uint64
	TotalMismatched uint64
	FalsePositives  uint64 // agent said valid, actual invalid
	FalseNegatives  uint64 // agent said invalid, actual valid

	// EVM-specific shadow stats (L3)
	EVMGasMatches      uint64
	EVMGasMismatches   uint64
	EVMStateMatches    uint64
	EVMStateMismatches uint64
}

// NewShadowComparator creates a new shadow comparator.
func NewShadowComparator(logger *zap.Logger) *ShadowComparator {
	return &ShadowComparator{
		logger: logger,
	}
}

// RecordAgentResults stores agent results for later comparison.
func (s *ShadowComparator) RecordAgentResults(agentID string, batch *pb.BatchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range batch.Results {
		s.results = append(s.results, ShadowResult{
			TaskID:      r.TaskID,
			AgentResult: r,
			AgentID:     agentID,
		})
	}
}

// AgentAccuracy holds per-agent shadow match counts for a comparison batch.
type AgentAccuracy struct {
	Compared uint64
	Matched  uint64
}

// CompareWithActual compares stored agent results against actual execution.
// actualResults maps task_id → whether the tx was actually valid.
// Returns mismatches and per-agent accuracy counts.
func (s *ShadowComparator) CompareWithActual(actualResults map[uint32]bool, blockHeight uint64) ([]ShadowResult, map[string]*AgentAccuracy) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var mismatches []ShadowResult
	perAgent := make(map[string]*AgentAccuracy)

	for i := range s.results {
		r := &s.results[i]
		actual, exists := actualResults[r.TaskID]
		if !exists {
			continue
		}

		r.ActualValid = actual
		r.BlockHeight = blockHeight
		r.Match = r.AgentResult.Valid == actual

		// Track per-agent accuracy
		aa, ok := perAgent[r.AgentID]
		if !ok {
			aa = &AgentAccuracy{}
			perAgent[r.AgentID] = aa
		}
		aa.Compared++

		s.stats.TotalCompared++
		if r.Match {
			s.stats.TotalMatched++
			aa.Matched++
		} else {
			s.stats.TotalMismatched++
			if r.AgentResult.Valid && !actual {
				s.stats.FalsePositives++
			} else {
				s.stats.FalseNegatives++
			}
			mismatches = append(mismatches, *r)

			s.logger.Warn("shadow mismatch",
				zap.Uint32("task_id", r.TaskID),
				zap.String("agent", r.AgentID),
				zap.Bool("agent_valid", r.AgentResult.Valid),
				zap.Bool("actual_valid", actual),
				zap.String("reject_reason", r.AgentResult.RejectReason),
				zap.Uint64("block", blockHeight))
		}
	}

	if s.stats.TotalCompared > 0 {
		accuracy := float64(s.stats.TotalMatched) / float64(s.stats.TotalCompared) * 100
		s.logger.Info("shadow comparison complete",
			zap.Uint64("total", s.stats.TotalCompared),
			zap.Uint64("matched", s.stats.TotalMatched),
			zap.Uint64("mismatched", s.stats.TotalMismatched),
			zap.Float64("accuracy_pct", accuracy),
			zap.Uint64("block", blockHeight))
	}

	// Clear processed results
	s.results = s.results[:0]

	return mismatches, perAgent
}

// Stats returns current shadow comparison statistics.
func (s *ShadowComparator) Stats() ShadowStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// EVMActualResult holds the reference EVM execution result for shadow comparison.
type EVMActualResult struct {
	TaskID       uint32
	GasUsed      uint64
	StateChanges []*pb.StateChange
	ExecError    string
}

// CompareEVMResults compares agent EVM results against reference results.
func (s *ShadowComparator) CompareEVMResults(agentResults []*pb.TaskResult, actualResults map[uint32]*EVMActualResult, blockHeight uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ar := range agentResults {
		actual, ok := actualResults[ar.TaskID]
		if !ok || ar.GasUsed == 0 {
			continue // not an L3 result or no reference
		}

		// Compare gas used
		if ar.GasUsed == actual.GasUsed {
			s.stats.EVMGasMatches++
		} else {
			s.stats.EVMGasMismatches++
			s.logger.Warn("EVM gas mismatch",
				zap.Uint32("task_id", ar.TaskID),
				zap.Uint64("agent_gas", ar.GasUsed),
				zap.Uint64("actual_gas", actual.GasUsed),
				zap.Uint64("block", blockHeight))
		}

		// Compare state changes
		if stateChangesMatch(ar.StateChanges, actual.StateChanges) {
			s.stats.EVMStateMatches++
		} else {
			s.stats.EVMStateMismatches++
			s.logger.Warn("EVM state mismatch",
				zap.Uint32("task_id", ar.TaskID),
				zap.Int("agent_changes", len(ar.StateChanges)),
				zap.Int("actual_changes", len(actual.StateChanges)),
				zap.Uint64("block", blockHeight))
		}
	}
}

// stateChangesMatch compares two sets of state changes for equality.
func stateChangesMatch(a, b []*pb.StateChange) bool {
	if len(a) != len(b) {
		return false
	}
	// Build map from a
	type key struct{ addr, slot string }
	aMap := make(map[key]string, len(a))
	for _, sc := range a {
		aMap[key{sc.Address, sc.Slot}] = sc.NewValue
	}
	// Check b matches
	for _, sc := range b {
		if aMap[key{sc.Address, sc.Slot}] != sc.NewValue {
			return false
		}
	}
	return true
}

// ResetStats resets the statistics counters.
func (s *ShadowComparator) ResetStats() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats = ShadowStats{}
}
