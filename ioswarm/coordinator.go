package ioswarm

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/iotexproject/iotex-core/ioswarm/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// Coordinator is the central orchestrator that polls the actpool,
// prefetches account state, and dispatches validation tasks to agents.
type Coordinator struct {
	cfg        Config
	actPool    ActPoolReader
	prefetcher *Prefetcher
	registry   *Registry
	scheduler  *Scheduler
	shadow     *ShadowComparator
	reward     *RewardDistributor
	logger     *zap.Logger
	grpcServer *grpc.Server
	taskIDSeq  atomic.Uint32

	// Pending payouts: agent_id → PayoutInfo (consumed by next heartbeat)
	pendingPayouts sync.Map

	// Block tracking
	lastBlockHeight atomic.Uint64

	// Metrics
	totalDispatched atomic.Uint64
	totalReceived   atomic.Uint64

	// Recent results buffer for EVM shadow comparison
	recentResults   sync.Map // task_id → *pb.TaskResult
}

// NewCoordinator creates a new IOSwarm coordinator.
// actPool and stateReader are in-memory interfaces from iotex-core.
func NewCoordinator(cfg Config, actPool ActPoolReader, stateReader StateReader, opts ...Option) *Coordinator {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	logger := o.logger

	registry := NewRegistry(cfg.MaxAgents)
	return &Coordinator{
		cfg:        cfg,
		actPool:    actPool,
		prefetcher: NewPrefetcher(stateReader, logger),
		registry:   registry,
		scheduler:  NewScheduler(registry, logger),
		shadow:     NewShadowComparator(logger),
		reward:     NewRewardDistributor(cfg.Reward, cfg.DelegateAddress, logger),
		logger:     logger,
	}
}

// Option configures the coordinator.
type Option func(*options)

type options struct {
	logger *zap.Logger
}

func defaultOptions() options {
	logger, _ := zap.NewProduction()
	if logger == nil {
		logger = zap.NewNop()
	}
	return options{logger: logger}
}

// WithLogger sets the coordinator's logger.
func WithLogger(l *zap.Logger) Option {
	return func(o *options) { o.logger = l }
}

// Start launches the coordinator's gRPC server and main polling loop.
func (c *Coordinator) Start(ctx context.Context) error {
	c.logger.Info("starting IOSwarm coordinator",
		zap.Int("port", c.cfg.GRPCPort),
		zap.Bool("shadow_mode", c.cfg.ShadowMode),
		zap.String("task_level", c.cfg.TaskLevel))

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", c.cfg.GRPCPort, err)
	}

	grpcOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              30 * time.Second,
			Timeout:           10 * time.Second,
		}),
		grpc.MaxConcurrentStreams(100),
	}
	if c.cfg.MasterSecret != "" {
		grpcOpts = append(grpcOpts,
			grpc.UnaryInterceptor(hmacUnaryInterceptor(c.cfg.MasterSecret)),
			grpc.StreamInterceptor(hmacStreamInterceptor(c.cfg.MasterSecret)),
		)
		c.logger.Info("gRPC HMAC auth enabled")
	}
	c.grpcServer = grpc.NewServer(grpcOpts...)
	RegisterIOSwarmServer(c.grpcServer, &grpcHandler{coord: c})

	go func() {
		c.logger.Info("gRPC server listening", zap.Int("port", c.cfg.GRPCPort))
		if err := c.grpcServer.Serve(lis); err != nil {
			c.logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	// Start main polling loop
	go c.runLoop(ctx)

	// Start agent eviction loop
	go c.evictionLoop(ctx)

	// Start epoch reward loop
	go c.epochLoop(ctx)

	// Start SwarmAPI HTTP server
	if c.cfg.SwarmAPIPort > 0 {
		api := NewSwarmAPI(c, c.reward)
		var handler http.Handler = api.Handler()
		if c.cfg.MasterSecret != "" {
			handler = tokenHTTPMiddleware(c.cfg.MasterSecret, handler)
		}
		go func() {
			addr := fmt.Sprintf(":%d", c.cfg.SwarmAPIPort)
			c.logger.Info("swarm API listening", zap.Int("port", c.cfg.SwarmAPIPort))
			if err := http.ListenAndServe(addr, handler); err != nil {
				c.logger.Error("swarm API error", zap.Error(err))
			}
		}()
	}

	return nil
}

// Stop gracefully shuts down the coordinator.
func (c *Coordinator) Stop() {
	c.logger.Info("stopping IOSwarm coordinator")
	if c.grpcServer != nil {
		c.grpcServer.GracefulStop()
	}
}

// runLoop is the main coordinator loop: poll actpool → prefetch → dispatch.
func (c *Coordinator) runLoop(ctx context.Context) {
	interval := time.Duration(c.cfg.PollIntervalMS) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.pollAndDispatch()
		}
	}
}

// pollAndDispatch is the core logic: read actpool → prefetch state → build tasks → dispatch.
func (c *Coordinator) pollAndDispatch() {
	// 1. Get pending txs from actpool
	pending := c.actPool.PendingActions()
	if len(pending) == 0 {
		return
	}

	// Check if block height advanced — invalidate prefetcher cache
	blockHeight := c.actPool.BlockHeight()
	if prev := c.lastBlockHeight.Swap(blockHeight); prev != blockHeight && prev != 0 {
		c.prefetcher.InvalidateCache()
	}

	// 2. Prefetch account state for all involved addresses
	stateMap := c.prefetcher.Prefetch(pending)

	// 3. Build task packages
	level := parseTaskLevel(c.cfg.TaskLevel)
	tasks := make([]*pb.TaskPackage, 0, len(pending))

	for _, tx := range pending {
		taskID := c.taskIDSeq.Add(1)
		task := &pb.TaskPackage{
			TaskID:      taskID,
			TxRaw:       tx.RawBytes,
			Level:       level,
			BlockHeight: blockHeight,
		}

		if snap, ok := stateMap[tx.From]; ok {
			task.Sender = snap
		}
		if snap, ok := stateMap[tx.To]; ok {
			task.Receiver = snap
		}

		// L3: attach EVM execution data
		if level == pb.TaskLevel_L3_FULL_EXECUTE {
			c.enrichL3Task(task, tx, blockHeight)
		}

		tasks = append(tasks, task)
	}

	// 4. Dispatch to agents
	dispatched := c.scheduler.Dispatch(tasks, 10)
	c.totalDispatched.Add(uint64(dispatched))

	if dispatched > 0 {
		c.logger.Info("dispatched tasks",
			zap.Int("pending", len(pending)),
			zap.Int("dispatched", dispatched),
			zap.Int("agents", c.registry.Count()))
	}
}

// evictionLoop periodically removes stale agents.
func (c *Coordinator) evictionLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			evicted := c.registry.EvictStale(60 * time.Second)
			for _, id := range evicted {
				c.logger.Info("evicted stale agent", zap.String("agent", id))
			}
		}
	}
}

// handleResults processes results from an agent.
func (c *Coordinator) handleResults(result *pb.BatchResult) {
	c.totalReceived.Add(uint64(len(result.Results)))

	if c.cfg.ShadowMode {
		c.shadow.RecordAgentResults(result.AgentID, result)
	}

	// Store results for EVM shadow comparison
	for _, r := range result.Results {
		c.recentResults.Store(r.TaskID, r)
	}

	// Feed reward system: count tasks and valid results
	var totalLatencyUs uint64
	correct := uint64(0)
	for _, r := range result.Results {
		totalLatencyUs += r.LatencyUs
		if r.Valid {
			correct++
		}
	}
	c.reward.RecordWork(result.AgentID, uint64(len(result.Results)), correct, totalLatencyUs)

	c.logger.Info("received results",
		zap.String("agent", result.AgentID),
		zap.String("batch", result.BatchID),
		zap.Int("count", len(result.Results)))
}

// ShadowStats returns the current shadow comparison statistics.
func (c *Coordinator) ShadowStats() ShadowStats {
	return c.shadow.Stats()
}

// OnBlockExecuted should be called after iotex-core executes a block.
// It compares agent results against actual execution and invalidates
// the prefetcher cache so stale state isn't served to agents.
//
// actualResults maps task_id → whether the tx was actually valid.
// In production, this is wired from iotex-core's block executor.
// In simulation, pass generated results.
func (c *Coordinator) OnBlockExecuted(blockHeight uint64, actualResults map[uint32]bool) {
	// 1. Compare agent results against actual execution
	if c.cfg.ShadowMode && len(actualResults) > 0 {
		mismatches := c.shadow.CompareWithActual(actualResults, blockHeight)
		if len(mismatches) > 0 {
			c.logger.Warn("shadow mismatches detected",
				zap.Int("count", len(mismatches)),
				zap.Uint64("block", blockHeight))
		}
	}

	// 2. Invalidate prefetcher cache — state has changed
	c.prefetcher.InvalidateCache()
}

// LastTaskID returns the most recently assigned task ID.
func (c *Coordinator) LastTaskID() uint32 {
	return c.taskIDSeq.Load()
}

// DrainRecentResults removes and returns all recent task results (for EVM shadow comparison).
func (c *Coordinator) DrainRecentResults() []*pb.TaskResult {
	var results []*pb.TaskResult
	c.recentResults.Range(func(key, value any) bool {
		results = append(results, value.(*pb.TaskResult))
		c.recentResults.Delete(key)
		return true
	})
	return results
}

// CompareEVMShadow feeds EVM reference results to the shadow comparator
// for comparison against agent-submitted EVM results.
func (c *Coordinator) CompareEVMShadow(agentResults []*pb.TaskResult, actualResults map[uint32]*EVMActualResult, blockHeight uint64) {
	c.shadow.CompareEVMResults(agentResults, actualResults, blockHeight)
}

// NewGRPCHandler creates a gRPC handler for this coordinator.
// Use with RegisterIOSwarmServer to register on a grpc.Server.
func NewGRPCHandler(c *Coordinator) IOSwarmServer {
	return &grpcHandler{coord: c}
}

// epochLoop triggers reward distribution at epoch boundaries.
// An epoch = EpochBlocks blocks × ~10s/block ≈ 1 hour.
// For simplicity, we use a time-based timer: EpochBlocks * 10s.
func (c *Coordinator) epochLoop(ctx context.Context) {
	epochDuration := time.Duration(c.cfg.Reward.EpochBlocks) * 10 * time.Second
	if epochDuration < 30*time.Second {
		epochDuration = 30 * time.Second // safety floor
	}

	ticker := time.NewTicker(epochDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.distributeEpochReward()
		}
	}
}

// distributeEpochReward calculates and queues payouts for all agents.
// The actual IOTX transfer is delegated to an external mechanism (on-chain tx or contract).
// Here we compute the split and notify agents via heartbeat.
func (c *Coordinator) distributeEpochReward() {
	work := c.reward.CurrentWork()
	if len(work) == 0 {
		return
	}

	// TODO: In production, fetch actual delegate epoch reward from chain.
	// For now, use a placeholder: 800 IOTX per epoch (realistic for a top delegate).
	epochReward := new(big.Int).Mul(big.NewInt(800), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

	summary := c.reward.Distribute(epochReward)

	// Queue payout notifications for each agent's next heartbeat
	for i, p := range summary.Payouts {
		c.pendingPayouts.Store(p.AgentID, &pb.PayoutInfo{
			Epoch:       summary.Epoch,
			AmountIOTX:  p.AmountIOTX,
			Rank:        i + 1,
			TotalAgents: summary.AgentCount,
		})
	}

	c.logger.Info("epoch reward distributed",
		zap.Uint64("epoch", summary.Epoch),
		zap.Int("agents", summary.AgentCount),
		zap.Uint64("total_tasks", summary.TotalTasks))
}

// consumePayout removes and returns a pending payout for the agent, if any.
func (c *Coordinator) consumePayout(agentID string) *pb.PayoutInfo {
	val, ok := c.pendingPayouts.LoadAndDelete(agentID)
	if !ok {
		return nil
	}
	return val.(*pb.PayoutInfo)
}

// enrichL3Task attaches EVM execution data to an L3 task.
// It fetches contract bytecode, storage, and builds the BlockContext + EvmTx.
func (c *Coordinator) enrichL3Task(task *pb.TaskPackage, tx *PendingTx, blockHeight uint64) {
	// Build EvmTx from PendingTx fields
	task.EvmTx = &pb.EvmTx{
		To:       tx.To,
		Value:    tx.Amount,
		Data:     tx.Data,
		GasLimit: tx.GasLimit,
		GasPrice: tx.GasPrice,
	}

	// Build BlockContext
	task.BlockContext = &pb.BlockCtx{
		Number:    blockHeight,
		Timestamp: uint64(time.Now().Unix()),
		GasLimit:  30_000_000,
		BaseFee:   "0",
		Coinbase:  c.cfg.DelegateAddress,
	}

	// Fetch contract bytecode and storage for receiver (if it has CodeHash)
	if task.Receiver != nil && len(task.Receiver.CodeHash) > 0 && tx.To != "" {
		codes := c.prefetcher.PrefetchCode([]string{tx.To})
		if len(codes) > 0 {
			task.ContractCode = codes
		}
		// Prefetch storage slots touched by this contract.
		// In production, use static analysis or access lists to determine slots.
		// For now, prefetch slot 0 (common for simple contracts).
		storage := c.prefetcher.PrefetchStorage(tx.To, []string{
			"0x0000000000000000000000000000000000000000000000000000000000000000",
		})
		if len(storage) > 0 {
			task.StorageSlots = map[string]map[string]string{
				tx.To: storage,
			}
		}
	}
}

func parseTaskLevel(s string) pb.TaskLevel {
	switch s {
	case "L1":
		return pb.TaskLevel_L1_SIG_VERIFY
	case "L3":
		return pb.TaskLevel_L3_FULL_EXECUTE
	default:
		return pb.TaskLevel_L2_STATE_VERIFY
	}
}
