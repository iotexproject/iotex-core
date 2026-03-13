package ioswarm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

// BlockBuilderFactory creates blocks using iotex-core's Factory.
// This is the L5 agent's capability: building complete blocks independently.
type BlockBuilderFactory struct {
	factory    BlockFactory
	actPool    ActPoolReader
	signerPriv crypto.PrivateKey
	logger     *zap.Logger
}

// BlockFactory is the interface for minting blocks (subset of factory.Factory).
type BlockFactory interface {
	Mint(ctx context.Context, ap interface{}, pk crypto.PrivateKey) (*block.Block, error)
}

// NewBlockBuilderFactory creates a new block builder factory.
func NewBlockBuilderFactory(factory BlockFactory, actPool ActPoolReader, signerPriv crypto.PrivateKey, logger *zap.Logger) *BlockBuilderFactory {
	return &BlockBuilderFactory{
		factory:    factory,
		actPool:    actPool,
		signerPriv: signerPriv,
		logger:     logger,
	}
}

// BuildBlock builds a complete block for the given height.
// The agent signs the block with its own key, the operator will re-sign.
func (bbf *BlockBuilderFactory) BuildBlock(ctx context.Context) (*pb.BlockCandidate, error) {
	blk, err := bbf.factory.Mint(ctx, bbf.actPool, bbf.signerPriv)
	if err != nil {
		return nil, fmt.Errorf("failed to mint block: %w", err)
	}

	// Serialize block proto
	blkProto := blk.ConvertToBlockPb()
	blkBytes, err := proto.Marshal(blkProto)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize block: %w", err)
	}

	// Build receipt data (simplified for transmission)
	receipts := make([]*pb.ReceiptData, len(blk.Receipts))
	for i, r := range blk.Receipts {
		receipts[i] = &pb.ReceiptData{
			Status:          r.Status,
			GasUsed:         r.GasConsumed,
			ContractAddress: r.ContractAddress,
		}
	}

	// Calculate total gas used
	var totalGasUsed uint64
	for _, r := range blk.Receipts {
		totalGasUsed += r.GasConsumed
	}

	// Sign block hash with agent's key
	blockHash := blk.HashBlock()
	sig, err := bbf.signerPriv.Sign(blockHash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign block hash: %w", err)
	}

	candidate := &pb.BlockCandidate{
		AgentID:     hex.EncodeToString(bbf.signerPriv.PublicKey().Bytes()),
		BlockHeight: blk.Height(),
		BlockProto:  blkBytes,
		Receipts:    receipts,
		GasUsed:     totalGasUsed,
		TxCount:     len(blk.Actions),
		Timestamp:   uint64(blk.Timestamp().Unix()),
		Signature:   sig,
		BlockHash:   blockHash[:],
	}

	bbf.logger.Info("built block candidate",
		zap.Uint64("height", blk.Height()),
		zap.Int("txs", len(blk.Actions)),
		zap.Uint64("gas", totalGasUsed))

	return candidate, nil
}

// AgentBlockBuilder is an L5 agent that builds blocks and submits to coordinator.
type AgentBlockBuilder struct {
	cfg          Config
	blockBuilder *BlockBuilderFactory
	agentID      string
	walletAddr   string
	logger       *zap.Logger

	running    atomic.Bool
	stopCh     chan struct{}
	wg         sync.WaitGroup

	// Metrics
	blocksBuilt    uint64
	blocksAccepted uint64
	totalRewards   float64

	// Coordinator connection (in production, this would be a gRPC client)
	coordinatorAddr string
	masterSecret    string
}

// NewAgentBlockBuilder creates an L5 agent block builder.
func NewAgentBlockBuilder(
	cfg Config,
	factory BlockFactory,
	actPool ActPoolReader,
	signerPriv crypto.PrivateKey,
	walletAddr string,
	logger *zap.Logger,
) *AgentBlockBuilder {
	bbf := NewBlockBuilderFactory(factory, actPool, signerPriv, logger)
	return &AgentBlockBuilder{
		cfg:          cfg,
		blockBuilder: bbf,
		walletAddr:   walletAddr,
		logger:       logger,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the block building loop.
func (a *AgentBlockBuilder) Start(ctx context.Context) error {
	if !a.running.CompareAndSwap(false, true) {
		return nil // already running
	}

	// Generate agent ID
	agentIDBytes := make([]byte, 16)
	rand.Read(agentIDBytes)
	a.agentID = hex.EncodeToString(agentIDBytes)

	a.wg.Add(1)
	go a.buildLoop(ctx)

	a.logger.Info("L5 agent block builder started",
		zap.String("agent_id", a.agentID))

	return nil
}

// Stop stops the block builder.
func (a *AgentBlockBuilder) Stop() {
	if !a.running.CompareAndSwap(true, false) {
		return
	}
	close(a.stopCh)
	a.wg.Wait()
	a.logger.Info("L5 agent block builder stopped")
}

// buildLoop continuously builds blocks when notified by coordinator.
func (a *AgentBlockBuilder) buildLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(500 * time.Millisecond) // Check for new block every 500ms
	defer ticker.Stop()

	lastBuiltHeight := uint64(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			currentHeight := a.blockBuilder.actPool.BlockHeight()
			targetHeight := currentHeight + 1

			if targetHeight <= lastBuiltHeight {
				continue // Already built this height
			}

			// Build block
			candidate, err := a.blockBuilder.BuildBlock(ctx)
			if err != nil {
				a.logger.Error("failed to build block",
					zap.Uint64("height", targetHeight),
					zap.Error(err))
				continue
			}

			a.blocksBuilt++
			lastBuiltHeight = targetHeight

			a.logger.Info("built block",
				zap.Uint64("height", targetHeight),
				zap.Int("txs", candidate.TxCount),
				zap.Uint64("gas", candidate.GasUsed))
		}
	}
}

// Stats returns current builder statistics.
func (a *AgentBlockBuilder) Stats() (built, accepted uint64, rewards float64) {
	return a.blocksBuilt, a.blocksAccepted, a.totalRewards
}

// SetCoordinator configures the coordinator connection.
func (a *AgentBlockBuilder) SetCoordinator(addr, secret string) {
	a.coordinatorAddr = addr
	a.masterSecret = secret
}
