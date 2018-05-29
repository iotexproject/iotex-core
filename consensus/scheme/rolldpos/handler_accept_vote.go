// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"time"

	"github.com/iotexproject/iotex-core/consensus/fsm"
)

// acceptVote waits for 2k vote messages from others or timeout.
type acceptVote struct {
	*RollDPoS
}

func (h *acceptVote) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptVote.TTL
}

func (h *acceptVote) Handle(event *fsm.Event) {
	h.roundCtx.votes[event.SenderAddr] = event.BlockHash
}
