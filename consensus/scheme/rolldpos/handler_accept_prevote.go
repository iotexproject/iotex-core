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

// acceptPrevote waits for 2k prevote messages for the same block from others or timeout.
type acceptPrevote struct {
	*RollDPoS
}

func (h *acceptPrevote) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptPrevote.TTL
}

func (h *acceptPrevote) Handle(event *fsm.Event) {
	h.roundCtx.prevotes[event.SenderAddr] = event.BlockHash
}
