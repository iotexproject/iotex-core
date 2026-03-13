package ioswarm

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	iotextypes "github.com/iotexproject/iotex-proto/golang/iotextypes"

	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
)

// BlockSelector receives block candidates from multiple agents and selects the best one.
// The operator uses this to choose which block to sign and broadcast.
type BlockSelector struct {
	mu              sync.RWMutex
	candidates      map[uint64][]*blockCandidateEntry // height → candidates
	verifier        *SystemActionVerifier
	deserializer    *block.Deserializer
	operatorPrivKey crypto.PrivateKey
	maxCandidates   int           // max candidates per height
	selectionTime   time.Duration // time to wait for candidates before selection
	logger          *zap.Logger

	// Metrics
	totalReceived   uint64
	totalSelected   uint64
	totalRejected   uint64
}

type blockCandidateEntry struct {
	candidate  *pb.BlockCandidate
	block      *block.Block
	receivedAt time.Time
	agentID    string
}

// NewBlockSelector creates a new block selector.
func NewBlockSelector(
	operatorPrivKey crypto.PrivateKey,
	evmNetworkID uint32,
	maxCandidates int,
	selectionTime time.Duration,
	logger *zap.Logger,
) *BlockSelector {
	return &BlockSelector{
		candidates:      make(map[uint64][]*blockCandidateEntry),
		verifier:        NewSystemActionVerifier(logger),
		deserializer:    block.NewDeserializer(evmNetworkID),
		operatorPrivKey: operatorPrivKey,
		maxCandidates:   maxCandidates,
		selectionTime:   selectionTime,
		logger:          logger,
	}
}

// SubmitCandidate receives a block candidate from an agent.
// Returns whether the candidate was accepted for consideration.
func (s *BlockSelector) SubmitCandidate(ctx context.Context, candidate *pb.BlockCandidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalReceived++

	// Parse the block from proto
	blkProto := &iotextypes.Block{}
	if err := proto.Unmarshal(candidate.BlockProto, blkProto); err != nil {
		s.totalRejected++
		return fmt.Errorf("failed to unmarshal block proto: %w", err)
	}

	blk, err := s.deserializer.FromBlockProto(blkProto)
	if err != nil {
		s.totalRejected++
		return fmt.Errorf("failed to deserialize block: %w", err)
	}

	// Verify block hash matches
	expectedHash := blk.HashBlock()
	if hex.EncodeToString(expectedHash[:]) != hex.EncodeToString(candidate.BlockHash) {
		s.totalRejected++
		return fmt.Errorf("block hash mismatch")
	}

	// Verify the agent's signature
	if len(candidate.Signature) == 0 {
		s.totalRejected++
		return fmt.Errorf("candidate has no signature")
	}

	// Store the candidate
	height := blk.Height()
	entry := &blockCandidateEntry{
		candidate:  candidate,
		block:      blk,
		receivedAt: time.Now(),
		agentID:    candidate.AgentID,
	}

	s.candidates[height] = append(s.candidates[height], entry)

	// Limit candidates per height
	if len(s.candidates[height]) > s.maxCandidates {
		// Remove oldest
		oldestIdx := 0
		for i, e := range s.candidates[height] {
			if e.receivedAt.Before(s.candidates[height][oldestIdx].receivedAt) {
				oldestIdx = i
			}
		}
		s.candidates[height] = append(
			s.candidates[height][:oldestIdx],
			s.candidates[height][oldestIdx+1:]...,
		)
	}

	s.logger.Debug("candidate submitted",
		zap.String("agent", candidate.AgentID),
		zap.Uint64("height", height),
		zap.Int("txs", candidate.TxCount),
		zap.Uint64("gas", candidate.GasUsed))

	return nil
}

// SelectBlock selects the best block for the given height.
// It waits up to selectionTime for candidates to arrive, then picks the best one.
func (s *BlockSelector) SelectBlock(ctx context.Context, height uint64) (*block.Block, string, error) {
	// Wait for candidates or timeout
	deadline := time.NewTimer(s.selectionTime)
	defer deadline.Stop()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-deadline.C:
			// Time's up - select from what we have
			return s.selectBest(height)
		case <-ticker.C:
			s.mu.RLock()
			candidates := s.candidates[height]
			s.mu.RUnlock()

			if len(candidates) > 0 {
				// We have candidates - wait a bit more or select if we have enough
				if len(candidates) >= s.maxCandidates || time.Since(candidates[0].receivedAt) > s.selectionTime/2 {
					return s.selectBest(height)
				}
			}
		}
	}
}

// selectBest selects the best block from available candidates.
// Selection criteria:
// 1. Highest gas used (more productive)
// 2. Most transactions (more useful)
// 3. Earliest received (fairness)
func (s *BlockSelector) selectBest(height uint64) (*block.Block, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidates := s.candidates[height]
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no candidates for height %d", height)
	}

	// Sort by: gas used (desc), tx count (desc), received time (asc)
	var best *blockCandidateEntry
	for _, entry := range candidates {
		if best == nil {
			best = entry
			continue
		}

		// Compare gas used
		if entry.candidate.GasUsed > best.candidate.GasUsed {
			best = entry
			continue
		}
		if entry.candidate.GasUsed < best.candidate.GasUsed {
			continue
		}

		// Compare tx count
		if entry.candidate.TxCount > best.candidate.TxCount {
			best = entry
			continue
		}
		if entry.candidate.TxCount < best.candidate.TxCount {
			continue
		}

		// Compare received time (earlier is better)
		if entry.receivedAt.Before(best.receivedAt) {
			best = entry
		}
	}

	// Verify the selected block
	if err := s.verifier.VerifyBlock(context.Background(), best.block); err != nil {
		s.totalRejected++
		// Remove the bad candidate
		s.removeCandidateLocked(height, best.agentID)
		return nil, "", fmt.Errorf("selected block verification failed: %w", err)
	}

	s.totalSelected++

	// Clean up candidates for this height
	delete(s.candidates, height)

	s.logger.Info("selected best block",
		zap.String("agent", best.agentID),
		zap.Uint64("height", height),
		zap.Int("txs", best.candidate.TxCount),
		zap.Uint64("gas", best.candidate.GasUsed))

	return best.block, best.agentID, nil
}

// removeCandidateLocked removes a candidate from the list (must hold lock).
func (s *BlockSelector) removeCandidateLocked(height uint64, agentID string) {
	candidates := s.candidates[height]
	for i, entry := range candidates {
		if entry.agentID == agentID {
			s.candidates[height] = append(candidates[:i], candidates[i+1:]...)
			return
		}
	}
}

// ResignBlock re-signs a block with the operator's key.
// This replaces the agent's signature with the operator's signature.
func (s *BlockSelector) ResignBlock(blk *block.Block) (*block.Block, error) {
	// Get the block proto
	blkProto := blk.ConvertToBlockPb()

	// Sign the block hash with operator's key
	blockHash := blk.HashBlock()
	sig, err := s.operatorPrivKey.Sign(blockHash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign block: %w", err)
	}

	// Update the signature in the proto
	blkProto.Header.Signature = sig
	blkProto.Header.ProducerPubkey = s.operatorPrivKey.PublicKey().Bytes()

	// Convert back to block
	newBlk, err := s.deserializer.FromBlockProto(blkProto)
	if err != nil {
		return nil, fmt.Errorf("failed to convert re-signed block: %w", err)
	}

	s.logger.Info("block re-signed by operator",
		zap.Uint64("height", newBlk.Height()))

	return newBlk, nil
}

// Stats returns selector statistics.
func (s *BlockSelector) Stats() (received, selected, rejected uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalReceived, s.totalSelected, s.totalRejected
}

// CandidateCount returns the number of candidates for a given height.
func (s *BlockSelector) CandidateCount(height uint64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.candidates[height])
}

// PruneOldCandidates removes candidates older than maxAge.
func (s *BlockSelector) PruneOldCandidates(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	pruned := 0

	for height, candidates := range s.candidates {
		var remaining []*blockCandidateEntry
		for _, entry := range candidates {
			if entry.receivedAt.After(cutoff) {
				remaining = append(remaining, entry)
			} else {
				pruned++
			}
		}
		if len(remaining) == 0 {
			delete(s.candidates, height)
		} else {
			s.candidates[height] = remaining
		}
	}

	if pruned > 0 {
		s.logger.Debug("pruned old candidates", zap.Int("count", pruned))
	}

	return pruned
}
