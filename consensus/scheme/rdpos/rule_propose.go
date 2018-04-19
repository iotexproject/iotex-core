// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/proto"
)

// rulePropose sends out propose messages.
type rulePropose struct {
	*RDPoS
}

func (r rulePropose) Condition(event *fsm.Event) bool {
	if event.StateTimedOut || event.Err != nil || r.roundCtx.block == nil {
		event.Block = nil
		r.tellDelegates(
			&iproto.ViewChangeMsg{
				Vctype: iproto.ViewChangeMsg_PROPOSE,
				Block:  nil,
			},
		)
		return true
	}

	bkh := r.roundCtx.block.HashBlock()
	blkHash := &bkh
	msg := &iproto.ViewChangeMsg{
		Vctype:    iproto.ViewChangeMsg_PROPOSE,
		Block:     r.roundCtx.block.ConvertToBlockPb(),
		BlockHash: blkHash[:],
	}

	// tell other validators of the block
	r.tellDelegates(msg)

	// set self prevoted
	r.roundCtx.prevotes[r.self] = blkHash
	r.roundCtx.blockHash = blkHash

	return true
}
