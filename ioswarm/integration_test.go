package ioswarm

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// secp256k1N is the order of the secp256k1 curve used by IoTeX/Ethereum signatures.
var secp256k1N, _ = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

// --- Mock actpool and state reader ---

type mockActPool struct {
	mu     sync.Mutex
	txs    []*PendingTx
	height uint64
}

func (m *mockActPool) PendingActions() []*PendingTx {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*PendingTx, len(m.txs))
	copy(cp, m.txs)
	return cp
}

func (m *mockActPool) BlockHeight() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.height
}

func (m *mockActPool) AddTx(tx *PendingTx) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs = append(m.txs, tx)
}

type mockState struct {
	accounts map[string]*pb.AccountSnapshot
}

func (m *mockState) AccountState(address string) (*pb.AccountSnapshot, error) {
	if snap, ok := m.accounts[address]; ok {
		return snap, nil
	}
	return &pb.AccountSnapshot{Address: address, Balance: "0", Nonce: 0}, nil
}

func (m *mockState) GetCode(address string) ([]byte, error)           { return nil, nil }
func (m *mockState) GetStorageAt(address, slot string) (string, error) { return "", nil }

// buildMockTxRaw creates a fake serialized tx with valid-looking ECDSA sig.
func buildMockTxRaw() []byte {
	payload := make([]byte, 80)
	rand.Read(payload)

	r, _ := rand.Int(rand.Reader, secp256k1N)
	s, _ := rand.Int(rand.Reader, secp256k1N)

	raw := append(payload, r.FillBytes(make([]byte, 32))...)
	raw = append(raw, s.FillBytes(make([]byte, 32))...)
	raw = append(raw, byte(0))
	return raw
}

// --- Integration test: full gRPC round-trip ---

func TestIntegrationE2E(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 1. Mock data
	actPool := &mockActPool{height: 1000}
	stateDB := &mockState{
		accounts: map[string]*pb.AccountSnapshot{
			"io1alice": {Address: "io1alice", Balance: "5000000000000000000", Nonce: 10},
			"io1bob":   {Address: "io1bob", Balance: "1000000000000000000", Nonce: 3},
		},
	}

	for i := 0; i < 5; i++ {
		actPool.AddTx(&PendingTx{
			Hash:     fmt.Sprintf("tx-%d", i),
			From:     "io1alice",
			To:       "io1bob",
			Nonce:    uint64(10 + i),
			Amount:   "100",
			GasLimit: 21000,
			GasPrice: "1000000000",
			RawBytes: buildMockTxRaw(),
		})
	}

	// 2. Start coordinator gRPC server
	cfg := Config{
		Enabled:        true,
		MaxAgents:      10,
		TaskLevel:      "L2",
		ShadowMode:     true,
		PollIntervalMS: 100,
	}

	registry := NewRegistry(cfg.MaxAgents)
	coord := &Coordinator{
		cfg:        cfg,
		actPool:    actPool,
		prefetcher: NewPrefetcher(stateDB, logger),
		registry:   registry,
		scheduler:  NewScheduler(registry, logger),
		shadow:     NewShadowComparator(logger),
		reward:     NewRewardDistributor(DefaultRewardConfig(), "", logger),
		logger:     logger,
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := lis.Addr().(*net.TCPAddr).Port

	grpcSrv := grpc.NewServer()
	RegisterIOSwarmServer(grpcSrv, &grpcHandler{coord: coord})
	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop() // use Stop (not GracefulStop) to avoid hanging on open streams

	t.Logf("coordinator gRPC listening on port %d", port)

	// 3. Connect agent (simulated gRPC client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("agent connect failed: %v", err)
	}
	defer conn.Close()

	// 4. Register agent
	regResp := &pb.RegisterResponse{}
	err = conn.Invoke(ctx, "/ioswarm.IOSwarm/Register", &pb.RegisterRequest{
		AgentID:    "test-ant-1",
		Capability: pb.TaskLevel_L2_STATE_VERIFY,
		Region:     "test",
		Version:    "1.0-test",
	}, regResp)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if !regResp.Accepted {
		t.Fatalf("registration rejected: %s", regResp.Reason)
	}
	t.Logf("agent registered, heartbeat interval: %ds", regResp.HeartbeatIntervalSec)

	// 5. Verify agent is in registry
	if registry.Count() != 1 {
		t.Fatalf("expected 1 agent in registry, got %d", registry.Count())
	}

	// 6. Trigger coordinator dispatch (manually call pollAndDispatch)
	coord.pollAndDispatch()

	// 7. Agent opens task stream and receives batch
	streamDesc := &grpc.StreamDesc{StreamName: "GetTasks", ServerStreams: true}
	stream, err := conn.NewStream(ctx, streamDesc, "/ioswarm.IOSwarm/GetTasks")
	if err != nil {
		t.Fatalf("open task stream failed: %v", err)
	}
	if err := stream.SendMsg(&pb.GetTasksRequest{
		AgentID:      "test-ant-1",
		MaxLevel:     pb.TaskLevel_L2_STATE_VERIFY,
		MaxBatchSize: 10,
	}); err != nil {
		t.Fatalf("send GetTasksRequest failed: %v", err)
	}
	stream.CloseSend()

	// Read batch from stream (should have been dispatched by pollAndDispatch)
	batch := &pb.TaskBatch{}
	err = stream.RecvMsg(batch)
	if err != nil {
		t.Fatalf("receive batch failed: %v", err)
	}

	t.Logf("received batch %s with %d tasks", batch.BatchID, len(batch.Tasks))

	if len(batch.Tasks) == 0 {
		t.Fatal("expected non-empty task batch")
	}

	// 8. Agent processes tasks (simulate worker)
	results := make([]*pb.TaskResult, 0, len(batch.Tasks))
	for _, task := range batch.Tasks {
		valid := true
		reason := ""

		// L1: basic sig check
		if len(task.TxRaw) < 65 {
			valid = false
			reason = "tx too short"
		} else {
			sigStart := len(task.TxRaw) - 65
			r := new(big.Int).SetBytes(task.TxRaw[sigStart : sigStart+32])
			s := new(big.Int).SetBytes(task.TxRaw[sigStart+32 : sigStart+64])
			if r.Sign() == 0 || s.Sign() == 0 || r.Cmp(secp256k1N) >= 0 || s.Cmp(secp256k1N) >= 0 {
				valid = false
				reason = "invalid sig"
			}
		}

		// L2: state check
		if valid && task.Sender != nil {
			bal, ok := new(big.Int).SetString(task.Sender.Balance, 10)
			if !ok || bal.Sign() < 0 {
				valid = false
				reason = "invalid balance"
			}
		}

		results = append(results, &pb.TaskResult{
			TaskID:       task.TaskID,
			Valid:        valid,
			RejectReason: reason,
			GasEstimate:  21000,
			LatencyUs:    50,
		})
	}

	t.Logf("agent processed %d tasks", len(results))

	// 9. Submit results back
	submitResp := &pb.SubmitResponse{}
	err = conn.Invoke(ctx, "/ioswarm.IOSwarm/SubmitResults", &pb.BatchResult{
		AgentID:   "test-ant-1",
		BatchID:   batch.BatchID,
		Results:   results,
		Timestamp: uint64(time.Now().UnixMilli()),
	}, submitResp)
	if err != nil {
		t.Fatalf("submit results failed: %v", err)
	}
	if !submitResp.Accepted {
		t.Fatalf("results rejected: %s", submitResp.Reason)
	}
	t.Log("results accepted by coordinator")

	// 10. Heartbeat
	hbResp := &pb.HeartbeatResponse{}
	err = conn.Invoke(ctx, "/ioswarm.IOSwarm/Heartbeat", &pb.HeartbeatRequest{
		AgentID:        "test-ant-1",
		TasksProcessed: uint32(len(results)),
		CPUUsage:       0.25,
		MemUsage:       0.10,
	}, hbResp)
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if !hbResp.Alive {
		t.Fatal("expected alive=true")
	}
	if hbResp.Directive != "continue" {
		t.Fatalf("expected directive=continue, got %s", hbResp.Directive)
	}
	t.Log("heartbeat OK, directive: continue")

	// 11. Shadow comparison: simulate actual block execution
	//     All txs from io1alice with positive balance should be valid
	actualResults := make(map[uint32]bool)
	for _, task := range batch.Tasks {
		actualResults[task.TaskID] = true
	}

	mismatches := coord.shadow.CompareWithActual(actualResults, 1001)
	if len(mismatches) != 0 {
		t.Fatalf("expected 0 shadow mismatches, got %d", len(mismatches))
	}

	stats := coord.shadow.Stats()
	t.Logf("shadow stats: %d compared, %d matched, %.1f%% accuracy",
		stats.TotalCompared, stats.TotalMatched,
		float64(stats.TotalMatched)/float64(stats.TotalCompared)*100)

	if stats.TotalCompared == 0 {
		t.Fatal("expected shadow comparison to have run")
	}
	if stats.TotalMatched != stats.TotalCompared {
		t.Fatalf("expected 100%% match, got %d/%d", stats.TotalMatched, stats.TotalCompared)
	}

	// 12. Verify agent stats updated via heartbeat
	agent, ok := registry.GetAgent("test-ant-1")
	if !ok {
		t.Fatal("agent not found in registry")
	}
	if agent.TasksProcessed != uint32(len(results)) {
		t.Fatalf("expected %d tasks processed, got %d", len(results), agent.TasksProcessed)
	}

	t.Log("=== Integration test PASSED ===")
	t.Logf("  Transactions: %d", len(batch.Tasks))
	t.Logf("  All validated: true")
	t.Logf("  Shadow accuracy: 100%%")
	t.Logf("  Agent heartbeat: OK")
}
