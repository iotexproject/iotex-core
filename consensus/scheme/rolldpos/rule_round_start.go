// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import "github.com/iotexproject/iotex-core/consensus/fsm"

type ruleRoundStart struct {
	*RollDPoS
}

func (r ruleRoundStart) Condition(event *fsm.Event) bool {
	// If the consensus is timeout at ROUND_START, goes back to EPOCH_START
	if event.State == stateRoundStart && event.StateTimedOut {
		r.enqueueEvent(&fsm.Event{
			State: stateEpochStart,
		})
	}
	return event.State == stateRoundStart
}
