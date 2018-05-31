// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/iotex-core/consensus/fsm"
)

// ruleNotProposer checks if the event is not init propose.
type ruleNotProposer struct {
	*RollDPoS
}

func (r ruleNotProposer) Condition(event *fsm.Event) bool {
	// Proposer itself will be removed to START after committing the block. When receiving PROPOSE event at this moment,
	// we should just ignore it to prevent taking another round of transition.
	return (r.roundCtx == nil || !r.roundCtx.isPr) && event.State != stateInitPropose
}
