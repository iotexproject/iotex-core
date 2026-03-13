package ioswarm

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
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
	settler    RewardSettler // on-chain settlement (nil = disabled)
	logger     *zap.Logger
	grpcServer *grpc.Server
	httpServer *http.Server
	taskIDSeq  atomic.Uint32

	// Pending payouts: agent_id → PayoutInfo (consumed by next heartbeat)
	pendingPayouts sync.Map

	// Block tracking
	lastBlockHeight    atomic.Uint64
	lastBlockTimestamp atomic.Uint64

	// Metrics
	totalDispatched atomic.Uint64
	totalReceived   atomic.Uint64

	// Recent results buffer for EVM shadow comparison
	recentResults sync.Map // task_id → *pb.TaskResult

	// txHash → taskID mapping for shadow comparison with on-chain results
	txHashToTaskID sync.Map // hex tx hash → uint32 task ID

	// State diff broadcaster for L4 agents
	diffBroadcaster *StateDiffBroadcaster

	// Persistent diff store (disk-backed) for catch-up
	diffStore *DiffStore
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
	coord := &Coordinator{
		cfg:             cfg,
		actPool:         actPool,
		prefetcher:      NewPrefetcher(stateReader, logger),
		registry:        registry,
		scheduler:       NewScheduler(registry, logger),
		shadow:          NewShadowComparator(logger),
		reward:          NewRewardDistributor(cfg.Reward, cfg.DelegateAddress, logger),
		settler:         o.settler,
		diffBroadcaster: NewStateDiffBroadcaster(cfg.DiffBufferSize, logger),
		logger:          logger,
	}

	// Open persistent diff store if enabled
	if cfg.DiffStoreEnabled {
		path := cfg.DiffStorePath
		if path == "" && o.dataDir != "" {
			path = filepath.Join(o.dataDir, "statediffs.db")
		}
		if path != "" {
			ds, err := OpenDiffStore(path, logger)
			if err != nil {
				logger.Error("failed to open diffstore, continuing without persistence", zap.Error(err))
			} else {
				coord.diffStore = ds
			}
		}
	}

	return coord
}

// Option configures the coordinator.
type Option func(*options)

type options struct {
	logger  *zap.Logger
	dataDir string // data directory for persistent stores
	settler RewardSettler
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

// WithDataDir sets the directory for persistent stores (e.g. statediffs.db).
func WithDataDir(dir string) Option {
	return func(o *options) { o.dataDir = dir }
}

// WithRewardSettler sets the on-chain reward settler.
// When set, distributeEpochReward will call depositAndSettle on the contract.
func WithRewardSettler(s RewardSettler) Option {
	return func(o *options) { o.settler = s }
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
		defer func() {
			if r := recover(); r != nil {
				c.logger.Error("panic in gRPC server goroutine", zap.Any("recover", r))
			}
		}()
		c.logger.Info("gRPC server listening", zap.Int("port", c.cfg.GRPCPort))
		if err := c.grpcServer.Serve(lis); err != nil {
			c.logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	// Start main polling loop
	go func() {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Error("panic in IOSwarm runLoop", zap.Any("recover", r))
			}
		}()
		c.runLoop(ctx)
	}()

	// Start agent eviction loop
	go func() {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Error("panic in IOSwarm evictionLoop", zap.Any("recover", r))
			}
		}()
		c.evictionLoop(ctx)
	}()

	// Start epoch reward loop
	go func() {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Error("panic in IOSwarm epochLoop", zap.Any("recover", r))
			}
		}()
		c.epochLoop(ctx)
	}()

	// Start SwarmAPI HTTP server
	if c.cfg.SwarmAPIPort > 0 {
		api := NewSwarmAPI(c, c.reward)
		var handler http.Handler = api.Handler()
		if c.cfg.MasterSecret != "" {
			handler = tokenHTTPMiddleware(c.cfg.MasterSecret, handler)
		}
		c.httpServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", c.cfg.SwarmAPIPort),
			Handler: handler,
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					c.logger.Error("panic in SwarmAPI server", zap.Any("recover", r))
				}
			}()
			c.logger.Info("swarm API listening", zap.Int("port", c.cfg.SwarmAPIPort))
			if err := c.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				c.logger.Error("swarm API error", zap.Error(err))
			}
		}()
	}

	return nil
}

// Stop gracefully shuts down the coordinator with a 5-second timeout.
// If GracefulStop doesn't complete in time, it falls back to a hard Stop
// to avoid blocking the node's shutdown sequence.
func (c *Coordinator) Stop() {
	c.logger.Info("stopping IOSwarm coordinator")
	if c.diffStore != nil {
		if err := c.diffStore.Close(); err != nil {
			c.logger.Warn("diffstore close error", zap.Error(err))
		}
	}
	if c.httpServer != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := c.httpServer.Shutdown(shutCtx); err != nil {
			c.logger.Warn("IOSwarm HTTP server shutdown error", zap.Error(err))
		} else {
			c.logger.Info("IOSwarm HTTP server stopped")
		}
	}
	if c.grpcServer != nil {
		done := make(chan struct{})
		go func() {
			c.grpcServer.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
			c.logger.Info("IOSwarm gRPC server stopped gracefully")
		case <-time.After(5 * time.Second):
			c.logger.Warn("IOSwarm gRPC graceful stop timed out, forcing stop")
			c.grpcServer.Stop()
		}
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
			task.Sender.Address = io1ToHex(task.Sender.Address)
		} else {
			task.Sender = &pb.AccountSnapshot{Address: io1ToHex(tx.From)}
		}
		if tx.To != "" {
			if snap, ok := stateMap[tx.To]; ok {
				task.Receiver = snap
				task.Receiver.Address = io1ToHex(task.Receiver.Address)
			} else {
				task.Receiver = &pb.AccountSnapshot{Address: io1ToHex(tx.To)}
			}
		}

		// L3: attach EVM execution data
		if level == pb.TaskLevel_L3_FULL_EXECUTE {
			c.enrichL3Task(task, tx, blockHeight)
		}

		// Track txHash → taskID for shadow comparison
		c.txHashToTaskID.Store(tx.Hash, taskID)

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

// ReceiveBlock implements blockchain.BlockCreationSubscriber.
// Called by iotex-core after each block is committed to chain.
// It maps on-chain tx results back to dispatched task IDs for shadow comparison.
func (c *Coordinator) ReceiveBlock(blk *block.Block) error {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("panic in ReceiveBlock", zap.Any("recover", r))
		}
	}()

	blockHeight := blk.Height()
	c.lastBlockTimestamp.Store(uint64(blk.Timestamp().Unix()))
	actualResults := make(map[uint32]bool)
	matched := 0

	for i, act := range blk.Actions {
		h, err := act.Hash()
		if err != nil {
			continue
		}
		txHash := hex.EncodeToString(h[:])

		// Look up the task ID we assigned to this tx
		val, ok := c.txHashToTaskID.LoadAndDelete(txHash)
		if !ok {
			continue // tx wasn't dispatched by us (or already processed)
		}
		taskID := val.(uint32)
		matched++

		// Receipt status: 1 = success (tx valid and executed), 0 = reverted
		valid := true
		if i < len(blk.Receipts) {
			valid = blk.Receipts[i].Status == 1
		}
		actualResults[taskID] = valid
	}

	if matched > 0 || blockHeight%100 == 0 {
		c.logger.Info("ReceiveBlock",
			zap.Uint64("block", blockHeight),
			zap.Int("actions", len(blk.Actions)),
			zap.Int("matched", matched))
	}

	// Run shadow comparison
	c.OnBlockExecuted(blockHeight, actualResults)

	// Periodic cleanup of stale txHash entries (>10000 blocks old)
	if blockHeight%1000 == 0 {
		c.txHashToTaskID.Range(func(key, _ any) bool {
			c.txHashToTaskID.Delete(key)
			return true
		})
	}

	return nil
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

	// Use configured epoch reward (configurable via epochRewardIOTX in config).
	// In future, fetch actual delegate epoch reward from chain.
	rewardIOTX := c.cfg.EpochRewardIOTX
	if rewardIOTX <= 0 {
		rewardIOTX = 800
	}
	one := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	epochReward := new(big.Float).Mul(big.NewFloat(rewardIOTX), new(big.Float).SetInt(one))
	epochRewardInt, _ := epochReward.Int(nil)

	summary := c.reward.Distribute(epochRewardInt)

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

	// On-chain settlement: call depositAndSettle on the reward pool contract
	c.logger.Info("settlement check",
		zap.Bool("settler_nil", c.settler == nil),
		zap.Int("payouts", len(summary.Payouts)),
		zap.Uint64("epoch", summary.Epoch))
	if c.settler != nil && len(summary.Payouts) > 0 {
		agents := make([]string, 0, len(summary.Payouts))
		weights := make([]*big.Int, 0, len(summary.Payouts))
		for _, p := range summary.Payouts {
			c.logger.Info("settlement payout entry",
				zap.String("agent", p.AgentID),
				zap.String("wallet", p.WalletAddress),
				zap.Uint64("tasks", p.TasksDone))
			if p.WalletAddress == "" {
				continue
			}
			agents = append(agents, p.WalletAddress)
			effectiveWeight := int64(p.TasksDone) * 1000
			if p.BonusApplied {
				effectiveWeight = int64(float64(p.TasksDone) * c.cfg.Reward.BonusMultiplier * 1000)
			}
			weights = append(weights, big.NewInt(effectiveWeight))
		}

		c.logger.Info("settlement agents collected",
			zap.Int("count", len(agents)),
			zap.Strings("wallets", agents))
		if len(agents) > 0 {
			agentPool := new(big.Int).Set(summary.AgentPool)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := c.settler.Settle(ctx, agents, weights, agentPool, agents); err != nil {
				c.logger.Error("on-chain settlement failed",
					zap.Uint64("epoch", summary.Epoch),
					zap.Error(err))
			} else {
				c.logger.Info("on-chain settlement submitted",
					zap.Uint64("epoch", summary.Epoch),
					zap.Int("agents", len(agents)),
					zap.String("value", FormatIOTX(agentPool)))
			}
		}
	}
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
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("panic in enrichL3Task", zap.Any("recover", r), zap.String("tx", tx.Hash))
		}
	}()
	// Build EvmTx from PendingTx fields (convert io1 addresses to 0x hex
	// so the agent can use common.HexToAddress correctly)
	task.EvmTx = &pb.EvmTx{
		To:       io1ToHex(tx.To),
		Value:    tx.Amount,
		Data:     tx.Data,
		GasLimit: tx.GasLimit,
		GasPrice: tx.GasPrice,
	}

	// Build BlockContext — use last known block timestamp (not time.Now)
	ts := c.lastBlockTimestamp.Load()
	if ts == 0 {
		ts = uint64(time.Now().Unix()) // fallback before first ReceiveBlock
	}
	task.BlockContext = &pb.BlockCtx{
		Number:    blockHeight,
		Timestamp: ts,
		GasLimit:  30_000_000,
		BaseFee:   "0",
		Coinbase:  io1ToHex(c.cfg.DelegateAddress),
	}

	// Fetch contract bytecode and storage for receiver (if it has CodeHash)
	if task.Receiver != nil && len(task.Receiver.CodeHash) > 0 && tx.To != "" {
		// Simulate EVM execution to discover all accessed storage slots and addresses.
		// This pre-executes the tx read-only against current state to capture
		// the exact slots the EVM touches (via EIP-2929 access list tracking).
		accessedSlots, err := c.prefetcher.stateReader.SimulateAccessList(
			tx.From, tx.To, tx.Data, tx.Amount, tx.GasLimit,
		)
		if err != nil {
			c.logger.Warn("SimulateAccessList failed, falling back to slot 0",
				zap.String("tx", tx.Hash), zap.Error(err))
			// Fallback: prefetch slot 0 only
			accessedSlots = map[string][]string{
				tx.To: {"0x0000000000000000000000000000000000000000000000000000000000000000"},
			}
		} else {
			totalSlots := 0
			for _, s := range accessedSlots {
				totalSlots += len(s)
			}
			c.logger.Debug("SimulateAccessList OK",
				zap.String("tx", tx.Hash),
				zap.Int("addresses", len(accessedSlots)),
				zap.Int("totalSlots", totalSlots))
		}

		// Prefetch code for all accessed contract addresses (including cross-contract calls)
		codeAddrs := []string{tx.To}
		for addr := range accessedSlots {
			if addr != tx.To {
				codeAddrs = append(codeAddrs, addr)
			}
		}
		codes := c.prefetcher.PrefetchCode(codeAddrs)
		// Re-key ContractCode map from io1 → 0x hex
		if len(codes) > 0 {
			task.ContractCode = make(map[string][]byte, len(codes))
			for addr, code := range codes {
				task.ContractCode[io1ToHex(addr)] = code
			}
		}

		// Prefetch all discovered storage slots, re-keying to 0x hex
		task.StorageSlots = make(map[string]map[string]string)
		for addr, slots := range accessedSlots {
			if len(slots) == 0 {
				continue
			}
			storage := c.prefetcher.PrefetchStorage(addr, slots)
			if len(storage) > 0 {
				task.StorageSlots[io1ToHex(addr)] = storage
			}
		}
	}
}

// io1ToHex converts an io1-format address to its 0x-hex representation.
// This is necessary because the agent uses common.HexToAddress() which only
// works with hex addresses, not bech32-encoded io1 addresses.
func io1ToHex(addr string) string {
	ioAddr, err := address.FromString(addr)
	if err != nil {
		return addr // return as-is if not parseable
	}
	return "0x" + hex.EncodeToString(ioAddr.Bytes())
}

// ReceiveStateDiff is called by the stateDB diff callback after each block commit.
// It converts WriteQueueEntry to StateDiffEntry and publishes to the broadcaster.
func (c *Coordinator) ReceiveStateDiff(height uint64, entries []StateDiffEntry, digest []byte) {
	diff := &StateDiff{
		Height:      height,
		Entries:     entries,
		DigestBytes: digest,
	}

	// Persist to disk before broadcasting
	if c.diffStore != nil {
		if err := c.diffStore.Append(diff); err != nil {
			c.logger.Error("failed to persist state diff", zap.Uint64("height", height), zap.Error(err))
		}
	}

	c.diffBroadcaster.Publish(diff)
	c.logger.Debug("state diff published",
		zap.Uint64("height", height),
		zap.Int("entries", len(entries)),
		zap.Int("subscribers", c.diffBroadcaster.SubscriberCount()))
}

// DiffBroadcaster returns the state diff broadcaster for gRPC streaming.
func (c *Coordinator) DiffBroadcaster() *StateDiffBroadcaster {
	return c.diffBroadcaster
}

// DiffStore returns the persistent diff store (may be nil if disabled).
func (c *Coordinator) DiffStore() *DiffStore {
	return c.diffStore
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
