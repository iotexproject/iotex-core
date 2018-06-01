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
	// Prevent automatically transiting to PROPOSER after going to ROUND_START
	return event.State != stateRoundStart &&
		// Prevent the node that will become the proposer
		event.State != stateInitPropose &&
		// Ignore the proposer event from the proposer node itself to prevent taking another round of transition
		!(event.State == stateAcceptPropose && event.SenderAddr != nil && r.self.String() == event.SenderAddr.String())
}
