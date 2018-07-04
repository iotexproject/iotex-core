// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
)

// rulePrevote sends out prevote messages.
type rulePrevote struct {
	*RollDPoS
}

func (r rulePrevote) Condition(event *fsm.Event) bool {
	if event.StateTimedOut || event.Err != nil {
		r.tellDelegates(
			&iproto.ViewChangeMsg{
				Vctype:    iproto.ViewChangeMsg_PREVOTE,
				BlockHash: nil,
			},
		)
		return true
	}

	// set self prevoted
	r.roundCtx.block = event.Block
	var blkHash *hash.Hash32B
	if event.Block != nil {
		bkh := event.Block.HashBlock()
		blkHash = &bkh
	} else {
		blkHash = nil
	}

	r.roundCtx.prevotes[r.self] = blkHash
	r.roundCtx.blockHash = blkHash

	msg := &iproto.ViewChangeMsg{
		Vctype: iproto.ViewChangeMsg_PREVOTE,
	}
	if blkHash != nil {
		msg.BlockHash = blkHash[:]
	}
	r.tellDelegates(msg)
	return true
}
