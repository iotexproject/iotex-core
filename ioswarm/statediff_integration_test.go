package ioswarm

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestStateDiffStreamE2E tests the full state diff pipeline:
// Coordinator publishes diffs → gRPC StreamStateDiffs → agent receives them.
func TestStateDiffStreamE2E(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 1. Set up coordinator with broadcaster
	cfg := Config{
		Enabled:        true,
		MaxAgents:      10,
		TaskLevel:      "L2",
		ShadowMode:     true,
		PollIntervalMS: 100,
		DiffBufferSize: 50,
	}

	actPool := &mockActPool{height: 1000}
	stateDB := &mockState{
		accounts: map[string]*pb.AccountSnapshot{
			"io1alice": {Address: "io1alice", Balance: "5000000000000000000", Nonce: 10},
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

	// 2. Start gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := lis.Addr().(*net.TCPAddr).Port

	grpcSrv := grpc.NewServer()
	RegisterIOSwarmServer(grpcSrv, &grpcHandler{coord: coord})
	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop()

	t.Logf("gRPC server on port %d", port)

	// 3. Pre-publish some diffs (so catch-up works)
	for h := uint64(1); h <= 5; h++ {
		coord.ReceiveStateDiff(h, []StateDiffEntry{
			{Type: 0, Namespace: "Account", Key: []byte("io1alice"), Value: []byte(fmt.Sprintf("balance-%d", h))},
			{Type: 0, Namespace: "Contract", Key: []byte("slot-" + fmt.Sprintf("%d", h)), Value: []byte("val")},
		}, []byte(fmt.Sprintf("digest-%d", h)))
	}

	// 4. Connect as agent and open StreamStateDiffs
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer conn.Close()

	streamDesc := &grpc.StreamDesc{StreamName: "StreamStateDiffs", ServerStreams: true}
	stream, err := conn.NewStream(ctx, streamDesc, "/ioswarm.IOSwarm/StreamStateDiffs")
	if err != nil {
		t.Fatalf("open stream failed: %v", err)
	}

	req := &pb.StreamStateDiffsRequest{
		AgentID:    "test-l4-agent",
		FromHeight: 1, // catch-up from height 1
	}
	if err := stream.SendMsg(req); err != nil {
		t.Fatalf("send request failed: %v", err)
	}
	stream.CloseSend()

	// 5. Receive catch-up diffs (heights 1-5)
	received := make([]*pb.StateDiffResponse, 0)
	for i := 0; i < 5; i++ {
		resp := &pb.StateDiffResponse{}
		if err := stream.RecvMsg(resp); err != nil {
			t.Fatalf("recv catch-up diff %d failed: %v", i, err)
		}
		received = append(received, resp)
		t.Logf("received catch-up diff: height=%d entries=%d", resp.Height, len(resp.Entries))
	}

	// Verify catch-up diffs
	for i, diff := range received {
		expectedHeight := uint64(i + 1)
		if diff.Height != expectedHeight {
			t.Errorf("catch-up diff %d: expected height %d, got %d", i, expectedHeight, diff.Height)
		}
		if len(diff.Entries) != 2 {
			t.Errorf("catch-up diff %d: expected 2 entries, got %d", i, len(diff.Entries))
		}
		if diff.Entries[0].Namespace != "Account" {
			t.Errorf("catch-up diff %d: expected namespace Account, got %s", i, diff.Entries[0].Namespace)
		}
	}

	// 6. Publish new diffs while stream is open
	go func() {
		time.Sleep(100 * time.Millisecond) // let subscriber settle
		for h := uint64(6); h <= 10; h++ {
			coord.ReceiveStateDiff(h, []StateDiffEntry{
				{Type: 0, Namespace: "Account", Key: []byte("io1bob"), Value: []byte(fmt.Sprintf("new-%d", h))},
			}, []byte(fmt.Sprintf("digest-%d", h)))
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// 7. Receive live diffs (heights 6-10)
	for i := 0; i < 5; i++ {
		resp := &pb.StateDiffResponse{}
		if err := stream.RecvMsg(resp); err != nil {
			t.Fatalf("recv live diff %d failed: %v", i, err)
		}
		received = append(received, resp)
		t.Logf("received live diff: height=%d entries=%d", resp.Height, len(resp.Entries))
	}

	// Verify we got all 10 diffs
	if len(received) != 10 {
		t.Fatalf("expected 10 total diffs, got %d", len(received))
	}

	// Verify the last live diff
	lastDiff := received[9]
	if lastDiff.Height != 10 {
		t.Errorf("last diff height: expected 10, got %d", lastDiff.Height)
	}
	if string(lastDiff.Entries[0].Key) != "io1bob" {
		t.Errorf("last diff key: expected io1bob, got %s", string(lastDiff.Entries[0].Key))
	}

	// 8. Verify broadcaster stats
	if coord.DiffBroadcaster().BufferedCount() != 10 {
		t.Errorf("expected 10 buffered diffs, got %d", coord.DiffBroadcaster().BufferedCount())
	}
	if coord.DiffBroadcaster().LatestHeight() != 10 {
		t.Errorf("expected latest height 10, got %d", coord.DiffBroadcaster().LatestHeight())
	}

	t.Log("state diff stream E2E test passed!")
}

// TestStateDiffCatchUpOnly tests that an agent starting from height 0
// receives all buffered diffs.
func TestStateDiffCatchUpOnly(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	cfg := Config{
		Enabled:        true,
		MaxAgents:      10,
		DiffBufferSize: 100,
	}

	actPool := &mockActPool{height: 1000}
	stateDB := &mockState{accounts: map[string]*pb.AccountSnapshot{}}

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

	// Publish 20 diffs
	for h := uint64(100); h < 120; h++ {
		coord.ReceiveStateDiff(h, []StateDiffEntry{
			{Type: 0, Namespace: "Account", Key: []byte("addr"), Value: []byte(fmt.Sprintf("%d", h))},
		}, nil)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := lis.Addr().(*net.TCPAddr).Port

	grpcSrv := grpc.NewServer()
	RegisterIOSwarmServer(grpcSrv, &grpcHandler{coord: coord})
	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	streamDesc := &grpc.StreamDesc{StreamName: "StreamStateDiffs", ServerStreams: true}
	stream, err := conn.NewStream(ctx, streamDesc, "/ioswarm.IOSwarm/StreamStateDiffs")
	if err != nil {
		t.Fatal(err)
	}

	// Request from height 105 → should get 15 diffs (105-119)
	req := &pb.StreamStateDiffsRequest{AgentID: "test-catchup", FromHeight: 105}
	if err := stream.SendMsg(req); err != nil {
		t.Fatal(err)
	}
	stream.CloseSend()

	// Read all catch-up diffs
	count := 0
	for {
		resp := &pb.StateDiffResponse{}
		err := stream.RecvMsg(resp)
		if err != nil {
			break // stream blocks on subscription after catch-up, timeout breaks it
		}
		count++
		if count == 15 { // expected catch-up count
			break
		}
	}

	if count < 15 {
		t.Errorf("expected 15 catch-up diffs (105-119), got %d", count)
	}
	t.Logf("catch-up test: received %d diffs", count)
}
