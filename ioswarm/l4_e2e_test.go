package ioswarm

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestL4StateDiffFullPipeline tests the complete L4 state diff pipeline:
//
//  1. Coordinator with StateDiffBroadcaster
//  2. Simulate block commits by publishing state diffs (as the stateDB callback would)
//  3. Agent subscribes via gRPC StreamStateDiffs
//  4. Agent receives and verifies all diffs in order
//
// This simulates the exact flow that would happen in production:
//
//	stateDB.PutBlock() → diffCallback → coordinator.ReceiveStateDiff()
//	  → broadcaster.Publish() → gRPC stream → agent BoltDB
func TestL4StateDiffFullPipeline(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := Config{
		Enabled:        true,
		MaxAgents:      10,
		TaskLevel:      "L3",
		ShadowMode:     true,
		PollIntervalMS: 500,
		DiffBufferSize: 100,
	}

	actPool := &mockActPool{height: 1}
	stateDB := &mockState{
		accounts: map[string]*pb.AccountSnapshot{
			"io1alice": {Address: "io1alice", Balance: "5000000000000000000", Nonce: 0},
		},
	}

	registry := NewRegistry(cfg.MaxAgents)
	coord := &Coordinator{
		cfg:             cfg,
		actPool:         actPool,
		prefetcher:      NewPrefetcher(stateDB, logger),
		registry:        registry,
		scheduler:       NewScheduler(registry, logger),
		shadow:          NewShadowComparator(logger),
		reward:          NewRewardDistributor(DefaultRewardConfig(), "", logger),
		diffBroadcaster: NewStateDiffBroadcaster(cfg.DiffBufferSize, logger),
		logger:          logger,
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := lis.Addr().(*net.TCPAddr).Port

	grpcSrv := grpc.NewServer()
	RegisterIOSwarmServer(grpcSrv, &grpcHandler{coord: coord})
	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop()

	t.Logf("coordinator gRPC on :%d", port)

	// Simulate block production: 20 blocks with realistic state diffs
	// Each block has Account puts (nonce/balance updates) + sometimes Contract storage puts
	totalBlocks := 20
	var blocksDone atomic.Int32

	go func() {
		for h := uint64(1); h <= uint64(totalBlocks); h++ {
			entries := []StateDiffEntry{
				// Every block: update account nonce/balance
				{Type: 0, Namespace: "Account", Key: []byte("io1alice"), Value: []byte(fmt.Sprintf("nonce=%d,balance=4999999999999999999", h))},
				// Block producer reward account
				{Type: 0, Namespace: "Account", Key: []byte("io1rewardpool"), Value: []byte(fmt.Sprintf("balance=%d", 16*h))},
			}
			// Every 5th block: contract storage change
			if h%5 == 0 {
				entries = append(entries, StateDiffEntry{
					Type:      0,
					Namespace: "Contract",
					Key:       []byte(fmt.Sprintf("io1contract:slot%d", h/5)),
					Value:     []byte(fmt.Sprintf("value-%d", h)),
				})
			}
			// Height 10: deploy a contract (Code namespace)
			if h == 10 {
				entries = append(entries, StateDiffEntry{
					Type:      0,
					Namespace: "Code",
					Key:       []byte("0xdeadbeef"),
					Value:     []byte{0x60, 0x80, 0x60, 0x40, 0x52}, // minimal EVM bytecode
				})
			}

			coord.ReceiveStateDiff(h, entries, []byte(fmt.Sprintf("block-digest-%d", h)))
			blocksDone.Add(1)
			time.Sleep(50 * time.Millisecond) // simulate 50ms block interval
		}
	}()

	// Wait a bit for some blocks to be produced before agent connects
	time.Sleep(200 * time.Millisecond)

	// Agent connects and opens StreamStateDiffs from height 0 (get everything)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("agent connect failed: %v", err)
	}
	defer conn.Close()

	streamDesc := &grpc.StreamDesc{StreamName: "StreamStateDiffs", ServerStreams: true}
	stream, err := conn.NewStream(ctx, streamDesc, "/ioswarm.IOSwarm/StreamStateDiffs")
	if err != nil {
		t.Fatalf("open stream failed: %v", err)
	}

	req := &pb.StreamStateDiffsRequest{
		AgentID:    "l4-agent-001",
		FromHeight: 1,
	}
	if err := stream.SendMsg(req); err != nil {
		t.Fatalf("send request failed: %v", err)
	}
	stream.CloseSend()

	// Receive all 20 blocks worth of diffs
	var (
		received     []*pb.StateDiffResponse
		totalEntries int
		lastHeight   uint64
	)

	for len(received) < totalBlocks {
		resp := &pb.StateDiffResponse{}
		if err := stream.RecvMsg(resp); err != nil {
			t.Fatalf("recv failed after %d diffs: %v", len(received), err)
		}
		received = append(received, resp)
		totalEntries += len(resp.Entries)
		lastHeight = resp.Height
	}

	// Verify results
	t.Logf("received %d diffs, %d total entries, last height=%d",
		len(received), totalEntries, lastHeight)

	// Check ordering: heights must be monotonically increasing
	for i := 1; i < len(received); i++ {
		if received[i].Height <= received[i-1].Height {
			t.Errorf("height not increasing: diff[%d]=%d, diff[%d]=%d",
				i-1, received[i-1].Height, i, received[i].Height)
		}
	}

	// Check first diff
	if received[0].Height != 1 {
		t.Errorf("first diff height: expected 1, got %d", received[0].Height)
	}

	// Check last diff
	if lastHeight != uint64(totalBlocks) {
		t.Errorf("last diff height: expected %d, got %d", totalBlocks, lastHeight)
	}

	// Verify block 10 has Code entry
	block10 := received[9]
	hasCode := false
	for _, e := range block10.Entries {
		if e.Namespace == "Code" {
			hasCode = true
			if string(e.Key) != "0xdeadbeef" {
				t.Errorf("block 10 Code key: expected 0xdeadbeef, got %s", string(e.Key))
			}
		}
	}
	if !hasCode {
		t.Error("block 10 should have Code entry (contract deploy)")
	}

	// Verify every 5th block has Contract entry
	for i, diff := range received {
		h := diff.Height
		if h%5 == 0 {
			hasContract := false
			for _, e := range diff.Entries {
				if e.Namespace == "Contract" {
					hasContract = true
				}
			}
			if !hasContract {
				t.Errorf("diff[%d] height=%d should have Contract entry", i, h)
			}
		}
	}

	// Verify digest bytes are present
	for i, diff := range received {
		if len(diff.DigestBytes) == 0 {
			t.Errorf("diff[%d] height=%d missing digest bytes", i, diff.Height)
		}
	}

	// Stats check
	if coord.DiffBroadcaster().LatestHeight() != uint64(totalBlocks) {
		t.Errorf("broadcaster latest height: expected %d, got %d",
			totalBlocks, coord.DiffBroadcaster().LatestHeight())
	}

	t.Logf("L4 full pipeline test passed: %d blocks, %d entries, heights 1-%d",
		len(received), totalEntries, lastHeight)
}
