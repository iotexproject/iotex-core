// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/proto"
)

// ruleVote prevotes based on 2k + 1 prevote messages, or vote for an empty block if
// timeout or error occurred.
type ruleVote struct {
	*RollDPoS
}

func (r ruleVote) Condition(event *fsm.Event) bool {
	if !event.StateTimedOut && event.Err == nil && !r.reachedMaj() {
		return false
	}

	if event.StateTimedOut || event.Err != nil || !r.reachedMaj() {
		r.tellDelegates(
			&iproto.ViewChangeMsg{
				Vctype:    iproto.ViewChangeMsg_VOTE,
				BlockHash: nil,
			},
		)
		return true
	}

	// set self voted
	r.roundCtx.votes[r.self] = r.roundCtx.blockHash

	msg := &iproto.ViewChangeMsg{
		Vctype: iproto.ViewChangeMsg_VOTE,
	}
	if r.roundCtx.blockHash != nil {
		msg.BlockHash = r.roundCtx.blockHash[:]
	}
	r.tellDelegates(msg)
	return true
}

func (r ruleVote) reachedMaj() bool {
	agreed := 0
	for _, blkHash := range r.roundCtx.prevotes {
		if blkHash == nil && r.roundCtx.blockHash == nil ||
			(blkHash != nil && r.roundCtx.blockHash != nil && *r.roundCtx.blockHash == *blkHash) {
			agreed++
		}
	}
	return agreed >= len(r.delegates)*2/3+1
}
