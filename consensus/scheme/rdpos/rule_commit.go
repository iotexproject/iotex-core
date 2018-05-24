// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/logger"
)

// ruleCommit commits the block based on 2k + 1 vote messages (K + 1 yes or no),,
// or commits an empty block if timeout or error occurred.
type ruleCommit struct {
	*RDPoS
}

func (r ruleCommit) Condition(event *fsm.Event) bool {
	if !event.StateTimedOut && event.Err == nil && !r.reachedMaj() {
		return false
	}

	// no consensus reached
	if event.StateTimedOut || event.Err != nil || !r.reachedMaj() {
		logger.Warn().
			Str("node", r.self.String()).
			Bool("state time out", event.StateTimedOut).
			Err(event.Err).
			Bool("r.reachedMaj()", r.reachedMaj()).
			Msg("|||||| no consensus agreed")
		return true
	}

	// consensus reached
	// TODO: Can roundCtx.block be nil as well? nil may also be a valid consensus result
	if r.roundCtx.block != nil {
		r.consCb(r.roundCtx.block)

		// only proposer needs to broadcast the consensus block
		if r.proposer {
			logger.Warn().
				Str("node", r.self.String()).
				Msg("|||||| broadcast block")
			r.pubCb(r.roundCtx.block)
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
	return agreed >= len(r.delegates)*2/3+1
}
