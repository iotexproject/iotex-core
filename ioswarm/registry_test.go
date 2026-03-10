package ioswarm

import (
	"testing"
	"time"

	pb "github.com/iotexproject/iotex-core/ioswarm/proto"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry(3)

	req := &pb.RegisterRequest{
		AgentID:    "ant-1",
		Capability: pb.TaskLevel_L2_STATE_VERIFY,
		Region:     "us-west",
		Version:    "1.0",
	}

	agent, ok := r.Register(req)
	if !ok {
		t.Fatal("expected registration to succeed")
	}
	if agent.ID != "ant-1" {
		t.Fatalf("expected agent ID ant-1, got %s", agent.ID)
	}
	if r.Count() != 1 {
		t.Fatalf("expected 1 agent, got %d", r.Count())
	}

	// Get
	got, ok := r.GetAgent("ant-1")
	if !ok || got.ID != "ant-1" {
		t.Fatal("GetAgent failed")
	}

	// Not found
	_, ok = r.GetAgent("ant-999")
	if ok {
		t.Fatal("expected GetAgent to return false for unknown agent")
	}
}

func TestRegistryCapacity(t *testing.T) {
	r := NewRegistry(2)

	r.Register(&pb.RegisterRequest{AgentID: "a1"})
	r.Register(&pb.RegisterRequest{AgentID: "a2"})

	_, ok := r.Register(&pb.RegisterRequest{AgentID: "a3"})
	if ok {
		t.Fatal("expected registration to fail at capacity")
	}
	if r.Count() != 2 {
		t.Fatalf("expected 2 agents, got %d", r.Count())
	}
}

func TestRegistryReconnect(t *testing.T) {
	r := NewRegistry(2)

	r.Register(&pb.RegisterRequest{AgentID: "a1", Region: "us"})
	r.Register(&pb.RegisterRequest{AgentID: "a2"})

	// Reconnect a1 — should succeed even at capacity
	agent, ok := r.Register(&pb.RegisterRequest{AgentID: "a1", Region: "eu"})
	if !ok {
		t.Fatal("expected reconnection to succeed")
	}
	if agent.Region != "eu" {
		t.Fatal("expected region to be updated")
	}
	if r.Count() != 2 {
		t.Fatalf("expected 2 agents after reconnect, got %d", r.Count())
	}
}

func TestRegistryHeartbeat(t *testing.T) {
	r := NewRegistry(10)
	r.Register(&pb.RegisterRequest{AgentID: "ant-1"})

	ok := r.Heartbeat(&pb.HeartbeatRequest{
		AgentID:        "ant-1",
		TasksProcessed: 42,
		CPUUsage:       0.5,
	})
	if !ok {
		t.Fatal("expected heartbeat to succeed")
	}

	agent, _ := r.GetAgent("ant-1")
	if agent.TasksProcessed != 42 {
		t.Fatalf("expected 42 tasks, got %d", agent.TasksProcessed)
	}

	// Unknown agent
	ok = r.Heartbeat(&pb.HeartbeatRequest{AgentID: "unknown"})
	if ok {
		t.Fatal("expected heartbeat to fail for unknown agent")
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := NewRegistry(10)
	r.Register(&pb.RegisterRequest{AgentID: "ant-1"})

	r.Unregister("ant-1")
	if r.Count() != 0 {
		t.Fatal("expected 0 agents after unregister")
	}

	// Double unregister should not panic
	r.Unregister("ant-1")
}

func TestRegistryEvictStale(t *testing.T) {
	r := NewRegistry(10)
	r.Register(&pb.RegisterRequest{AgentID: "fresh"})

	// Manually set old heartbeat for a stale agent
	r.Register(&pb.RegisterRequest{AgentID: "stale"})
	r.mu.Lock()
	r.agents["stale"].LastHeartbeat = time.Now().Add(-5 * time.Minute)
	r.mu.Unlock()

	evicted := r.EvictStale(1 * time.Minute)
	if len(evicted) != 1 || evicted[0] != "stale" {
		t.Fatalf("expected stale to be evicted, got %v", evicted)
	}
	if r.Count() != 1 {
		t.Fatal("expected 1 agent remaining")
	}
}

func TestRegistryLiveAgents(t *testing.T) {
	r := NewRegistry(10)
	r.Register(&pb.RegisterRequest{AgentID: "a1"})
	r.Register(&pb.RegisterRequest{AgentID: "a2"})

	// Make a2 stale
	r.mu.Lock()
	r.agents["a2"].LastHeartbeat = time.Now().Add(-5 * time.Minute)
	r.mu.Unlock()

	live := r.LiveAgents(1 * time.Minute)
	if len(live) != 1 || live[0].ID != "a1" {
		t.Fatalf("expected only a1 as live, got %v", live)
	}
}
