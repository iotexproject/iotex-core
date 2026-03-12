package ioswarm

import (
	"fmt"
	"math/big"
	"testing"

	"go.uber.org/zap"
)

// 1 IOTX = 10^18 rau
func iotx(n int64) *big.Int {
	base := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	return new(big.Int).Mul(big.NewInt(n), base)
}

func TestRewardBasicDistribution(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultRewardConfig()
	rd := NewRewardDistributor(cfg, "", logger)

	// 3 agents did work
	rd.RecordWork("ant-1", 100, 100, 5000)
	rd.RecordWork("ant-2", 200, 200, 8000)
	rd.RecordWork("ant-3", 100, 100, 4000)
	rd.SetAgentWallet("ant-1", "io1wallet1")
	rd.SetAgentWallet("ant-2", "io1wallet2")
	rd.SetAgentWallet("ant-3", "io1wallet3")

	// Distribute 100 IOTX
	summary := rd.Distribute(iotx(100))

	// Delegate keeps 10% = 10 IOTX
	expectedDelegateCut := iotx(10)
	if summary.DelegateCut.Cmp(expectedDelegateCut) != 0 {
		t.Fatalf("expected delegate cut %s, got %s",
			FormatIOTX(expectedDelegateCut), FormatIOTX(summary.DelegateCut))
	}

	// Agent pool = 90 IOTX
	expectedPool := iotx(90)
	if summary.AgentPool.Cmp(expectedPool) != 0 {
		t.Fatalf("expected agent pool %s, got %s",
			FormatIOTX(expectedPool), FormatIOTX(summary.AgentPool))
	}

	// ant-2 did 200 tasks (50% of 400 total) → ~45 IOTX
	// ant-1 and ant-3 did 100 each (25% each) → ~22.5 IOTX each
	if len(summary.Payouts) != 3 {
		t.Fatalf("expected 3 payouts, got %d", len(summary.Payouts))
	}

	// ant-2 should be first (most tasks)
	if summary.Payouts[0].AgentID != "ant-2" {
		t.Fatalf("expected ant-2 first, got %s", summary.Payouts[0].AgentID)
	}

	// ant-2's share should be ~50%
	if summary.Payouts[0].Share < 45 || summary.Payouts[0].Share > 55 {
		t.Fatalf("expected ant-2 share ~50%%, got %.1f%%", summary.Payouts[0].Share)
	}

	t.Log("\n" + PrintPayoutTable(summary))
}

func TestRewardMinTasksThreshold(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultRewardConfig()
	cfg.MinTasksForReward = 50
	rd := NewRewardDistributor(cfg, "", logger)

	rd.RecordWork("ant-active", 100, 100, 1000)
	rd.RecordWork("ant-lazy", 5, 5, 100) // below threshold

	summary := rd.Distribute(iotx(100))

	if summary.AgentCount != 1 {
		t.Fatalf("expected 1 eligible agent, got %d", summary.AgentCount)
	}
	if summary.Payouts[0].AgentID != "ant-active" {
		t.Fatal("expected ant-active to be the only recipient")
	}

	// ant-active gets entire agent pool (90 IOTX)
	if summary.Payouts[0].AmountIOTX < 89 {
		t.Fatalf("expected ~90 IOTX, got %.2f", summary.Payouts[0].AmountIOTX)
	}
}

func TestRewardAccuracyBonus(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultRewardConfig()
	cfg.BonusAccuracyPct = 99.0
	cfg.BonusMultiplier = 1.5 // 50% bonus for clarity
	rd := NewRewardDistributor(cfg, "", logger)

	// Both do 100 tasks, but ant-perfect has 100% accuracy
	rd.RecordWork("ant-perfect", 100, 100, 1000)  // 100% accuracy → bonus
	rd.RecordWork("ant-sloppy", 100, 90, 1000)    // 90% accuracy → no bonus

	summary := rd.Distribute(iotx(100))

	// ant-perfect: weight = 100 * 1.5 = 150
	// ant-sloppy:  weight = 100 * 1.0 = 100
	// ant-perfect share: 150/250 = 60%
	// ant-sloppy share:  100/250 = 40%

	var perfectPayout, sloppyPayout Payout
	for _, p := range summary.Payouts {
		if p.AgentID == "ant-perfect" {
			perfectPayout = p
		}
		if p.AgentID == "ant-sloppy" {
			sloppyPayout = p
		}
	}

	if !perfectPayout.BonusApplied {
		t.Fatal("expected bonus for ant-perfect")
	}
	if sloppyPayout.BonusApplied {
		t.Fatal("expected no bonus for ant-sloppy")
	}

	// Perfect should get more
	if perfectPayout.Amount.Cmp(sloppyPayout.Amount) <= 0 {
		t.Fatalf("expected ant-perfect > ant-sloppy, got %s vs %s",
			FormatIOTX(perfectPayout.Amount), FormatIOTX(sloppyPayout.Amount))
	}

	if perfectPayout.Share < 55 || perfectPayout.Share > 65 {
		t.Fatalf("expected ant-perfect share ~60%%, got %.1f%%", perfectPayout.Share)
	}

	t.Log("\n" + PrintPayoutTable(summary))
}

func TestRewardEpochReset(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultRewardConfig()
	rd := NewRewardDistributor(cfg, "", logger)

	rd.RecordWork("ant-1", 100, 100, 1000)
	rd.Distribute(iotx(50))

	// After distribute, work should be reset
	work := rd.CurrentWork()
	if len(work) != 0 {
		t.Fatalf("expected work reset after distribute, got %d entries", len(work))
	}

	// Epoch should advance
	rd.RecordWork("ant-1", 200, 200, 2000)
	summary := rd.Distribute(iotx(50))
	if summary.Epoch != 1 {
		t.Fatalf("expected epoch 1, got %d", summary.Epoch)
	}
}

func TestReward100AgentsScenario(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultRewardConfig()
	rd := NewRewardDistributor(cfg, "", logger)

	// Simulate 100 agents with varying workloads
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("ant-%02d", i)
		tasks := uint64(150 + (i % 50)) // 150-199 tasks each
		correct := tasks
		if i%10 == 0 {
			correct = tasks - 5 // some agents have lower accuracy
		}
		rd.RecordWork(id, tasks, correct, tasks*50)
		rd.SetAgentWallet(id, fmt.Sprintf("io1wallet%02d", i))
	}

	// IoTeX delegate earns ~800 IOTX per epoch (rough estimate)
	summary := rd.Distribute(iotx(800))

	t.Logf("Epoch #%d: %d agents, %d tasks", summary.Epoch, summary.AgentCount, summary.TotalTasks)
	t.Logf("Delegate cut: %s IOTX", FormatIOTX(summary.DelegateCut))
	t.Logf("Agent pool:   %s IOTX", FormatIOTX(summary.AgentPool))

	if summary.AgentCount != 100 {
		t.Fatalf("expected 100 eligible agents, got %d", summary.AgentCount)
	}

	// All payouts should sum to agent pool
	totalPaid := new(big.Int)
	for _, p := range summary.Payouts {
		totalPaid.Add(totalPaid, p.Amount)
	}

	// Allow rounding error (float64 precision loss across 100 agents)
	diff := new(big.Int).Sub(summary.AgentPool, totalPaid)
	diff.Abs(diff)
	maxErr := new(big.Int).Exp(big.NewInt(10), big.NewInt(12), nil) // 10^12 rau = 0.000001 IOTX
	if diff.Cmp(maxErr) > 0 {
		t.Fatalf("payout sum mismatch: pool=%s, paid=%s, diff=%s",
			summary.AgentPool.String(), totalPaid.String(), diff.String())
	}

	// Print top 10 and bottom 5
	t.Log("\nTop 10:")
	for _, p := range summary.Payouts[:10] {
		t.Logf("  %-12s %4d tasks  %5.1f%% acc  %5.2f IOTX  share=%.2f%%",
			p.AgentID, p.TasksDone, p.Accuracy*100, p.AmountIOTX, p.Share)
	}
	t.Log("Bottom 5:")
	for _, p := range summary.Payouts[95:] {
		t.Logf("  %-12s %4d tasks  %5.1f%% acc  %5.2f IOTX  share=%.2f%%",
			p.AgentID, p.TasksDone, p.Accuracy*100, p.AmountIOTX, p.Share)
	}
}
