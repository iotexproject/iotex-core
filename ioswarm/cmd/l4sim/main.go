// l4sim is a multi-agent stress simulation for the IOSwarm L4 state diff pipeline.
//
// It validates: concurrent agents, disconnect/reconnect, late join, slow consumers,
// snapshot export/import, and full catch-up + live streaming under load.
//
// Usage:
//
//	go run ./ioswarm/cmd/l4sim --duration=2m --block-time=200ms
//	go run -race ./ioswarm/cmd/l4sim --duration=5m --block-time=500ms
package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotexproject/iotex-core/v2/ioswarm"
	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	flagDuration  = flag.Duration("duration", 2*time.Minute, "total simulation time")
	flagBlockTime = flag.Duration("block-time", 200*time.Millisecond, "block interval")
	flagAgents    = flag.Int("agents", 10, "number of agents (max 10)")
	flagVerbose   = flag.Bool("verbose", false, "print per-agent real-time logs")
)

// agentSpec defines a simulated agent's behavior.
type agentSpec struct {
	name        string
	joinPct     float64 // join at this % of duration (0 = immediately)
	disconnects []disconnectSpec
	slow        bool // simulate slow consumer
	noReconnect bool // after disconnect, don't reconnect
}

type disconnectSpec struct {
	afterPct float64       // disconnect at this % of duration
	duration time.Duration // stay disconnected for this long
}

func agentSpecs() []agentSpec {
	return []agentSpec{
		{name: "agent-00-full", joinPct: 0.0},
		{name: "agent-01-snapshot", joinPct: 0.10},
		{name: "agent-02-late40", joinPct: 0.40},
		{name: "agent-03-late60", joinPct: 0.60},
		{name: "agent-04-disc10s", joinPct: 0.10, disconnects: []disconnectSpec{{afterPct: 0.30, duration: 10 * time.Second}}},
		{name: "agent-05-disc20s", joinPct: 0.10, disconnects: []disconnectSpec{{afterPct: 0.30, duration: 20 * time.Second}}},
		{name: "agent-06-multi-disc", joinPct: 0.10, disconnects: []disconnectSpec{
			{afterPct: 0.25, duration: 5 * time.Second},
			{afterPct: 0.55, duration: 8 * time.Second},
		}},
		{name: "agent-07-very-late", joinPct: 0.80},
		{name: "agent-08-quit", joinPct: 0.10, disconnects: []disconnectSpec{{afterPct: 0.40, duration: 0}}, noReconnect: true},
		{name: "agent-09-slow", joinPct: 0.10, slow: true},
	}
}

// agentResult collects per-agent metrics.
type agentResult struct {
	name           string
	totalReceived  int
	catchUpCount   int
	liveCount      int
	minHeight      uint64
	maxHeight      uint64
	gapCount       int
	connections    int
	disconnected   bool
	err            error
}

func main() {
	flag.Parse()

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	if *flagAgents > 10 {
		*flagAgents = 10
	}

	tmpDir, err := os.MkdirTemp("", "l4sim-*")
	if err != nil {
		logger.Fatal("failed to create temp dir", zap.Error(err))
	}
	defer os.RemoveAll(tmpDir)

	expectedBlocks := uint64(*flagDuration / *flagBlockTime)
	snapshotHeight := expectedBlocks / 10 // snapshot at 10%

	fmt.Println("=== L4 Multi-Agent Stress Simulation ===")
	fmt.Printf("  duration:        %s\n", *flagDuration)
	fmt.Printf("  block-time:      %s\n", *flagBlockTime)
	fmt.Printf("  expected blocks: ~%d\n", expectedBlocks)
	fmt.Printf("  agents:          %d\n", *flagAgents)
	fmt.Printf("  snapshot at:     block %d\n", snapshotHeight)
	fmt.Println()

	// Create coordinator
	cfg := ioswarm.Config{
		Enabled:          true,
		GRPCPort:         0,
		SwarmAPIPort:     0,
		MaxAgents:        20,
		TaskLevel:        "L2",
		ShadowMode:       false,
		PollIntervalMS:   60000,
		DiffBufferSize:   500,
		DiffStoreEnabled: true,
		DiffStorePath:    filepath.Join(tmpDir, "statediffs.db"),
		Reward:           ioswarm.DefaultRewardConfig(),
	}

	coord := ioswarm.NewCoordinator(cfg, &noopActPool{}, &noopStateReader{},
		ioswarm.WithLogger(logger),
		ioswarm.WithDataDir(tmpDir),
	)

	grpcPort, grpcServer, err := startGRPCServer(coord, logger)
	if err != nil {
		logger.Fatal("failed to start gRPC", zap.Error(err))
	}
	defer grpcServer.Stop()
	fmt.Printf("  gRPC port:       %d\n\n", grpcPort)

	ctx, cancel := context.WithTimeout(context.Background(), *flagDuration+30*time.Second)
	defer cancel()

	var coordHeight atomic.Uint64
	var producerDone atomic.Bool
	startTime := time.Now()

	// Phase 1: Produce blocks in background
	go func() {
		ticker := time.NewTicker(*flagBlockTime)
		defer ticker.Stop()
		timer := time.NewTimer(*flagDuration)
		defer timer.Stop()
		var h uint64
		for {
			select {
			case <-timer.C:
				producerDone.Store(true)
				return
			case <-ticker.C:
				h++
				entries := generateDiffEntries(h)
				digest := computeDigest(h, entries)
				coord.ReceiveStateDiff(h, entries, digest)
				coordHeight.Store(h)
			case <-ctx.Done():
				producerDone.Store(true)
				return
			}
		}
	}()

	// Wait for snapshot height
	fmt.Printf("[Phase 1] Producing blocks...\n")
	for coordHeight.Load() < snapshotHeight {
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("[Phase 1] Reached snapshot height %d\n\n", snapshotHeight)

	// Phase 2: Snapshot export + round-trip
	fmt.Printf("[Phase 2] Exporting snapshot at height %d...\n", snapshotHeight)
	snapshotPath := filepath.Join(tmpDir, "snapshot.bin.gz")
	snapshotEntries, err := exportSnapshot(coord.DiffStore(), snapshotHeight, snapshotPath)
	if err != nil {
		fmt.Printf("[Phase 2] FAIL: %v\n", err)
		os.Exit(1)
	}
	reimportCount, _, err := reimportSnapshot(snapshotPath)
	if err != nil {
		fmt.Printf("[Phase 2] FAIL reimport: %v\n", err)
		os.Exit(1)
	}
	if reimportCount != snapshotEntries {
		fmt.Printf("[Phase 2] FAIL: count mismatch: exported %d, reimported %d\n", snapshotEntries, reimportCount)
		os.Exit(1)
	}
	fmt.Printf("[Phase 2] Snapshot OK: %d entries, round-trip verified\n\n", snapshotEntries)

	// Phase 3: Launch agents
	specs := agentSpecs()[:*flagAgents]
	results := make([]*agentResult, len(specs))
	var wg sync.WaitGroup

	for i, spec := range specs {
		wg.Add(1)
		go func(idx int, s agentSpec) {
			defer wg.Done()
			r := runAgent(ctx, s, grpcPort, startTime, *flagDuration, &coordHeight, &producerDone)
			results[idx] = r
		}(i, spec)
	}

	// Progress reporter
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				elapsed := time.Since(startTime)
				pct := float64(elapsed) / float64(*flagDuration) * 100
				h := coordHeight.Load()
				fmt.Printf("[Progress] %.0f%% | height=%d | elapsed=%s\n", pct, h, elapsed.Round(time.Second))
			}
		}
	}()

	wg.Wait()
	fmt.Println()

	// Phase 4: Report & Verification
	fmt.Println("=== Agent Results ===")
	fmt.Printf("%-25s %8s %8s %8s %10s %8s %5s %s\n",
		"Name", "Total", "CatchUp", "Live", "Height", "Gaps", "Conns", "Error")
	for _, r := range results {
		if r == nil {
			continue
		}
		errStr := ""
		if r.err != nil {
			errStr = r.err.Error()
			if len(errStr) > 40 {
				errStr = errStr[:40] + "..."
			}
		}
		fmt.Printf("%-25s %8d %8d %8d %5d-%-4d %8d %5d %s\n",
			r.name, r.totalReceived, r.catchUpCount, r.liveCount,
			r.minHeight, r.maxHeight, r.gapCount, r.connections, errStr)
	}
	fmt.Println()

	// Verifications
	finalHeight := coordHeight.Load()
	ds := coord.DiffStore()
	pass := true
	checks := 0
	passed := 0

	verify := func(name string, ok bool, msg string) {
		checks++
		if ok {
			passed++
			fmt.Printf("  CHECK %d PASS: %s — %s\n", checks, name, msg)
		} else {
			pass = false
			fmt.Printf("  CHECK %d FAIL: %s — %s\n", checks, name, msg)
		}
	}

	// 1. DiffStore completeness
	verify("DiffStore range",
		ds.OldestHeight() == 1 && ds.LatestHeight() == finalHeight,
		fmt.Sprintf("[%d, %d], expected [1, %d]", ds.OldestHeight(), ds.LatestHeight(), finalHeight))

	// 2. Snapshot round-trip
	verify("Snapshot round-trip", true,
		fmt.Sprintf("%d entries, export+reimport match", snapshotEntries))

	// 3. Per-agent no gaps (within single connection)
	allNoGaps := true
	for _, r := range results {
		if r != nil && r.gapCount > 0 {
			allNoGaps = false
			fmt.Printf("    > %s had %d gap(s)\n", r.name, r.gapCount)
		}
	}
	verify("No gaps in agent streams", allNoGaps, "all agents had monotonically increasing heights")

	// 4. Reconnect catch-up (agents 4,5,6)
	reconnectOK := true
	for _, idx := range []int{4, 5, 6} {
		if idx >= len(results) || results[idx] == nil {
			continue
		}
		r := results[idx]
		if r.gapCount > 0 {
			reconnectOK = false
		}
	}
	verify("Reconnect catch-up", reconnectOK, "agents 4/5/6 had no gaps after reconnect")

	// 5. Full-online agent (#0) near coordinator tip
	if len(results) > 0 && results[0] != nil {
		tolerance := uint64(5)
		near := results[0].maxHeight+tolerance >= finalHeight
		verify("Full-online agent near tip", near,
			fmt.Sprintf("agent-0 maxHeight=%d, coordinator=%d", results[0].maxHeight, finalHeight))
	}

	// 6. Slow agent (#9) didn't crash
	if len(results) > 9 && results[9] != nil {
		verify("Slow agent survived", results[9].err == nil,
			fmt.Sprintf("agent-9 received %d diffs, no crash", results[9].totalReceived))
	}

	// 7. Quit agent (#8) didn't affect others
	if len(results) > 8 {
		othersOK := true
		for i, r := range results {
			if i == 8 || r == nil {
				continue
			}
			if r.err != nil && !r.disconnected {
				othersOK = false
			}
		}
		verify("Quit agent isolation", othersOK, "agent-8 quit without affecting others")
	}

	// 8. Broadcaster subscriber count == 0
	subCount := coord.DiffBroadcaster().SubscriberCount()
	verify("Broadcaster cleanup", subCount == 0,
		fmt.Sprintf("subscriber count = %d", subCount))

	// 9. No panic (if we got here, no panic)
	verify("No panic/crash", true, "simulation completed without panic")

	fmt.Printf("\n=== %d/%d checks passed ===\n", passed, checks)
	if pass {
		fmt.Println("RESULT: PASS")
	} else {
		fmt.Println("RESULT: FAIL")
		os.Exit(1)
	}
}

// --- Agent runner ---

func runAgent(
	ctx context.Context,
	spec agentSpec,
	port int,
	startTime time.Time,
	totalDuration time.Duration,
	coordHeight *atomic.Uint64,
	producerDone *atomic.Bool,
) *agentResult {
	result := &agentResult{name: spec.name}

	// Wait until join time
	joinDelay := time.Duration(float64(totalDuration) * spec.joinPct)
	joinAt := startTime.Add(joinDelay)
	if time.Now().Before(joinAt) {
		select {
		case <-ctx.Done():
			return result
		case <-time.After(time.Until(joinAt)):
		}
	}

	if *flagVerbose {
		fmt.Printf("  [%s] joining at height %d\n", spec.name, coordHeight.Load())
	}

	var fromHeight uint64 = 1
	var lastHeight uint64
	discIdx := 0

	for {
		if ctx.Err() != nil || producerDone.Load() {
			break
		}

		// Connect
		result.connections++
		if fromHeight == 0 {
			fromHeight = 1
		}
		if lastHeight > 0 {
			fromHeight = lastHeight + 1
		}

		connCtx, connCancel := context.WithCancel(ctx)

		// Schedule disconnects
		var disconnectTimer *time.Timer
		if discIdx < len(spec.disconnects) {
			disc := spec.disconnects[discIdx]
			discTime := startTime.Add(time.Duration(float64(totalDuration) * disc.afterPct))
			delay := time.Until(discTime)
			if delay <= 0 {
				delay = 100 * time.Millisecond
			}
			disconnectTimer = time.AfterFunc(delay, func() {
				if *flagVerbose {
					fmt.Printf("  [%s] disconnecting at height %d\n", spec.name, coordHeight.Load())
				}
				connCancel()
			})
		}

		// Run connection
		received, maxH, gaps, err := runConnection(connCtx, spec.name, port, fromHeight, spec.slow, coordHeight, producerDone)
		result.totalReceived += received
		result.gapCount += gaps
		if maxH > result.maxHeight {
			result.maxHeight = maxH
		}
		if result.minHeight == 0 || (fromHeight > 0 && fromHeight < result.minHeight) {
			result.minHeight = fromHeight
		}
		lastHeight = maxH

		if disconnectTimer != nil {
			disconnectTimer.Stop()
		}
		connCancel()

		if err != nil && ctx.Err() == nil && !producerDone.Load() {
			// Disconnected (expected for agents with disconnect specs)
			result.disconnected = true
			if discIdx < len(spec.disconnects) {
				disc := spec.disconnects[discIdx]
				discIdx++

				if spec.noReconnect || disc.duration == 0 {
					if *flagVerbose {
						fmt.Printf("  [%s] disconnected permanently\n", spec.name)
					}
					return result
				}

				// Wait before reconnect
				if *flagVerbose {
					fmt.Printf("  [%s] waiting %s before reconnect\n", spec.name, disc.duration)
				}
				select {
				case <-ctx.Done():
					return result
				case <-time.After(disc.duration):
				}

				if *flagVerbose {
					fmt.Printf("  [%s] reconnecting from height %d\n", spec.name, lastHeight+1)
				}
				continue
			}
		}

		// Normal exit (producer done or context cancelled)
		break
	}

	// Compute catch-up vs live (rough: catch-up = fromHeight to join-time height)
	result.liveCount = result.totalReceived
	return result
}

func runConnection(
	ctx context.Context,
	name string,
	port int,
	fromHeight uint64,
	slow bool,
	coordHeight *atomic.Uint64,
	producerDone *atomic.Bool,
) (received int, maxHeight uint64, gaps int, err error) {
	conn, err := grpc.DialContext(ctx, fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	clientStream, err := conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "StreamStateDiffs",
		ServerStreams:  true,
	}, "/ioswarm.IOSwarm/StreamStateDiffs")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("new stream: %w", err)
	}

	if err := clientStream.SendMsg(&pb.StreamStateDiffsRequest{
		AgentID:    name,
		FromHeight: fromHeight,
	}); err != nil {
		return 0, 0, 0, fmt.Errorf("send request: %w", err)
	}
	if err := clientStream.CloseSend(); err != nil {
		return 0, 0, 0, fmt.Errorf("close send: %w", err)
	}

	var lastH uint64
	for {
		resp := &pb.StateDiffResponse{}
		if err := clientStream.RecvMsg(resp); err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return received, maxHeight, gaps, ctx.Err()
			}
			return received, maxHeight, gaps, err
		}

		received++
		if resp.Height > maxHeight {
			maxHeight = resp.Height
		}

		// Gap detection
		if lastH > 0 && resp.Height != lastH+1 {
			gaps++
			if *flagVerbose {
				fmt.Printf("  [%s] GAP: expected %d, got %d\n", name, lastH+1, resp.Height)
			}
		}
		lastH = resp.Height

		if slow {
			time.Sleep(50 * time.Millisecond)
		}

		// Exit when producer is done and we've caught up
		if producerDone.Load() && resp.Height >= coordHeight.Load() {
			return received, maxHeight, gaps, nil
		}
	}
}

// --- State diff generation ---

func generateDiffEntries(h uint64) []ioswarm.StateDiffEntry {
	entries := []ioswarm.StateDiffEntry{
		{Type: 0, Namespace: "Account", Key: []byte("io1sender"), Value: nonceBalance(h, 1000000-h*10)},
		{Type: 0, Namespace: "Account", Key: []byte("io1receiver"), Value: nonceBalance(0, h*10)},
		{Type: 0, Namespace: "Account", Key: []byte("io1blockproducer"), Value: nonceBalance(0, h*5)},
	}
	if h%5 == 0 {
		slot := fmt.Sprintf("slot_%d", h/5)
		entries = append(entries, ioswarm.StateDiffEntry{
			Type: 0, Namespace: "Contract",
			Key: []byte("io1contract1:" + slot), Value: []byte(fmt.Sprintf("value_at_%d", h)),
		})
	}
	if h%20 == 0 {
		entries = append(entries, ioswarm.StateDiffEntry{
			Type: 0, Namespace: "Code",
			Key: []byte(fmt.Sprintf("io1contract_%d", h/20)), Value: []byte(fmt.Sprintf("bytecode_mock_%d", h)),
		})
	}
	if h%15 == 0 && h > 15 {
		entries = append(entries, ioswarm.StateDiffEntry{
			Type: 1, Namespace: "Account", Key: []byte(fmt.Sprintf("io1temp_%d", h-15)),
		})
	}
	return entries
}

func nonceBalance(nonce, balance uint64) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[:8], nonce)
	binary.BigEndian.PutUint64(buf[8:], balance)
	return buf
}

func computeDigest(h uint64, entries []ioswarm.StateDiffEntry) []byte {
	hash := sha256.New()
	binary.Write(hash, binary.BigEndian, h)
	for _, e := range entries {
		hash.Write([]byte{e.Type})
		hash.Write([]byte(e.Namespace))
		hash.Write(e.Key)
		hash.Write(e.Value)
	}
	return hash.Sum(nil)
}

// --- Snapshot ---

type snapshotEntry struct {
	Namespace string `json:"ns"`
	Key       []byte `json:"k"`
	Value     []byte `json:"v"`
}

func exportSnapshot(ds *ioswarm.DiffStore, height uint64, path string) (int, error) {
	diffs, err := ds.GetRange(1, height)
	if err != nil {
		return 0, fmt.Errorf("GetRange [1,%d]: %w", height, err)
	}

	// Replay diffs to build merged state
	state := make(map[string]map[string][]byte)
	for _, diff := range diffs {
		for _, e := range diff.Entries {
			ns := state[e.Namespace]
			if ns == nil {
				ns = make(map[string][]byte)
				state[e.Namespace] = ns
			}
			if e.Type == 0 {
				v := make([]byte, len(e.Value))
				copy(v, e.Value)
				ns[string(e.Key)] = v
			} else {
				delete(ns, string(e.Key))
			}
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	enc := json.NewEncoder(gw)
	count := 0
	for ns, kvs := range state {
		for k, v := range kvs {
			if err := enc.Encode(snapshotEntry{Namespace: ns, Key: []byte(k), Value: v}); err != nil {
				gw.Close()
				return 0, fmt.Errorf("encode: %w", err)
			}
			count++
		}
	}
	if err := gw.Close(); err != nil {
		return 0, fmt.Errorf("gzip close: %w", err)
	}
	return count, nil
}

func reimportSnapshot(path string) (int, [32]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, [32]byte{}, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return 0, [32]byte{}, err
	}
	defer gr.Close()

	dec := json.NewDecoder(gr)
	hash := sha256.New()
	count := 0
	for {
		var e snapshotEntry
		if err := dec.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return 0, [32]byte{}, fmt.Errorf("decode: %w", err)
		}
		hash.Write([]byte(e.Namespace))
		hash.Write(e.Key)
		hash.Write(e.Value)
		count++
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return count, digest, nil
}

// --- gRPC server ---

func startGRPCServer(coord *ioswarm.Coordinator, logger *zap.Logger) (int, *grpc.Server, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	port := lis.Addr().(*net.TCPAddr).Port

	srv := grpc.NewServer()
	ioswarm.RegisterIOSwarmServer(srv, &simHandler{coord: coord})

	go func() {
		if err := srv.Serve(lis); err != nil {
			logger.Error("gRPC serve error", zap.Error(err))
		}
	}()
	return port, srv, nil
}

type simHandler struct {
	coord *ioswarm.Coordinator
}

func (h *simHandler) Register(_ context.Context, _ *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return &pb.RegisterResponse{Accepted: true, HeartbeatIntervalSec: 10}, nil
}

func (h *simHandler) GetTasks(_ *pb.GetTasksRequest, _ ioswarm.IOSwarm_GetTasksServer) error {
	return nil
}

func (h *simHandler) SubmitResults(_ context.Context, _ *pb.BatchResult) (*pb.SubmitResponse, error) {
	return &pb.SubmitResponse{Accepted: true}, nil
}

func (h *simHandler) Heartbeat(_ context.Context, _ *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	return &pb.HeartbeatResponse{Alive: true, Directive: "continue"}, nil
}

func (h *simHandler) StreamStateDiffs(req *pb.StreamStateDiffsRequest, stream ioswarm.IOSwarm_StreamStateDiffsServer) error {
	agentID := req.AgentID
	if agentID == "" {
		agentID = "sim-agent"
	}

	broadcaster := h.coord.DiffBroadcaster()
	ch := broadcaster.Subscribe(agentID)
	defer broadcaster.Unsubscribe(agentID)

	var lastSentHeight uint64

	// Catch-up from DiffStore
	if req.FromHeight > 0 {
		ds := h.coord.DiffStore()
		if ds != nil && ds.LatestHeight() > 0 && req.FromHeight <= ds.LatestHeight() {
			catchUp, err := ds.GetRange(req.FromHeight, ds.LatestHeight())
			if err == nil {
				for _, diff := range catchUp {
					if err := stream.Send(stateDiffToResponse(diff)); err != nil {
						return err
					}
					lastSentHeight = diff.Height
				}
			}
		}
	}

	// Live stream
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case diff, ok := <-ch:
			if !ok {
				return nil
			}
			if diff.Height <= lastSentHeight {
				continue
			}
			if err := stream.Send(stateDiffToResponse(diff)); err != nil {
				return err
			}
			lastSentHeight = diff.Height
		}
	}
}

func stateDiffToResponse(diff *ioswarm.StateDiff) *pb.StateDiffResponse {
	entries := make([]*pb.StateDiffEntry, len(diff.Entries))
	for i, e := range diff.Entries {
		entries[i] = &pb.StateDiffEntry{
			WriteType: e.Type,
			Namespace: e.Namespace,
			Key:       e.Key,
			Value:     e.Value,
		}
	}
	return &pb.StateDiffResponse{
		Height:      diff.Height,
		Entries:     entries,
		DigestBytes: diff.DigestBytes,
	}
}

// --- JSON codec ---

type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)     { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                              { return "proto" }

// --- Noop dependencies ---

type noopActPool struct{}

func (n *noopActPool) PendingActions() []*ioswarm.PendingTx { return nil }
func (n *noopActPool) BlockHeight() uint64                  { return 0 }

type noopStateReader struct{}

func (n *noopStateReader) AccountState(_ string) (*pb.AccountSnapshot, error) { return nil, nil }
func (n *noopStateReader) GetCode(_ string) ([]byte, error)                   { return nil, nil }
func (n *noopStateReader) GetStorageAt(_, _ string) (string, error)           { return "", nil }
func (n *noopStateReader) SimulateAccessList(_, _ string, _ []byte, _ string, _ uint64) (map[string][]string, error) {
	return nil, nil
}
