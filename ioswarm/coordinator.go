package ioswarm

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

// L5Coordinator is the coordinator for L5 block building.
// It receives block candidates from agents and selects the best one for the operator to sign.
type L5Coordinator struct {
	cfg          Config
	registry     *Registry
	selector     *BlockSelector
	verifier     *SystemActionVerifier
	reward       *RewardDistributor
	settler      RewardSettler
	logger       *zap.Logger
	grpcServer   *grpc.Server

	// Pending payouts: agent_id → PayoutInfo
	pendingPayouts sync.Map

	// Block tracking
	lastBlockHeight    atomic.Uint64
	lastBlockTimestamp atomic.Uint64

	// Metrics
	totalCandidates atomic.Uint64
	totalSelected   atomic.Uint64
	totalRejected   atomic.Uint64

	// Running state
	running atomic.Bool
}

// L5Option configures the L5 coordinator.
type L5Option func(*l5Options)

type l5Options struct {
	logger       *zap.Logger
	settler      RewardSettler
	operatorKey  crypto.PrivateKey
	evmNetworkID uint32
}

func defaultL5Options() l5Options {
	logger, _ := zap.NewProduction()
	if logger == nil {
		logger = zap.NewNop()
	}
	return l5Options{logger: logger}
}

// WithL5Logger sets the coordinator's logger.
func WithL5Logger(l *zap.Logger) L5Option {
	return func(o *l5Options) { o.logger = l }
}

// WithL5RewardSettler sets the on-chain reward settler.
func WithL5RewardSettler(s RewardSettler) L5Option {
	return func(o *l5Options) { o.settler = s }
}

// WithL5OperatorKey sets the operator's private key for block signing.
func WithL5OperatorKey(pk crypto.PrivateKey) L5Option {
	return func(o *l5Options) { o.operatorKey = pk }
}

// WithL5EVMNetworkID sets the EVM network ID.
func WithL5EVMNetworkID(id uint32) L5Option {
	return func(o *l5Options) { o.evmNetworkID = id }
}

// NewL5Coordinator creates a new L5 coordinator.
func NewL5Coordinator(
	cfg Config,
	opts ...L5Option,
) *L5Coordinator {
	o := defaultL5Options()
	for _, fn := range opts {
		fn(&o)
	 }

	 registry := NewRegistry(cfg.MaxAgents)

    c := &L5Coordinator{
        cfg:          cfg,
        registry:     registry,
        reward:       NewRewardDistributor(cfg.Reward, cfg.DelegateAddress, o.logger),
        settler:      o.settler,
        logger:       o.logger,
        pendingPayouts: sync.Map{},
    }

    if o.operatorKey != nil {
        c.selector = NewBlockSelector(o.operatorKey, o.evmNetworkID, cfg.MaxAgents, 1*time.Second, o.logger)
        c.verifier = NewSystemActionVerifier(o.logger)
    }

    return c
}

// Start launches the coordinator's gRPC server.
func (c *L5Coordinator) Start(ctx context.Context) error {
    if !c.running.CompareAndSwap(false, true) {
        return nil
    }

    c.logger.Info("starting L5 coordinator",
        zap.Int("port", c.cfg.GRPCPort))

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
    RegisterL5BlockBuildingServer(c.grpcServer, &l5GRPCHandler{coord: c})

    go func() {
        defer func() {
            if r := recover(); r != nil {
                c.logger.Error("panic in gRPC server goroutine", zap.Any("recover", r))
            }
        }()
        c.logger.Info("L5 gRPC server listening", zap.Int("port", c.cfg.GRPCPort))
        if err := c.grpcServer.Serve(lis); err != nil {
            c.logger.Error("L5 gRPC server error", zap.Error(err))
        }
    }()

    // Start epoch reward loop
    go c.epochLoop(ctx)

    // Start agent eviction loop
    go c.evictionLoop(ctx)

    c.running.Store(true)
    return nil
}

// Stop gracefully shuts down the coordinator.
func (c *L5Coordinator) Stop() {
    if !c.running.CompareAndSwap(true, false) {
        return
    }
    c.logger.Info("stopping L5 coordinator")

    if c.grpcServer != nil {
        done := make(chan struct{})
        go func() {
            c.grpcServer.GracefulStop()
            close(done)
        }()
        select {
        case <-done:
            c.logger.Info("L5 gRPC server stopped gracefully")
        case <-time.After(5 * time.Second):
            c.logger.Warn("L5 gRPC graceful stop timed out, forcing stop")
            c.grpcServer.Stop()
        }
    }
}

// RegisterAgent registers an L5 agent.
func (c *L5Coordinator) RegisterAgent(agentID, walletAddr, region, version string) error {
    c.registry.RegisterL5(agentID, walletAddr, region, version)
    c.logger.Info("L5 agent registered",
        zap.String("agent", agentID),
        zap.String("wallet", walletAddr))
    return nil
}

// SubmitBlockCandidate receives a block candidate from an agent.
func (c *L5Coordinator) SubmitBlockCandidate(candidate *pb.BlockCandidate) (*pb.SubmitBlockCandidateResponse, error) {
    c.totalCandidates.Add(1)

    // Submit to selector
    if err := c.selector.SubmitCandidate(context.Background(), candidate); err != nil {
        c.totalRejected.Add(1)
        return &pb.SubmitBlockCandidateResponse{
            Accepted: false,
            Reason:   err.Error(),
        }, nil
    }

    c.logger.Debug("block candidate submitted",
        zap.String("agent", candidate.AgentID),
        zap.Uint64("height", candidate.BlockHeight),
        zap.Int("txs", candidate.TxCount),
        zap.Uint64("gas", candidate.GasUsed))

    return &pb.SubmitBlockCandidateResponse{
        Accepted: true,
    }, nil
}

// SelectBlock selects the best block for the given height.
func (c *L5Coordinator) SelectBlock(ctx context.Context, height uint64) (*block.Block, string, error) {
    blk, agentID, err := c.selector.SelectBlock(ctx, height)
    if err != nil {
        return nil, "", err
    }

    c.totalSelected.Add(1)
    c.reward.RecordWork(agentID, 1, 1, 0)

    return blk, agentID, nil
}

// VerifyAndResignBlock verifies a block and re-signs it with the operator's key.
func (c *L5Coordinator) VerifyAndResignBlock(ctx context.Context, blk *block.Block) (*block.Block, error) {
    // Verify system actions
    if err := c.verifier.VerifyBlock(ctx, blk); err != nil {
        return nil, fmt.Errorf("verification failed: %w", err)
    }

    // Re-sign with operator's key
    return c.selector.ResignBlock(blk)
}

// Heartbeat processes an L5 agent heartbeat.
func (c *L5Coordinator) Heartbeat(agentID string, blocksBuilt, blocksAccepted, lastBuiltHeight uint64, cpuUsage, memUsage float64) *pb.HeartbeatL5Response {
    c.registry.HeartbeatL5(agentID)

    // Update metrics
    c.registry.UpdateMetrics(agentID, cpuUsage, memUsage)

    // Check for pending payout
    payout := c.consumePayout(agentID)

    return &pb.HeartbeatL5Response{
        Alive:     true,
        Directive: "continue",
        Payout:    payout,
    }
}

// Stats returns coordinator statistics.
func (c *L5Coordinator) Stats() (candidates, selected, rejected uint64) {
    return c.totalCandidates.Load(), c.totalSelected.Load(), c.totalRejected.Load()
}

// ReceiveBlock is called when a new block is committed to the chain.
func (c *L5Coordinator) ReceiveBlock(blk *block.Block) error {
    c.lastBlockHeight.Store(blk.Height())
    c.lastBlockTimestamp.Store(uint64(blk.Timestamp().Unix()))
    return nil
}

// evictionLoop periodically removes stale agents.
func (c *L5Coordinator) evictionLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            evicted := c.registry.EvictStale(60 * time.Second)
            for _, id := range evicted {
                c.logger.Info("evicted stale L5 agent", zap.String("agent", id))
            }
        }
    }
}

// epochLoop triggers reward distribution at epoch boundaries.
func (c *L5Coordinator) epochLoop(ctx context.Context) {
    epochDuration := time.Duration(c.cfg.Reward.EpochBlocks) * 10 * time.Second
    if epochDuration < 30*time.Second {
        epochDuration = 30 * time.Second
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
func (c *L5Coordinator) distributeEpochReward() {
    work := c.reward.CurrentWork()
    if len(work) == 0 {
        return
    }

    rewardIOTX := c.cfg.EpochRewardIOTX
    if rewardIOTX <= 0 {
        rewardIOTX = 800
    }
    one := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
    epochReward := new(big.Float).Mul(big.NewFloat(rewardIOTX), new(big.Float).SetInt(one))
    epochRewardInt, _ := epochReward.Int(nil)

    summary := c.reward.Distribute(epochRewardInt)

    for i, p := range summary.Payouts {
        c.pendingPayouts.Store(p.AgentID, &pb.PayoutInfo{
            Epoch:       summary.Epoch,
            AmountIOTX:  p.AmountIOTX,
            Rank:        i + 1,
            TotalAgents: summary.AgentCount,
        })
    }

    c.logger.Info("L5 epoch reward distributed",
        zap.Uint64("epoch", summary.Epoch),
        zap.Int("agents", summary.AgentCount))

    // On-chain settlement
    if c.settler != nil && len(summary.Payouts) > 0 {
        agents := make([]string, 0, len(summary.Payouts))
        weights := make([]*big.Int, 0, len(summary.Payouts))
        for _, p := range summary.Payouts {
            if p.WalletAddress == "" {
                continue
            }
            agents = append(agents, p.WalletAddress)
            weights = append(weights, big.NewInt(int64(p.TasksDone)*1000))
        }

        if len(agents) > 0 {
            agentPool := new(big.Int).Set(summary.AgentPool)
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()

            if err := c.settler.Settle(ctx, agents, weights, agentPool); err != nil {
                c.logger.Error("L5 on-chain settlement failed", zap.Error(err))
            } else {
                c.logger.Info("L5 on-chain settlement submitted",
                    zap.Int("agents", len(agents))
            }
        }
    }
}

// consumePayout removes and returns a pending payout for the agent.
func (c *L5Coordinator) consumePayout(agentID string) *pb.PayoutInfo {
    val, ok := c.pendingPayouts.LoadAndDelete(agentID)
    if !ok {
        return nil
    }
    return val.(*pb.PayoutInfo)
}

// l5GRPCHandler implements the L5BlockBuilding gRPC service.
type l5GRPCHandler struct {
    coord *L5Coordinator
}

// RegisterL5 handles L5 agent registration.
func (h *l5GRPCHandler) RegisterL5(ctx context.Context, req *pb.RegisterL5Request) (*pb.RegisterL5Response, error) {
    if err := h.coord.RegisterAgent(req.AgentID, req.WalletAddress, req.Region, req.Version); err != nil {
        return &pb.RegisterL5Response{Accepted: false, Reason: err.Error()}, nil
    }
    return &pb.RegisterL5Response{
        Accepted:           true,
        SelectionTimeoutMs: 1000,
    }, nil
}

// SubmitBlockCandidate handles block candidate submission.
func (h *l5GRPCHandler) SubmitBlockCandidate(ctx context.Context, req *pb.SubmitBlockCandidateRequest) (*pb.SubmitBlockCandidateResponse, error) {
    return h.coord.SubmitBlockCandidate(req.Candidate), nil
}

// HeartbeatL5 handles L5 agent heartbeats.
func (h *l5GRPCHandler) HeartbeatL5(ctx context.Context, req *pb.HeartbeatL5Request) (*pb.HeartbeatL5Response, error) {
    return h.coord.Heartbeat(req.AgentID, req.BlocksBuilt, req.BlocksAccepted, req.LastBuiltHeight, req.CPUUsage, req.MemUsage), nil
}