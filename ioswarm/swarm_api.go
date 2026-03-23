package ioswarm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

// SwarmAPI serves HTTP endpoints for monitoring the IOSwarm.
// Endpoints:
//
//	GET /api/stats         — public network stats (CORS-enabled, no auth)
//	GET /swarm/status      — overall swarm status
//	GET /swarm/agents      — list all connected agents
//	GET /swarm/leaderboard — agents ranked by tasks processed
//	GET /swarm/epoch       — current epoch work stats
//	GET /swarm/shadow      — shadow mode comparison stats
//	GET /healthz           — health check
type SwarmAPI struct {
	coord    *Coordinator
	reward   *RewardDistributor
	startAt  time.Time
}

// NewSwarmAPI creates a new swarm API.
func NewSwarmAPI(coord *Coordinator, reward *RewardDistributor) *SwarmAPI {
	return &SwarmAPI{
		coord:   coord,
		reward:  reward,
		startAt: time.Now(),
	}
}

// Handler returns an http.Handler with all swarm routes registered.
func (s *SwarmAPI) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/stats", cors(s.handlePublicStats))
	mux.HandleFunc("/api/rewards", cors(s.handleRewards))
	mux.HandleFunc("/swarm/status", s.handleStatus)
	mux.HandleFunc("/swarm/agents", s.handleAgents)
	mux.HandleFunc("/swarm/leaderboard", s.handleLeaderboard)
	mux.HandleFunc("/swarm/epoch", s.handleEpoch)
	mux.HandleFunc("/swarm/shadow", s.handleShadow)
	mux.HandleFunc("/healthz", s.handleHealthz)
	return mux
}

// cors wraps a handler with permissive CORS headers for public endpoints.
func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// handlePublicStats returns a summary of the swarm for community dashboards.
// This endpoint is unauthenticated and CORS-enabled.
func (s *SwarmAPI) handlePublicStats(w http.ResponseWriter, r *http.Request) {
	agents := s.coord.registry.LiveAgents(60 * time.Second)
	shadow := s.coord.ShadowStats()

	// Count agents by level
	byLevel := make(map[string]int)
	for _, a := range agents {
		byLevel[levelStr(a.Capability)]++
	}

	// Aggregate epoch work
	work := s.reward.CurrentWork()
	var totalTasks, totalCorrect uint64
	for _, w := range work {
		totalTasks += w.TasksProcessed
		totalCorrect += w.TasksCorrect
	}

	stats := map[string]interface{}{
		"active_agents":    len(agents),
		"agents_by_level":  byLevel,
		"tasks_dispatched": s.coord.totalDispatched.Load(),
		"tasks_received":   s.coord.totalReceived.Load(),
		"shadow_accuracy":  shadowAccuracy(shadow),
		"shadow_compared":  shadow.TotalCompared,
		"shadow_matched":   shadow.TotalMatched,
		"current_epoch":    s.reward.CurrentEpoch(),
		"epoch_tasks":      totalTasks,
		"epoch_accuracy":   epochAccuracy(totalTasks, totalCorrect),
		"task_level":       s.coord.cfg.TaskLevel,
		"uptime":           time.Since(s.startAt).Round(time.Second).String(),
		"updated_at":       time.Now().Unix(),
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, stats)
}

func (s *SwarmAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	agents := s.coord.registry.LiveAgents(60 * time.Second)
	shadow := s.coord.ShadowStats()

	// Count by region
	regions := make(map[string]int)
	for _, a := range agents {
		regions[a.Region]++
	}

	status := map[string]interface{}{
		"uptime":           time.Since(s.startAt).String(),
		"agents_total":     s.coord.registry.Count(),
		"agents_live":      len(agents),
		"tasks_dispatched": s.coord.totalDispatched.Load(),
		"tasks_received":   s.coord.totalReceived.Load(),
		"shadow_mode":      s.coord.cfg.ShadowMode,
		"shadow_accuracy":  shadowAccuracy(shadow),
		"task_level":       s.coord.cfg.TaskLevel,
		"regions":          regions,
	}

	writeJSON(w, status)
}

func (s *SwarmAPI) handleAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.coord.registry.LiveAgents(120 * time.Second)

	type agentView struct {
		ID             string  `json:"id"`
		Region         string  `json:"region"`
		Version        string  `json:"version"`
		Level          string  `json:"level"`
		TasksProcessed uint32  `json:"tasks_processed"`
		TasksPending   uint32  `json:"tasks_pending"`
		CPUUsage       float64 `json:"cpu_usage"`
		MemUsage       float64 `json:"mem_usage"`
		LastHeartbeat  string  `json:"last_heartbeat"`
		Uptime         string  `json:"uptime"`
	}

	views := make([]agentView, 0, len(agents))
	for _, a := range agents {
		views = append(views, agentView{
			ID:             a.ID,
			Region:         a.Region,
			Version:        a.Version,
			Level:          levelStr(a.Capability),
			TasksProcessed: a.TasksProcessed,
			TasksPending:   a.TasksPending,
			CPUUsage:       a.CPUUsage,
			MemUsage:       a.MemUsage,
			LastHeartbeat:  a.LastHeartbeat.Format(time.RFC3339),
			Uptime:         time.Since(a.RegisteredAt).Round(time.Second).String(),
		})
	}

	// Sort by tasks processed (descending)
	sort.Slice(views, func(i, j int) bool {
		return views[i].TasksProcessed > views[j].TasksProcessed
	})

	writeJSON(w, map[string]interface{}{
		"count":  len(views),
		"agents": views,
	})
}

func (s *SwarmAPI) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	work := s.reward.CurrentWork()

	type entry struct {
		Rank     int     `json:"rank"`
		AgentID  string  `json:"agent_id"`
		Tasks    uint64  `json:"tasks"`
		Accuracy float64 `json:"accuracy"`
		AvgLatUs float64 `json:"avg_latency_us"`
		Wallet   string  `json:"wallet,omitempty"`
	}

	entries := make([]entry, 0, len(work))
	for _, w := range work {
		avgLat := float64(0)
		if w.TasksProcessed > 0 {
			avgLat = float64(w.TotalLatencyUs) / float64(w.TasksProcessed)
		}
		entries = append(entries, entry{
			AgentID:  w.AgentID,
			Tasks:    w.TasksProcessed,
			Accuracy: w.Accuracy(),
			AvgLatUs: avgLat,
			Wallet:   w.WalletAddress,
		})
	}

	// Sort by tasks descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Tasks > entries[j].Tasks
	})

	// Assign ranks
	for i := range entries {
		entries[i].Rank = i + 1
	}

	writeJSON(w, map[string]interface{}{
		"epoch":   s.reward.CurrentEpoch(),
		"agents":  len(entries),
		"entries": entries,
	})
}

func (s *SwarmAPI) handleEpoch(w http.ResponseWriter, r *http.Request) {
	work := s.reward.CurrentWork()

	var totalTasks, totalCorrect uint64
	for _, w := range work {
		totalTasks += w.TasksProcessed
		totalCorrect += w.TasksCorrect
	}

	accuracy := float64(0)
	if totalTasks > 0 {
		accuracy = float64(totalCorrect) / float64(totalTasks)
	}

	writeJSON(w, map[string]interface{}{
		"epoch":         s.reward.CurrentEpoch(),
		"agents":        len(work),
		"total_tasks":   totalTasks,
		"total_correct": totalCorrect,
		"accuracy":      accuracy,
		"elapsed":       s.reward.EpochElapsed().String(),
	})
}

func (s *SwarmAPI) handleShadow(w http.ResponseWriter, r *http.Request) {
	stats := s.coord.ShadowStats()
	writeJSON(w, stats)
}

func (s *SwarmAPI) handleRewards(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent")
	if agentID == "" {
		http.Error(w, `{"error":"agent parameter required"}`, http.StatusBadRequest)
		return
	}

	history := s.reward.EpochHistory()
	type entry struct {
		Epoch      uint64  `json:"epoch"`
		AmountIOTX float64 `json:"amount_iotx"`
		Tasks      uint64  `json:"tasks"`
		Accuracy   float64 `json:"accuracy_pct"`
		Rank       int     `json:"rank"`
		Bonus      bool    `json:"bonus_applied"`
	}
	entries := make([]entry, 0)
	var totalEarned float64
	var totalTasks uint64
	var totalAccuracy float64
	for _, epoch := range history {
		for rank, p := range epoch.Payouts {
			if p.AgentID == agentID {
				entries = append(entries, entry{
					Epoch:      epoch.Epoch,
					AmountIOTX: p.AmountIOTX,
					Tasks:      p.TasksDone,
					Accuracy:   p.Accuracy,
					Rank:       rank + 1,
					Bonus:      p.BonusApplied,
				})
				totalEarned += p.AmountIOTX
				totalTasks += p.TasksDone
				totalAccuracy += p.Accuracy
			}
		}
	}

	avgAccuracy := float64(0)
	if len(entries) > 0 {
		avgAccuracy = totalAccuracy / float64(len(entries))
	}

	writeJSON(w, map[string]interface{}{
		"agent_id": agentID,
		"history":  entries,
		"totals": map[string]interface{}{
			"earned_iotx":  totalEarned,
			"total_tasks":  totalTasks,
			"avg_accuracy": avgAccuracy,
		},
	})
}

func (s *SwarmAPI) handleHealthz(w http.ResponseWriter, r *http.Request) {
	liveCount := len(s.coord.registry.LiveAgents(60 * time.Second))
	if liveCount > 0 {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok (%d agents)", liveCount)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("no live agents"))
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func levelStr(l pb.TaskLevel) string {
	switch l {
	case pb.TaskLevel_L1_SIG_VERIFY:
		return "L1"
	case pb.TaskLevel_L2_STATE_VERIFY:
		return "L2"
	case pb.TaskLevel_L3_FULL_EXECUTE:
		return "L3"
	case pb.TaskLevel_L4_STATE_SYNC:
		return "L4"
	default:
		return "unknown"
	}
}

func shadowAccuracy(s ShadowStats) float64 {
	if s.TotalCompared == 0 {
		return 0
	}
	return float64(s.TotalMatched) / float64(s.TotalCompared) * 100
}

func epochAccuracy(total, correct uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(correct) / float64(total) * 100
}
