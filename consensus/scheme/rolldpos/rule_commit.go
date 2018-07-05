// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/logger"
)

// ruleCommit commits the block based on 2k + 1 vote messages (K + 1 yes or no),,
// or commits an empty block if timeout or error occurred.
type ruleCommit struct {
	*RollDPoS
}

func (r ruleCommit) Condition(event *fsm.Event) bool {
	if !event.StateTimedOut && event.Err == nil && !r.reachedMaj() {
		return false
	}

	// no matter consensus is reached or is confirmed not to reach, always finish the current consensus round
	defer r.notifyRoundFinish()

	// no consensus reached
	if event.StateTimedOut || event.Err != nil || !r.reachedMaj() {
		var height uint64
		if r.roundCtx.block != nil {
			height = r.roundCtx.block.Height()
		}
		logger.Warn().
			Str("node", r.self.String()).
			Bool("timeout", event.StateTimedOut).
			Bool("majority", r.reachedMaj()).
			Err(event.Err).
			Uint64("block", height).
			Msg("no consensus reached")

		// TODO: need to commit and broadcast empty block to make proposer and block height map consistently
		return true
	}

	// TODO: Can roundCtx.block be nil as well? nil may also be a valid consensus result
	if r.roundCtx.block != nil {
		dlgs := make([]string, 0)
		for _, d := range r.epochCtx.delegates {
			dlgs = append(dlgs, d.String())
		}
		logger.Info().
			Strs("delegates", dlgs).
			Uint64("block", r.roundCtx.block.Height()).
			Msg("consensus reached")
		if err := r.consCb(r.roundCtx.block); err != nil {
			logger.Error().
				Str("name", r.self.String()).
				Uint64("block", r.roundCtx.block.Height()).
				Msg("error when committing a block")
			event.Err = err
			return true
		}

		// All delegates need to broadcast the consensus block
		if err := r.pubCb(r.roundCtx.block); err != nil {
			logger.Error().
				Str("name", r.self.String()).
				Uint64("block", r.roundCtx.block.Height()).
				Msg("error when committing a block")
			event.Err = err
			return true
		}
	}
	return true
}

func (r ruleCommit) reachedMaj() bool {
	agreed := 0
	for _, blkHash := range r.roundCtx.votes {
		if blkHash == nil && r.roundCtx.blockHash == nil ||
			(blkHash != nil && r.roundCtx.blockHash != nil && *r.roundCtx.blockHash == *blkHash) {
			agreed++
		}
	}
	return agreed >= len(r.epochCtx.delegates)*2/3+1
}

func (r ruleCommit) notifyRoundFinish() {
	// Fist emit an event to test if FSM should move back to epoch start
	// If not, FSM will stay at round start, and the second emitted event for proposer will move it to init propose
	r.enqueueEvent(&fsm.Event{
		State: stateEpochStart,
	})
	if r.cfg.ProposerInterval == 0 {
		r.prnd.Handle()
	}
}
