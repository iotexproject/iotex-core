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

// initPropose proposes a new block and send it out.
type initPropose struct {
	fsm.NilTimeout
	*RollDPoS
}

// TimeoutDuration returns the duration for timeout
func (h *initPropose) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptPropose.TTL
}

func (h *initPropose) Handle(event *fsm.Event) {
	blk, err := h.propCb()
	if err != nil {
		event.Err = err
		return
	}

	h.roundCtx.block = blk
}
