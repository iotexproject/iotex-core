// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"net"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/consensus/fsm"
)

// start is the initial and idle state of all consensus states. It initiates
// the round context.
type start struct {
	fsm.NilTimeout
	*RDPoS
}

func (h *start) Handle(event *fsm.Event) {
	h.roundCtx = &roundCtx{
		prevotes: make(map[net.Addr]*common.Hash32B),
		votes:    make(map[net.Addr]*common.Hash32B),
	}
}
