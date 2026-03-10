// sim runs a full IOSwarm simulation: 1 delegate coordinator + N agents.
//
// Simulates realistic IoTeX mainnet workload:
//   - Block time: ~5s
//   - Tx per block: 50-200 (configurable)
//   - Mix: 70% transfers, 20% contract calls, 10% staking
//   - Accounts: 1000 unique addresses
//
// Usage:
//   go run ./cmd/sim --agents=10 --duration=30s --tps=50
package main

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/iotexproject/iotex-core/ioswarm"
	pb "github.com/iotexproject/iotex-core/ioswarm/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// --- Simulation config ---

type simConfig struct {
	numAgents    int
	duration     time.Duration
	tps          int // target transactions per second
	blockTime    time.Duration
	numAccounts  int
	regions      []string
}

// --- Simulated actpool ---

type simActPool struct {
	mu      sync.Mutex
	txs     []*ioswarm.PendingTx
	height  atomic.Uint64
	accounts []string
}

func newSimActPool(numAccounts int) *simActPool {
	accounts := make([]string, numAccounts)
	for i := range accounts {
		accounts[i] = fmt.Sprintf("io1addr%04d", i)
	}
	pool := &simActPool{accounts: accounts}
	pool.height.Store(20_000_000) // simulate mainnet height
	return pool
}

func (p *simActPool) PendingActions() []*ioswarm.PendingTx {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]*ioswarm.PendingTx, len(p.txs))
	copy(cp, p.txs)
	return cp
}

func (p *simActPool) BlockHeight() uint64 {
	return p.height.Load()
}

// generateBlock creates a batch of transactions simulating one block.
func (p *simActPool) generateBlock(txCount int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.txs = make([]*ioswarm.PendingTx, 0, txCount)
	h := p.height.Add(1)

	for i := 0; i < txCount; i++ {
		fromIdx := int(h*uint64(i+1)) % len(p.accounts)
		toIdx := int(h*uint64(i+7)) % len(p.accounts)
		if toIdx == fromIdx {
			toIdx = (toIdx + 1) % len(p.accounts)
		}

		// Mix tx types by size: transfer=145B, contract=300-800B, staking=200B
		var dataSize int
		r := i % 10
		switch {
		case r < 7: // 70% transfers
			dataSize = 0
		case r < 9: // 20% contract calls
			dataSize = 300 + (i % 500)
		default: // 10% staking
			dataSize = 150
		}

		raw := buildSimTxRaw(80 + dataSize) // payload + sig

		p.txs = append(p.txs, &ioswarm.PendingTx{
			Hash:     fmt.Sprintf("0x%x%04d", h, i),
			From:     p.accounts[fromIdx],
			To:       p.accounts[toIdx],
			Nonce:    h*1000 + uint64(i),
			Amount:   "1000000000000000000", // 1 IOTX
			GasLimit: 21000,
			GasPrice: "1000000000000",
			RawBytes: raw,
			Data:     make([]byte, dataSize),
		})
	}
}

// --- Simulated state DB ---

type simStateDB struct {
	numAccounts int
}

func (s *simStateDB) AccountState(address string) (*pb.AccountSnapshot, error) {
	// Simulate ~100μs state read latency
	time.Sleep(50 * time.Microsecond)
	return &pb.AccountSnapshot{
		Address: address,
		Balance: "100000000000000000000000", // 100k IOTX
		Nonce:   0,
	}, nil
}

func (s *simStateDB) GetCode(address string) ([]byte, error) {
	return nil, nil
}

func (s *simStateDB) GetStorageAt(address, slot string) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

// --- Agent stats ---

type agentStats struct {
	id             string
	region         string
	level          string
	batchesRecv    atomic.Uint64
	tasksProcessed atomic.Uint64
	totalLatencyUs atomic.Uint64 // sum of per-task latency
	errors         atomic.Uint64
}

func (a *agentStats) avgLatencyUs() float64 {
	t := a.tasksProcessed.Load()
	if t == 0 {
		return 0
	}
	return float64(a.totalLatencyUs.Load()) / float64(t)
}

// --- Simulated agent ---

func runAgent(ctx context.Context, port int, stats *agentStats, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		stats.errors.Add(1)
		return
	}
	defer conn.Close()

	// Register
	regResp := &pb.RegisterResponse{}
	err = conn.Invoke(ctx, "/ioswarm.IOSwarm/Register", &pb.RegisterRequest{
		AgentID:    stats.id,
		Capability: parseLevel(stats.level),
		Region:     stats.region,
		Version:    "sim-1.0",
	}, regResp)
	if err != nil || !regResp.Accepted {
		stats.errors.Add(1)
		return
	}

	// Open task stream
	streamDesc := &grpc.StreamDesc{StreamName: "GetTasks", ServerStreams: true}
	stream, err := conn.NewStream(ctx, streamDesc, "/ioswarm.IOSwarm/GetTasks")
	if err != nil {
		stats.errors.Add(1)
		return
	}
	if err := stream.SendMsg(&pb.GetTasksRequest{
		AgentID:      stats.id,
		MaxLevel:     parseLevel(stats.level),
		MaxBatchSize: 50,
	}); err != nil {
		stats.errors.Add(1)
		return
	}
	stream.CloseSend()

	// Heartbeat goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				conn.Invoke(ctx, "/ioswarm.IOSwarm/Heartbeat", &pb.HeartbeatRequest{
					AgentID:        stats.id,
					TasksProcessed: uint32(stats.tasksProcessed.Load()),
				}, &pb.HeartbeatResponse{})
			}
		}
	}()

	// Process loop
	for {
		batch := &pb.TaskBatch{}
		if err := stream.RecvMsg(batch); err != nil {
			return // stream closed
		}

		stats.batchesRecv.Add(1)

		// Process tasks
		results := make([]*pb.TaskResult, 0, len(batch.Tasks))
		for _, task := range batch.Tasks {
			start := time.Now()
			valid, reason := validateTask(task, stats.level)
			elapsed := time.Since(start)

			results = append(results, &pb.TaskResult{
				TaskID:       task.TaskID,
				Valid:        valid,
				RejectReason: reason,
				GasEstimate:  21000,
				LatencyUs:    uint64(elapsed.Microseconds()),
			})

			stats.tasksProcessed.Add(1)
			stats.totalLatencyUs.Add(uint64(elapsed.Microseconds()))
		}

		// Submit results
		conn.Invoke(ctx, "/ioswarm.IOSwarm/SubmitResults", &pb.BatchResult{
			AgentID:   stats.id,
			BatchID:   batch.BatchID,
			Results:   results,
			Timestamp: uint64(time.Now().UnixMilli()),
		}, &pb.SubmitResponse{})
	}
}

func validateTask(task *pb.TaskPackage, level string) (bool, string) {
	// L1: sig verify
	raw := task.TxRaw
	if len(raw) < 65 {
		return false, "tx_too_short"
	}
	sigStart := len(raw) - 65
	r := new(big.Int).SetBytes(raw[sigStart : sigStart+32])
	s := new(big.Int).SetBytes(raw[sigStart+32 : sigStart+64])
	curve := elliptic.P256()
	if r.Sign() == 0 || s.Sign() == 0 || r.Cmp(curve.Params().N) >= 0 || s.Cmp(curve.Params().N) >= 0 {
		return false, "invalid_sig"
	}

	// L2: state check
	if level == "L2" && task.Sender != nil {
		bal, ok := new(big.Int).SetString(task.Sender.Balance, 10)
		if !ok || bal.Sign() < 0 {
			return false, "invalid_balance"
		}
	}

	return true, ""
}

// --- Helpers ---

func buildSimTxRaw(payloadSize int) []byte {
	payload := make([]byte, payloadSize)
	rand.Read(payload)
	curve := elliptic.P256()
	r, _ := rand.Int(rand.Reader, curve.Params().N)
	s, _ := rand.Int(rand.Reader, curve.Params().N)
	raw := append(payload, r.FillBytes(make([]byte, 32))...)
	raw = append(raw, s.FillBytes(make([]byte, 32))...)
	raw = append(raw, byte(0))
	return raw
}

func parseLevel(s string) pb.TaskLevel {
	switch s {
	case "L1":
		return pb.TaskLevel_L1_SIG_VERIFY
	case "L3":
		return pb.TaskLevel_L3_FULL_EXECUTE
	default:
		return pb.TaskLevel_L2_STATE_VERIFY
	}
}

// --- Main ---

func main() {
	numAgents := flag.Int("agents", 10, "number of swarm agents")
	duration := flag.Duration("duration", 30*time.Second, "simulation duration")
	tps := flag.Int("tps", 50, "target transactions per second")
	blockTime := flag.Duration("block-time", 5*time.Second, "simulated block interval")
	numAccounts := flag.Int("accounts", 1000, "number of unique addresses")
	flag.Parse()

	cfg := simConfig{
		numAgents:   *numAgents,
		duration:    *duration,
		tps:         *tps,
		blockTime:   *blockTime,
		numAccounts: *numAccounts,
		regions:     []string{"us-west", "us-east", "eu-west", "eu-central", "ap-east", "ap-south"},
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              IOSwarm Delegate Simulation                    ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Agents:     %-3d                                            ║\n", cfg.numAgents)
	fmt.Printf("║  Duration:   %-10s                                      ║\n", cfg.duration)
	fmt.Printf("║  Target TPS: %-3d                                            ║\n", cfg.tps)
	fmt.Printf("║  Block time: %-10s                                      ║\n", cfg.blockTime)
	fmt.Printf("║  Accounts:   %-5d                                          ║\n", cfg.numAccounts)
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Quiet logger for coordinator (only errors)
	logCfg := zap.NewProductionConfig()
	logCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	logger, _ := logCfg.Build()

	// Setup coordinator
	txPerBlock := cfg.tps * int(cfg.blockTime.Seconds())
	actPool := newSimActPool(cfg.numAccounts)
	stateDB := &simStateDB{numAccounts: cfg.numAccounts}

	coordCfg := ioswarm.Config{
		Enabled:        true,
		MaxAgents:      cfg.numAgents + 10,
		TaskLevel:      "L2",
		ShadowMode:     true,
		PollIntervalMS: int(cfg.blockTime.Milliseconds()),
	}

	coord := ioswarm.NewCoordinator(coordCfg, actPool, stateDB, ioswarm.WithLogger(logger))

	// Start gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen failed: %v\n", err)
		os.Exit(1)
	}
	port := lis.Addr().(*net.TCPAddr).Port

	grpcSrv := grpc.NewServer()
	ioswarm.RegisterIOSwarmServer(grpcSrv, ioswarm.NewGRPCHandler(coord))
	go grpcSrv.Serve(lis)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration+10*time.Second)
	defer cancel()

	// Trap signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := coord.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "coordinator start failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Coordinator listening on :%d\n", port)

	// Spawn agents
	agentList := make([]*agentStats, cfg.numAgents)
	var agentWg sync.WaitGroup

	// Mix: 60% L2, 30% L1, 10% L2 (different regions)
	for i := 0; i < cfg.numAgents; i++ {
		level := "L2"
		if i%10 >= 7 {
			level = "L1"
		}
		region := cfg.regions[i%len(cfg.regions)]

		stats := &agentStats{
			id:     fmt.Sprintf("ant-%s-%02d", region, i),
			region: region,
			level:  level,
		}
		agentList[i] = stats

		agentWg.Add(1)
		go runAgent(ctx, port, stats, &agentWg)
	}

	// Wait for agents to register
	time.Sleep(500 * time.Millisecond)

	fmt.Printf("  %d agents registered\n", cfg.numAgents)
	fmt.Println()
	fmt.Println("  Simulating...")
	fmt.Println()

	// Generate blocks
	simStart := time.Now()
	ticker := time.NewTicker(cfg.blockTime)
	defer ticker.Stop()

	blockCount := 0
	totalTxGenerated := 0
	progressTicker := time.NewTicker(5 * time.Second)
	defer progressTicker.Stop()

	simCtx, simCancel := context.WithTimeout(ctx, cfg.duration)
	defer simCancel()

	for {
		select {
		case <-simCtx.Done():
			goto done
		case <-ticker.C:
			blockCount++
			// Vary tx count: ±30% around target
			variance := txPerBlock / 3
			count := txPerBlock - variance + (blockCount % (2*variance + 1))
			if count < 10 {
				count = 10
			}
			actPool.generateBlock(count)
			totalTxGenerated += count

		case <-progressTicker.C:
			elapsed := time.Since(simStart).Seconds()
			var totalProcessed uint64
			for _, a := range agentList {
				totalProcessed += a.tasksProcessed.Load()
			}
			fmt.Printf("  [%5.0fs] blocks=%d  generated=%d  processed=%d  effective_tps=%.0f\n",
				elapsed, blockCount, totalTxGenerated, totalProcessed,
				float64(totalProcessed)/elapsed)
		}
	}

done:
	// Allow final batches to drain
	time.Sleep(1 * time.Second)
	cancel()

	fmt.Println()
	fmt.Println("  Shutting down agents...")
	grpcSrv.Stop()

	// Wait for agents (with timeout)
	doneCh := make(chan struct{})
	go func() {
		agentWg.Wait()
		close(doneCh)
	}()
	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
	}

	coord.Stop()
	elapsed := time.Since(simStart)

	// --- Report ---
	fmt.Println()
	printReport(cfg, elapsed, blockCount, totalTxGenerated, agentList, coord)
}

func printReport(cfg simConfig, elapsed time.Duration, blockCount, totalTxGenerated int, agents []*agentStats, coord *ioswarm.Coordinator) {
	var totalProcessed, totalBatches, totalErrors uint64
	var totalLatencyUs uint64
	for _, a := range agents {
		totalProcessed += a.tasksProcessed.Load()
		totalBatches += a.batchesRecv.Load()
		totalErrors += a.errors.Load()
		totalLatencyUs += a.totalLatencyUs.Load()
	}

	avgLatency := float64(0)
	if totalProcessed > 0 {
		avgLatency = float64(totalLatencyUs) / float64(totalProcessed)
	}

	effectiveTPS := float64(totalProcessed) / elapsed.Seconds()

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    SIMULATION REPORT                        ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  OVERVIEW                                                   ║")
	fmt.Printf("║    Duration:           %-10s                             ║\n", elapsed.Round(time.Millisecond))
	fmt.Printf("║    Blocks produced:    %-5d                                 ║\n", blockCount)
	fmt.Printf("║    Txs generated:      %-6d                                ║\n", totalTxGenerated)
	fmt.Printf("║    Txs validated:      %-6d                                ║\n", totalProcessed)
	fmt.Printf("║    Validation rate:    %-6.1f%%                               ║\n", float64(totalProcessed)/float64(totalTxGenerated)*100)
	fmt.Printf("║    Effective TPS:      %-6.1f                                ║\n", effectiveTPS)
	fmt.Printf("║    Avg latency/tx:     %-6.0fμs                              ║\n", avgLatency)
	fmt.Printf("║    Total batches:      %-5d                                 ║\n", totalBatches)
	fmt.Printf("║    Agent errors:       %-3d                                   ║\n", totalErrors)
	fmt.Println("║                                                             ║")
	fmt.Println("║  AGENT BREAKDOWN                                            ║")
	fmt.Println("║  ┌────────────────────────┬────────┬────────┬──────────────┐ ║")
	fmt.Println("║  │ Agent ID               │ Tasks  │ Batch  │ Avg Lat (μs) │ ║")
	fmt.Println("║  ├────────────────────────┼────────┼────────┼──────────────┤ ║")

	// Sort agents by tasks processed (desc)
	sorted := make([]*agentStats, len(agents))
	copy(sorted, agents)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].tasksProcessed.Load() > sorted[j].tasksProcessed.Load()
	})

	for _, a := range sorted {
		tasks := a.tasksProcessed.Load()
		batches := a.batchesRecv.Load()
		lat := a.avgLatencyUs()
		name := a.id
		if len(name) > 22 {
			name = name[:22]
		}
		fmt.Printf("║  │ %-22s │ %6d │ %6d │ %12.0f │ ║\n", name, tasks, batches, lat)
	}

	fmt.Println("║  └────────────────────────┴────────┴────────┴──────────────┘ ║")

	// Region summary
	regionTasks := make(map[string]uint64)
	regionCount := make(map[string]int)
	for _, a := range agents {
		regionTasks[a.region] += a.tasksProcessed.Load()
		regionCount[a.region]++
	}

	fmt.Println("║                                                             ║")
	fmt.Println("║  REGION DISTRIBUTION                                        ║")
	fmt.Println("║  ┌──────────────┬────────┬────────────────────────────┐      ║")
	fmt.Println("║  │ Region       │ Agents │ Tasks                      │      ║")
	fmt.Println("║  ├──────────────┼────────┼────────────────────────────┤      ║")

	regions := make([]string, 0, len(regionTasks))
	for r := range regionTasks {
		regions = append(regions, r)
	}
	sort.Strings(regions)

	for _, r := range regions {
		tasks := regionTasks[r]
		bar := strings.Repeat("█", int(float64(tasks)/float64(totalProcessed)*25))
		fmt.Printf("║  │ %-12s │ %6d │ %-5d %-21s │      ║\n", r, regionCount[r], tasks, bar)
	}

	fmt.Println("║  └──────────────┴────────┴────────────────────────────┘      ║")

	// Shadow stats
	shadowStats := coord.ShadowStats()
	fmt.Println("║                                                             ║")
	fmt.Println("║  SHADOW MODE                                                ║")
	fmt.Printf("║    Compared:      %-6d                                     ║\n", shadowStats.TotalCompared)
	fmt.Printf("║    Matched:       %-6d                                     ║\n", shadowStats.TotalMatched)
	fmt.Printf("║    Mismatched:    %-6d                                     ║\n", shadowStats.TotalMismatched)
	if shadowStats.TotalCompared > 0 {
		fmt.Printf("║    Accuracy:      %-6.2f%%                                    ║\n",
			float64(shadowStats.TotalMatched)/float64(shadowStats.TotalCompared)*100)
	}

	fmt.Println("║                                                             ║")

	// Cost analysis
	costPerAgent := 5.0 // $5/mo
	totalCost := costPerAgent * float64(len(agents))
	costPerMTx := totalCost / (effectiveTPS * 86400 * 30 / 1_000_000) // per million tx/month
	fmt.Println("║  COST ANALYSIS (projected)                                  ║")
	fmt.Printf("║    Agents:        %-3d × $5/mo = $%-5.0f/mo                    ║\n", len(agents), totalCost)
	fmt.Printf("║    Throughput:    %-6.0f tx/s                                 ║\n", effectiveTPS)
	fmt.Printf("║    Monthly vol:   %-5.1fM tx                                  ║\n", effectiveTPS*86400*30/1_000_000)
	fmt.Printf("║    Cost/M tx:     $%-6.2f                                    ║\n", costPerMTx)

	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
}
