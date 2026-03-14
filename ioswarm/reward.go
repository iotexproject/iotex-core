package ioswarm

import (
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RewardDistributor tracks agent work and calculates reward distribution.
//
// Flow:
//   1. Coordinator calls RecordWork() on every SubmitResults
//   2. At epoch end (every N blocks), delegate calls Distribute()
//   3. Distribute() returns a payout list: agent_address → IOTX amount
//   4. Delegate signs a batch transfer tx (or calls a RewardPool contract)
//
// The delegate keeps a configurable cut (e.g. 10%) for operating costs,
// and distributes the rest proportionally by tasks validated.
type RewardDistributor struct {
	mu              sync.Mutex
	logger          *zap.Logger
	cfg             RewardConfig
	delegateAddress string

	// Work tracking per epoch
	currentEpoch  uint64
	epochStart    time.Time
	agentWork     map[string]*AgentWork // agent_id → work stats
	epochHistory  []EpochSummary
}

// RewardConfig controls reward distribution parameters.
type RewardConfig struct {
	// DelegateCutPct is the percentage the delegate keeps (0-100).
	// Default: 10 (delegate keeps 10%, agents split 90%)
	DelegateCutPct float64 `yaml:"delegateCutPct"`

	// EpochBlocks is how many blocks per reward epoch.
	// Default: 360 (IoTeX epoch = 1 hour at 10s blocks)
	EpochBlocks uint64 `yaml:"epochBlocks"`

	// MinTasksForReward is the minimum tasks an agent must process
	// in an epoch to receive rewards. Prevents freeloading.
	// Default: 10
	MinTasksForReward uint64 `yaml:"minTasksForReward"`

	// BonusAccuracyPct: agents with shadow accuracy above this get a bonus.
	// Default: 99.5
	BonusAccuracyPct float64 `yaml:"bonusAccuracyPct"`

	// BonusMultiplier: bonus multiplier for high-accuracy agents.
	// Default: 1.2 (20% bonus)
	BonusMultiplier float64 `yaml:"bonusMultiplier"`
}

// DefaultRewardConfig returns sane defaults.
func DefaultRewardConfig() RewardConfig {
	return RewardConfig{
		DelegateCutPct:    10,
		EpochBlocks:       360,
		MinTasksForReward: 10,
		BonusAccuracyPct:  99.5,
		BonusMultiplier:   1.2,
	}
}

// AgentWork tracks an agent's contribution in the current epoch.
type AgentWork struct {
	AgentID        string
	WalletAddress  string // agent's IOTX address for payout
	TasksProcessed uint64
	TasksCorrect   uint64 // matched in shadow mode
	TotalLatencyUs uint64
	Uptime         time.Duration // time since registration
	Level          string        // L1, L2, L3
}

// Accuracy returns the agent's shadow mode accuracy (0.0-1.0).
func (w *AgentWork) Accuracy() float64 {
	if w.TasksProcessed == 0 {
		return 0
	}
	return float64(w.TasksCorrect) / float64(w.TasksProcessed)
}

// Payout represents a single agent's reward for an epoch.
type Payout struct {
	AgentID       string
	WalletAddress string
	Amount        *big.Int // in rau (1 IOTX = 10^18 rau)
	AmountIOTX    float64  // human-readable
	TasksDone     uint64
	Accuracy      float64
	Share         float64 // percentage of agent pool
	BonusApplied  bool
}

// EpochSummary records a completed epoch's distribution.
type EpochSummary struct {
	Epoch           uint64
	TotalReward     *big.Int
	DelegateCut     *big.Int
	DelegateAddress string
	AgentPool       *big.Int
	Payouts         []Payout
	AgentCount      int
	TotalTasks      uint64
	Timestamp       time.Time
}

// NewRewardDistributor creates a new reward distributor.
func NewRewardDistributor(cfg RewardConfig, delegateAddress string, logger *zap.Logger) *RewardDistributor {
	return &RewardDistributor{
		cfg:             cfg,
		delegateAddress: delegateAddress,
		logger:          logger,
		agentWork:       make(map[string]*AgentWork),
		epochStart:      time.Now(),
	}
}

// RecordWork records an agent's completed work. Called on every SubmitResults.
func (r *RewardDistributor) RecordWork(agentID string, tasksProcessed, tasksCorrect uint64, totalLatencyUs uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	work, ok := r.agentWork[agentID]
	if !ok {
		work = &AgentWork{AgentID: agentID}
		r.agentWork[agentID] = work
	}

	work.TasksProcessed += tasksProcessed
	work.TasksCorrect += tasksCorrect
	work.TotalLatencyUs += totalLatencyUs
}

// RecordShadowAccuracy adds verified correct tasks from shadow comparison.
// Called from OnBlockExecuted after ground-truth comparison.
func (r *RewardDistributor) RecordShadowAccuracy(agentID string, matched uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	work, ok := r.agentWork[agentID]
	if !ok {
		return // agent not tracked (shouldn't happen)
	}
	work.TasksCorrect += matched
}

// SetAgentWallet sets the payout address for an agent.
func (r *RewardDistributor) SetAgentWallet(agentID, walletAddress string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	work, ok := r.agentWork[agentID]
	if !ok {
		work = &AgentWork{AgentID: agentID}
		r.agentWork[agentID] = work
	}
	work.WalletAddress = walletAddress
}

// computePayouts is the internal calculation logic used by Distribute.
// Caller must hold r.mu.
func (r *RewardDistributor) computePayouts(totalReward *big.Int) *EpochSummary {
	// 1. Calculate delegate cut
	delegateCut := new(big.Int).Mul(totalReward, big.NewInt(int64(r.cfg.DelegateCutPct)))
	delegateCut.Div(delegateCut, big.NewInt(100))

	agentPool := new(big.Int).Sub(totalReward, delegateCut)

	// 2. Filter eligible agents (minimum tasks threshold)
	var eligible []*AgentWork
	var totalTasks uint64
	for _, work := range r.agentWork {
		if work.TasksProcessed >= r.cfg.MinTasksForReward {
			eligible = append(eligible, work)
			totalTasks += work.TasksProcessed
		}
	}

	// 3. Calculate weighted shares (tasks × bonus)
	type weightedAgent struct {
		work   *AgentWork
		weight float64
		bonus  bool
	}

	var totalWeight float64
	weighted := make([]weightedAgent, 0, len(eligible))

	for _, work := range eligible {
		w := float64(work.TasksProcessed)
		bonus := false

		// Apply accuracy bonus
		if work.Accuracy()*100 >= r.cfg.BonusAccuracyPct && work.TasksProcessed > 0 {
			w *= r.cfg.BonusMultiplier
			bonus = true
		}

		weighted = append(weighted, weightedAgent{work: work, weight: w, bonus: bonus})
		totalWeight += w
	}

	// 4. Calculate payouts
	payouts := make([]Payout, 0, len(weighted))

	for _, wa := range weighted {
		share := float64(0)
		amount := new(big.Int)

		if totalWeight > 0 {
			share = wa.weight / totalWeight
			// amount = agentPool * share
			amount.Mul(agentPool, big.NewInt(int64(wa.weight*1e9)))
			amount.Div(amount, big.NewInt(int64(totalWeight*1e9)))
		}

		// Convert to IOTX for readability
		iotx := new(big.Float).SetInt(amount)
		iotx.Quo(iotx, new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)))
		amountIOTX, _ := iotx.Float64()

		payouts = append(payouts, Payout{
			AgentID:       wa.work.AgentID,
			WalletAddress: wa.work.WalletAddress,
			Amount:        amount,
			AmountIOTX:    amountIOTX,
			TasksDone:     wa.work.TasksProcessed,
			Accuracy:      wa.work.Accuracy(),
			Share:         share * 100,
			BonusApplied:  wa.bonus,
		})
	}

	// Sort by amount (descending)
	sort.Slice(payouts, func(i, j int) bool {
		return payouts[i].Amount.Cmp(payouts[j].Amount) > 0
	})

	return &EpochSummary{
		Epoch:           r.currentEpoch,
		TotalReward:     new(big.Int).Set(totalReward),
		DelegateCut:     delegateCut,
		DelegateAddress: r.delegateAddress,
		AgentPool:       agentPool,
		Payouts:         payouts,
		AgentCount:      len(eligible),
		TotalTasks:      totalTasks,
		Timestamp:       time.Now(),
	}
}

// Distribute computes payouts, freezes the snapshot, and advances the epoch
// atomically under a single lock hold. The returned summary is the exact
// snapshot that should be used for on-chain settlement.
func (r *RewardDistributor) Distribute(totalReward *big.Int) *EpochSummary {
	r.mu.Lock()
	defer r.mu.Unlock()

	summary := r.computePayouts(totalReward)
	r.epochHistory = append(r.epochHistory, *summary)

	r.logger.Info("epoch advanced",
		zap.Uint64("epoch", r.currentEpoch),
		zap.String("total", FormatIOTX(totalReward)),
		zap.String("agent_pool", FormatIOTX(summary.AgentPool)),
		zap.Int("eligible_agents", summary.AgentCount),
		zap.Uint64("total_tasks", summary.TotalTasks))

	// Reset for next epoch — preserve wallet addresses
	r.currentEpoch++
	r.epochStart = time.Now()
	newWork := make(map[string]*AgentWork)
	for id, w := range r.agentWork {
		if w.WalletAddress != "" {
			newWork[id] = &AgentWork{
				AgentID:       id,
				WalletAddress: w.WalletAddress,
			}
		}
	}
	r.agentWork = newWork

	return summary
}

// CurrentWork returns a snapshot of current epoch work stats.
func (r *RewardDistributor) CurrentWork() map[string]*AgentWork {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := make(map[string]*AgentWork, len(r.agentWork))
	for k, v := range r.agentWork {
		w := *v
		cp[k] = &w
	}
	return cp
}

// CurrentEpoch returns the current epoch number.
func (r *RewardDistributor) CurrentEpoch() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentEpoch
}

// EpochElapsed returns the time since the current epoch started.
func (r *RewardDistributor) EpochElapsed() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return time.Since(r.epochStart)
}

// EpochHistory returns all past epoch summaries.
func (r *RewardDistributor) EpochHistory() []EpochSummary {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := make([]EpochSummary, len(r.epochHistory))
	copy(cp, r.epochHistory)
	return cp
}

// PrintPayoutTable formats a payout summary as a readable table.
func PrintPayoutTable(summary *EpochSummary) string {
	var s string
	s += fmt.Sprintf("Epoch #%d Reward Distribution\n", summary.Epoch)
	s += fmt.Sprintf("Total: %s IOTX | Delegate: %s | Agent Pool: %s\n",
		FormatIOTX(summary.TotalReward), FormatIOTX(summary.DelegateCut), FormatIOTX(summary.AgentPool))
	s += fmt.Sprintf("Eligible: %d agents | Tasks: %d\n\n", summary.AgentCount, summary.TotalTasks)

	s += fmt.Sprintf("%-24s %8s %8s %7s %8s %s\n",
		"Agent", "Tasks", "Acc %", "Share%", "IOTX", "Bonus")
	s += fmt.Sprintf("%-24s %8s %8s %7s %8s %s\n",
		"────────────────────────", "────────", "────────", "───────", "────────", "─────")

	for _, p := range summary.Payouts {
		bonus := ""
		if p.BonusApplied {
			bonus = "+20%"
		}
		s += fmt.Sprintf("%-24s %8d %7.1f%% %6.1f%% %8.2f %s\n",
			truncate(p.AgentID, 24), p.TasksDone, p.Accuracy*100, p.Share, p.AmountIOTX, bonus)
	}

	return s
}

// FormatIOTX formats a rau amount as a human-readable IOTX string.
func FormatIOTX(rau *big.Int) string {
	f := new(big.Float).SetInt(rau)
	f.Quo(f, new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)))
	result, _ := f.Float64()
	return fmt.Sprintf("%.2f", result)
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
