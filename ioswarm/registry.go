package ioswarm

import (
	"sync"
	"time"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

// AgentInfo tracks a connected agent's state.
type AgentInfo struct {
	ID             string
	Capability     pb.TaskLevel
	Region         string
	Version        string
	RegisteredAt   time.Time
	LastHeartbeat  time.Time
	TasksProcessed uint32
	TasksPending   uint32
	CPUUsage       float64
	MemUsage       float64
	TaskChan       chan *pb.TaskBatch // channel to push tasks to this agent's stream
	closeOnce      sync.Once         // prevent double-close panic
}

// CloseTaskChan safely closes the TaskChan exactly once.
func (a *AgentInfo) CloseTaskChan() {
	a.closeOnce.Do(func() {
		close(a.TaskChan)
	})
}

// Registry tracks all connected agents.
type Registry struct {
	mu        sync.RWMutex
	agents    map[string]*AgentInfo
	maxAgents int
}

// NewRegistry creates a new agent registry.
func NewRegistry(maxAgents int) *Registry {
	return &Registry{
		agents:    make(map[string]*AgentInfo),
		maxAgents: maxAgents,
	}
}

// Register adds an agent to the registry. Returns false if at capacity.
func (r *Registry) Register(req *pb.RegisterRequest) (*AgentInfo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If agent is reconnecting, reuse the existing channel so that
	// any active GetTasks stream continues without interruption.
	if old, ok := r.agents[req.AgentID]; ok {
		old.Capability = req.Capability
		old.Region = req.Region
		old.Version = req.Version
		old.LastHeartbeat = time.Now()
		return old, true
	}

	if len(r.agents) >= r.maxAgents {
		return nil, false
	}

	agent := &AgentInfo{
		ID:            req.AgentID,
		Capability:    req.Capability,
		Region:        req.Region,
		Version:       req.Version,
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
		TaskChan:      make(chan *pb.TaskBatch, 16),
	}
	r.agents[req.AgentID] = agent
	return agent, true
}

// Unregister removes an agent from the registry.
func (r *Registry) Unregister(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agent, ok := r.agents[agentID]; ok {
		agent.CloseTaskChan()
		delete(r.agents, agentID)
	}
}

// Heartbeat updates an agent's last heartbeat time and stats.
func (r *Registry) Heartbeat(req *pb.HeartbeatRequest) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, ok := r.agents[req.AgentID]
	if !ok {
		return false
	}

	agent.LastHeartbeat = time.Now()
	agent.TasksProcessed = req.TasksProcessed
	agent.TasksPending = req.TasksPending
	agent.CPUUsage = req.CPUUsage
	agent.MemUsage = req.MemUsage
	return true
}

// GetAgent returns agent info by ID.
func (r *Registry) GetAgent(agentID string) (*AgentInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[agentID]
	return agent, ok
}

// LiveAgents returns all agents with a heartbeat within the given threshold.
func (r *Registry) LiveAgents(threshold time.Duration) []*AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cutoff := time.Now().Add(-threshold)
	var live []*AgentInfo
	for _, agent := range r.agents {
		if agent.LastHeartbeat.After(cutoff) {
			live = append(live, agent)
		}
	}
	return live
}

// Count returns the number of registered agents.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// EvictStale removes agents that haven't sent a heartbeat within the threshold.
func (r *Registry) EvictStale(threshold time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-threshold)
	var evicted []string
	for id, agent := range r.agents {
		if agent.LastHeartbeat.Before(cutoff) {
			agent.CloseTaskChan()
			delete(r.agents, id)
			evicted = append(evicted, id)
		}
	}
	return evicted
}

// ============== L5-specific Registry Methods ==============

// L5AgentInfo tracks an L5 agent's state.
type L5AgentInfo struct {
	ID            string
	WalletAddress string
	Region        string
	Version       string
	RegisteredAt  time.Time
	LastHeartbeat time.Time
	BlocksBuilt   uint64
	BlocksAccepted uint64
	LastBuiltHeight uint64
	CPUUsage      float64
	MemUsage      float64
}

// RegisterL5 registers an L5 agent with individual parameters.
func (r *Registry) RegisterL5(agentID, walletAddr, region, version string) *L5AgentInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if agent already exists
	if agent, ok := r.agents[agentID]; ok {
		agent.Region = region
		agent.Version = version
		agent.LastHeartbeat = time.Now()
		return &L5AgentInfo{
			ID:            agent.ID,
			WalletAddress: walletAddr,
			Region:        agent.Region,
			Version:       agent.Version,
			RegisteredAt:  agent.RegisteredAt,
			LastHeartbeat: agent.LastHeartbeat,
		}
	}

	if len(r.agents) >= r.maxAgents {
		return nil
	}

	agent := &AgentInfo{
		ID:           agentID,
		Region:       region,
		Version:      version,
		RegisteredAt: time.Now(),
		LastHeartbeat: time.Now(),
		TaskChan:     make(chan *pb.TaskBatch, 16),
	}
	r.agents[agentID] = agent
	return &L5AgentInfo{
		ID:            agentID,
		WalletAddress: walletAddr,
		Region:        region,
		Version:       version,
		RegisteredAt:  agent.RegisteredAt,
		LastHeartbeat: agent.LastHeartbeat,
	}
}

// HeartbeatL5 updates an L5 agent's heartbeat time.
func (r *Registry) HeartbeatL5(agentID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, ok := r.agents[agentID]
	if !ok {
		return false
	}

	agent.LastHeartbeat = time.Now()
	return true
}

// UpdateMetrics updates an agent's resource metrics.
func (r *Registry) UpdateMetrics(agentID string, cpuUsage, memUsage float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agent, ok := r.agents[agentID]; ok {
		agent.CPUUsage = cpuUsage
		agent.MemUsage = memUsage
	}
}
