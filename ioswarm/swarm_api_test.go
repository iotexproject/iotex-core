package ioswarm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "github.com/iotexproject/iotex-core/ioswarm/proto"
	"go.uber.org/zap"
)

func newTestSwarmAPI() (*SwarmAPI, *Coordinator, *RewardDistributor) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	actPool := &mockActPool{}
	stateReader := &mockState{accounts: make(map[string]*pb.AccountSnapshot)}

	coord := NewCoordinator(cfg, actPool, stateReader, WithLogger(logger))
	reward := NewRewardDistributor(DefaultRewardConfig(), "", logger)
	api := NewSwarmAPI(coord, reward)
	return api, coord, reward
}

func TestSwarmAPIStatus(t *testing.T) {
	api, coord, _ := newTestSwarmAPI()

	// Register an agent
	coord.registry.Register(&pb.RegisterRequest{
		AgentID:    "ant-1",
		Capability: pb.TaskLevel_L2_STATE_VERIFY,
		Region:     "us-west",
		Version:    "1.0",
	})

	req := httptest.NewRequest("GET", "/swarm/status", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var status map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &status)

	if status["agents_total"].(float64) != 1 {
		t.Fatalf("expected 1 agent, got %v", status["agents_total"])
	}
	if status["shadow_mode"] != true {
		t.Fatal("expected shadow_mode=true")
	}
}

func TestSwarmAPIAgents(t *testing.T) {
	api, coord, _ := newTestSwarmAPI()

	coord.registry.Register(&pb.RegisterRequest{
		AgentID: "ant-1", Capability: pb.TaskLevel_L2_STATE_VERIFY,
		Region: "us-west", Version: "1.0",
	})
	coord.registry.Register(&pb.RegisterRequest{
		AgentID: "ant-2", Capability: pb.TaskLevel_L1_SIG_VERIFY,
		Region: "eu-central", Version: "1.0",
	})

	req := httptest.NewRequest("GET", "/swarm/agents", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["count"].(float64) != 2 {
		t.Fatalf("expected 2 agents, got %v", resp["count"])
	}
}

func TestSwarmAPILeaderboard(t *testing.T) {
	api, _, reward := newTestSwarmAPI()

	reward.RecordWork("ant-1", 100, 100, 5000)
	reward.RecordWork("ant-2", 200, 190, 8000)
	reward.RecordWork("ant-3", 50, 50, 2000)

	req := httptest.NewRequest("GET", "/swarm/leaderboard", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	entries := resp["entries"].([]interface{})
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// ant-2 should be first (most tasks)
	first := entries[0].(map[string]interface{})
	if first["agent_id"] != "ant-2" {
		t.Fatalf("expected ant-2 first, got %v", first["agent_id"])
	}
	if first["rank"].(float64) != 1 {
		t.Fatalf("expected rank 1, got %v", first["rank"])
	}
}

func TestSwarmAPIEpoch(t *testing.T) {
	api, _, reward := newTestSwarmAPI()

	reward.RecordWork("ant-1", 100, 95, 5000)

	req := httptest.NewRequest("GET", "/swarm/epoch", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["total_tasks"].(float64) != 100 {
		t.Fatalf("expected 100 tasks, got %v", resp["total_tasks"])
	}
	if resp["total_correct"].(float64) != 95 {
		t.Fatalf("expected 95 correct, got %v", resp["total_correct"])
	}
}

func TestSwarmAPIShadow(t *testing.T) {
	api, _, _ := newTestSwarmAPI()

	req := httptest.NewRequest("GET", "/swarm/shadow", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats ShadowStats
	json.Unmarshal(w.Body.Bytes(), &stats)
	// Fresh stats should be zeros
	if stats.TotalCompared != 0 {
		t.Fatalf("expected 0 compared, got %d", stats.TotalCompared)
	}
}

func TestSwarmAPIHealthzWithAgents(t *testing.T) {
	api, coord, _ := newTestSwarmAPI()

	coord.registry.Register(&pb.RegisterRequest{
		AgentID: "ant-1", Region: "local",
	})

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSwarmAPIHealthzNoAgents(t *testing.T) {
	api, _, _ := newTestSwarmAPI()

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
