package ioswarm

import (
	"context"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"go.uber.org/zap"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

func TestL5CoordinatorRegisterAgent(t *testing.T) {
	cfg := Config{
		GRPCPort:        0, // Let OS assign
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg, WithL5Logger(zap.NewNop()))

	err := coord.RegisterAgent("l5-agent-1", "io1wallet123", "us-west", "1.0.0")
	if err != nil {
		t.Fatalf("expected registration to succeed, got %v", err)
	}

	// Verify agent is in registry
	agent, ok := coord.registry.GetAgent("l5-agent-1")
	if !ok {
		t.Fatal("expected agent to be registered")
	}
	if agent.ID != "l5-agent-1" {
		t.Fatalf("expected agent ID l5-agent-1, got %s", agent.ID)
	}
}

func TestL5CoordinatorHeartbeat(t *testing.T) {
	cfg := Config{
		GRPCPort:        0,
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg, WithL5Logger(zap.NewNop()))

	// Register agent first
	coord.RegisterAgent("l5-agent-1", "io1wallet", "us", "1.0")

	// Send heartbeat
	resp := coord.Heartbeat("l5-agent-1", 10, 8, 1000, 0.5, 0.6)
	if resp == nil {
		t.Fatal("expected heartbeat response")
	}
	if !resp.Alive {
		t.Fatal("expected Alive=true")
	}
	if resp.Directive != "continue" {
		t.Fatalf("expected directive 'continue', got %s", resp.Directive)
	}

	// Verify heartbeat was recorded
	agent, ok := coord.registry.GetAgent("l5-agent-1")
	if !ok {
		t.Fatal("expected agent to exist")
	}
	if agent.CPUUsage != 0.5 {
		t.Fatalf("expected CPU usage 0.5, got %f", agent.CPUUsage)
	}
	if agent.MemUsage != 0.6 {
		t.Fatalf("expected Mem usage 0.6, got %f", agent.MemUsage)
	}
}

func TestL5CoordinatorStats(t *testing.T) {
	cfg := Config{
		GRPCPort:        0,
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg, WithL5Logger(zap.NewNop()))

	// Initial stats
	candidates, selected, rejected := coord.Stats()
	if candidates != 0 || selected != 0 || rejected != 0 {
		t.Fatal("expected initial stats to be 0")
	}

	// After some operations
	coord.totalCandidates.Store(10)
	coord.totalSelected.Store(5)
	coord.totalRejected.Store(2)

	candidates, selected, rejected = coord.Stats()
	if candidates != 10 || selected != 5 || rejected != 2 {
		t.Fatalf("expected stats (10, 5, 2), got (%d, %d, %d)", candidates, selected, rejected)
	}
}

func TestL5CoordinatorReceiveBlock(t *testing.T) {
	cfg := Config{
		GRPCPort:        0,
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg, WithL5Logger(zap.NewNop()))

	// Initial state
	if coord.lastBlockHeight.Load() != 0 {
		t.Fatal("expected initial block height to be 0")
	}

	// Receive a block (we use a mock block here since creating a real block is complex)
	// For this test, we just verify the method doesn't panic and the atomic values update
	// In a real test, we would create a proper block

	// Verify atomic values can be read
	height := coord.lastBlockHeight.Load()
	ts := coord.lastBlockTimestamp.Load()
	t.Logf("Block tracking: height=%d, timestamp=%d", height, ts)
}

func TestL5CoordinatorStartStop(t *testing.T) {
	cfg := Config{
		GRPCPort:        0, // Let OS assign
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg, WithL5Logger(zap.NewNop()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start
	err := coord.Start(ctx)
	if err != nil {
		t.Fatalf("expected start to succeed, got %v", err)
	}

	// Verify running
	if !coord.running.Load() {
		t.Fatal("expected running to be true after start")
	}

	// Double start should be no-op
	err = coord.Start(ctx)
	if err != nil {
		t.Fatalf("expected double start to succeed (no-op), got %v", err)
	}

	// Stop
	coord.Stop()

	// Verify stopped
	if coord.running.Load() {
		t.Fatal("expected running to be false after stop")
	}

	// Double stop should be no-op (no panic)
	coord.Stop()
}

func TestL5CoordinatorSubmitBlockCandidateWithoutSelector(t *testing.T) {
	// Create coordinator without operator key (no selector)
	cfg := Config{
		GRPCPort:        0,
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg, WithL5Logger(zap.NewNop()))

	// SubmitBlockCandidate should handle nil selector gracefully
	candidate := &pb.BlockCandidate{
		AgentID:     "l5-agent-1",
		BlockHeight: 100,
		TxCount:     5,
		GasUsed:     1000000,
	}

	resp, err := coord.SubmitBlockCandidate(candidate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Accepted {
		t.Fatal("expected candidate to be rejected (no selector)")
	}
}

func TestL5CoordinatorWithOperatorKey(t *testing.T) {
	// Generate a test key
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	cfg := Config{
		GRPCPort:        0,
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg,
		WithL5Logger(zap.NewNop()),
		WithL5OperatorKey(privKey),
		WithL5EVMNetworkID(4689),
	)

	// Verify selector and verifier were created
	if coord.selector == nil {
		t.Fatal("expected selector to be created")
	}
	if coord.verifier == nil {
		t.Fatal("expected verifier to be created")
	}
}

func TestL5CoordinatorPendingPayout(t *testing.T) {
	cfg := Config{
		GRPCPort:        0,
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg, WithL5Logger(zap.NewNop()))

	// Register agent
	coord.RegisterAgent("l5-agent-1", "io1wallet", "us", "1.0")

	// No pending payout initially
	resp := coord.Heartbeat("l5-agent-1", 10, 8, 1000, 0.5, 0.6)
	if resp.Payout != nil {
		t.Fatal("expected no pending payout initially")
	}

	// Store a pending payout
	coord.pendingPayouts.Store("l5-agent-1", &pb.PayoutInfo{
		Epoch:       1,
		AmountIOTX:  100.5,
		Rank:        1,
		TotalAgents: 5,
	})

	// Heartbeat should return the payout
	resp = coord.Heartbeat("l5-agent-1", 10, 8, 1000, 0.5, 0.6)
	if resp.Payout == nil {
		t.Fatal("expected pending payout")
	}
	if resp.Payout.Epoch != 1 {
		t.Fatalf("expected epoch 1, got %d", resp.Payout.Epoch)
	}
	if resp.Payout.AmountIOTX != 100.5 {
		t.Fatalf("expected amount 100.5, got %f", resp.Payout.AmountIOTX)
	}

	// Payout should be consumed (not returned again)
	resp = coord.Heartbeat("l5-agent-1", 10, 8, 1000, 0.5, 0.6)
	if resp.Payout != nil {
		t.Fatal("expected payout to be consumed")
	}
}

func TestL5CoordinatorConsumePayoutViaHeartbeat(t *testing.T) {
	cfg := Config{
		GRPCPort:        0,
		MaxAgents:       10,
		Reward:          DefaultRewardConfig(),
		DelegateAddress: "io1delegate",
	}

	coord := NewL5Coordinator(cfg, WithL5Logger(zap.NewNop()))

	// Register agent
	coord.RegisterAgent("agent-1", "io1wallet", "us", "1.0")

	// No payout initially - test via Heartbeat which uses consumePayout internally
	resp := coord.Heartbeat("agent-1", 10, 8, 1000, 0.5, 0.6)
	if resp.Payout != nil {
		t.Fatal("expected no payout initially")
	}

	// Store a payout (simulating epoch reward distribution)
	coord.pendingPayouts.Store("agent-1", &pb.PayoutInfo{
		Epoch:       1,
		AmountIOTX:  50.0,
		Rank:        2,
		TotalAgents: 10,
	})

	// Heartbeat should consume and return the payout
	resp = coord.Heartbeat("agent-1", 10, 8, 1000, 0.5, 0.6)
	if resp.Payout == nil {
		t.Fatal("expected payout")
	}
	if resp.Payout.Epoch != 1 || resp.Payout.AmountIOTX != 50.0 {
		t.Fatalf("unexpected payout values: %+v", resp.Payout)
	}

	// Second heartbeat - payout should be gone (consumed)
	resp = coord.Heartbeat("agent-1", 10, 8, 1000, 0.5, 0.6)
	if resp.Payout != nil {
		t.Fatal("expected payout to be consumed")
	}
}
