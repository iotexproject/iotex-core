// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"time"

	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
)

// iip59OverrunTier is sized for two properties at once.
//
// The drain must not be able to finish: 400 voters at 20 a block need twenty
// continuation chunks, and a two-epoch era does not offer them. So the next
// freeze always supersedes a live window.
//
// And the gap between that supersession and the rollover that cleans up after
// it must be wide enough to show the symptom. The freeze rides PutPollResult,
// roughly one and a half epochs ahead of the boundary block where Phase A
// rewrites the cursor, and a continuation is emitted on every non-epoch-last
// block. numDelegates=5 with NumSubEpochs=2 makes an epoch 10 blocks, so that
// gap is about 15 blocks and carries roughly a dozen chunks. At one delegate
// an epoch is 2 blocks, the gap is 3, and a cursor that never retires looks
// almost exactly like one that does.
var iip59OverrunTier = perfTier{
	numDelegates:        5,
	numVoters:           400,
	epochsPerEra:        2,
	voterBudgetPerBlock: 20,
}

// TestIIP59SupersededDrainRetiresOnce is the Handle-path reproduction that no
// unit test in the rewarding package can provide.
//
// When a drain's copy-on-write window is superseded, the chunk fails with an
// abandon verdict and Handle retires the cursor. But Handle takes its snapshot
// at the top, before anything runs, and settleAction reverts to that snapshot
// for every Failure receipt -- so the retirement is rolled back together with
// the failed chunk that prompted it. The committed cursor stays non-terminal
// and CreatePostSystemActions emits the same failing chunk on every following
// non-boundary block.
//
// The symptom is one DRAIN_ABANDONED log per block rather than one per drain:
// logs ride the receipt and survive the revert that discards the state. So the
// observable is the log's own identity, "<era>:<cursorFreeze>:<windowFreeze>" --
// a retirement that sticks emits it once, a retirement that is rolled back
// emits it again on every following block until the era rolls over. Without
// the fix this run reports the same drain nine times on nine consecutive
// blocks.
//
// This cannot be written against the mock state manager. testdb.AllowRevert
// stubs Revert to a no-op ("No test may depend on revert semantics through
// it"), so a unit-level version passes whether or not the bug is present. Only
// a real working set reverts.
func TestIIP59SupersededDrainRetiresOnce(t *testing.T) {
	r := require.New(t)

	tier := iip59OverrunTier
	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)

	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()
	rewardProto := rewarding.FindProtocol(test.cs.Registry())
	r.NotNil(rewardProto, "rewarding protocol must be registered")

	blkTime := time.Unix(cfg.Genesis.Timestamp, 0)
	step := 1 * time.Second

	// Two full eras is enough: the drain starts in the first, its window is
	// superseded at the boundary into the second, and the blocks after that are
	// where a cursor that failed to retire keeps re-emitting.
	maxBlocks := drainMintCeiling(tier, 2)

	// Keyed by the log's own drain identity, "<era>:<cursorFreeze>:<windowFreeze>".
	// One entry per superseded drain is correct; more than one entry for the
	// same key is the retirement failing to stick.
	perDrain := map[string][]uint64{}
	chunkActions := 0
	for minted := 0; minted < maxBlocks; minted++ {
		blkTime = blkTime.Add(step)
		_, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())
		height := bc.TipHeight()

		drains, chunks, err := drainAbandonedLogs(test, height)
		r.NoErrorf(err, "read receipts at height %d", height)
		chunkActions += chunks
		for _, id := range drains {
			perDrain[id] = append(perDrain[id], height)
		}
	}

	r.NotEmptyf(perDrain,
		"fixture never superseded a live drain in %d blocks; it proves nothing", maxBlocks)
	for id, heights := range perDrain {
		r.Lenf(heights, 1,
			"drain %s announced its retirement %d times (heights %v); the cursor never committed "+
				"as terminal, so the dispatcher kept re-emitting the same failing chunk "+
				"(%d chunk actions across the run)",
			id, len(heights), heights, chunkActions)
	}
}

// drainAbandonedLogs returns the identity of every DRAIN_ABANDONED reward log
// in the block at height, and how many voter-reward-chunk system actions that
// block contained.
func drainAbandonedLogs(test *e2etest, height uint64) ([]string, int, error) {
	blk, err := test.cs.BlockDAO().GetBlockByHeight(height)
	if err != nil {
		return nil, 0, err
	}
	chunks := 0
	for _, act := range blk.Actions {
		if gr, ok := act.Action().(*action.GrantReward); ok &&
			gr.RewardType() == action.VoterRewardChunk {
			chunks++
		}
	}
	receipts, err := test.cs.BlockDAO().GetReceipts(height)
	if err != nil {
		return nil, chunks, err
	}
	var found []string
	for _, receipt := range receipts {
		for _, l := range receipt.Logs() {
			logs, err := rewarding.UnmarshalRewardLog(l.Data)
			if err != nil {
				// Not a reward log; other protocols share the receipt stream.
				continue
			}
			for _, rl := range logs.Logs {
				if rl.Type == rewardingpb.RewardLog_DRAIN_ABANDONED {
					found = append(found, rl.Addr)
				}
			}
		}
	}
	return found, chunks, nil
}
