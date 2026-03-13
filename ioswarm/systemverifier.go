package ioswarm

import (
	"context"
	"fmt"

	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

// SystemActionVerifier verifies that system actions in a block are correct.
// The operator uses this to verify agent-submitted blocks before signing.
type SystemActionVerifier struct {
	logger *zap.Logger
}

// NewSystemActionVerifier creates a new system action verifier.
func NewSystemActionVerifier(logger *zap.Logger) *SystemActionVerifier {
	return &SystemActionVerifier{logger: logger}
}

// VerifySystemActions verifies all system actions in the block.
// Returns nil if all system actions are valid, or an error describing what's wrong.
func (v *SystemActionVerifier) VerifySystemActions(ctx context.Context, blk *block.Block) error {
	for i, act := range blk.Actions {
		if !action.IsSystemAction(act) {
			continue // Skip non-system actions
		}

		switch sysAct := act.Action().(type) {
		case *action.GrantReward:
			if err := v.verifyGrantReward(ctx, blk.Height(), i, sysAct); err != nil {
				return fmt.Errorf("invalid GrantReward at index %d: %w", i, err)
			}

		case *action.PutPollResult:
			if err := v.verifyPutPollResult(ctx, blk.Height(), i, sysAct); err != nil {
				return fmt.Errorf("invalid PutPollResult at index %d: %w", i, err)
			}

		default:
			// Unknown system action type - reject
			return fmt.Errorf("unknown system action type at index %d: %T", i, sysAct)
		}
	}
	return nil
}

// verifyGrantReward verifies a GrantReward system action.
func (v *SystemActionVerifier) verifyGrantReward(ctx context.Context, blockHeight uint64, index int, gr *action.GrantReward) error {
	// GrantReward should be at the expected height
	if gr.Height() != blockHeight {
		return fmt.Errorf("grant reward height %d != block height %d", gr.Height(), blockHeight)
	}

	// Verify the reward type is valid
	if gr.RewardType() != action.BlockReward && gr.RewardType() != action.EpochReward {
		return fmt.Errorf("invalid reward type: %d", gr.RewardType())
	}

	v.logger.Debug("GrantReward verified",
		zap.Uint64("height", blockHeight),
		zap.Int("index", index),
		zap.Uint32("type", uint32(gr.RewardType())))

	return nil
}

// verifyPutPollResult verifies a PutPollResult system action.
func (v *SystemActionVerifier) verifyPutPollResult(ctx context.Context, blockHeight uint64, index int, ppr *action.PutPollResult) error {
	// PutPollResult should be at the expected height
	if ppr.Height() != blockHeight {
		return fmt.Errorf("put poll result height %d != block height %d", ppr.Height(), blockHeight)
	}

	// Verify the candidates list is present
	if len(ppr.Candidates()) == 0 {
		return fmt.Errorf("put poll result has no candidates")
	}

	v.logger.Debug("PutPollResult verified",
		zap.Uint64("height", blockHeight),
		zap.Int("index", index),
		zap.Int("candidates", len(ppr.Candidates())))

	return nil
}

// VerifyBlockHash verifies the block hash matches the calculated hash.
func (v *SystemActionVerifier) VerifyBlockHash(blk *block.Block) error {
	expectedHash := blk.HashBlock()
	actualHash := blk.Header.HashBlock()

	if expectedHash != actualHash {
		return fmt.Errorf("block hash mismatch: expected %x, got %x", expectedHash[:], actualHash[:])
	}

	return nil
}

// VerifyBlockSignature verifies the block was signed correctly.
// In L5, the agent signs with its own key, and the operator re-signs after verification.
func (v *SystemActionVerifier) VerifyBlockSignature(blk *block.Block) error {
	// Use the header's built-in signature verification
	if !blk.Header.VerifySignature() {
		return fmt.Errorf("block signature verification failed")
	}

	return nil
}

// VerifyBlock verifies all aspects of a block before signing.
// This is the main entry point for the operator to verify an agent's block.
func (v *SystemActionVerifier) VerifyBlock(ctx context.Context, blk *block.Block) error {
	// 1. Verify block hash
	if err := v.VerifyBlockHash(blk); err != nil {
		return fmt.Errorf("block hash verification failed: %w", err)
	}

	// 2. Verify system actions
	if err := v.VerifySystemActions(ctx, blk); err != nil {
		return fmt.Errorf("system action verification failed: %w", err)
	}

	// 3. Verify signature (from agent)
	if err := v.VerifyBlockSignature(blk); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	v.logger.Info("block verified successfully",
		zap.Uint64("height", blk.Height()),
		zap.Int("actions", len(blk.Actions)))

	return nil
}

// BlockSignatureInfo contains info needed for the operator to re-sign a block.
type BlockSignatureInfo struct {
	BlockHash hash.Hash256
	AgentID   string
	Height    uint64
}

// ExtractSignatureInfo extracts signature info from a verified block.
func (v *SystemActionVerifier) ExtractSignatureInfo(blk *block.Block) *BlockSignatureInfo {
	return &BlockSignatureInfo{
		BlockHash: blk.HashBlock(),
		Height:    blk.Height(),
		// AgentID would come from the BlockCandidate metadata
	}
}
