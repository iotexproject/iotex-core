// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import "github.com/iotexproject/iotex-core/consensus/fsm"

const (
	stateStart         fsm.State = "START"
	stateInitPropose   fsm.State = "INIT_PROPOSE"
	stateAcceptPropose fsm.State = "PROPOSE"
	stateAcceptPrevote fsm.State = "PREVOTE"
	stateAcceptVote    fsm.State = "VOTE"
)

func fsmCreate(r *RDPoS) fsm.Machine {
	sm := fsm.NewMachine()

	sm.SetInitialState(stateStart, &start{RDPoS: r})
	sm.AddState(stateInitPropose, &initPropose{RDPoS: r})
	sm.AddState(stateAcceptPropose, &acceptPropose{RDPoS: r})
	sm.AddState(stateAcceptPrevote, &acceptPrevote{RDPoS: r})
	sm.AddState(stateAcceptVote, &acceptVote{RDPoS: r})

	sm.AddTransition(stateStart, stateInitPropose, &ruleIsProposer{RDPoS: r})
	sm.AddTransition(stateStart, stateAcceptPropose, &ruleNotProposer{RDPoS: r})
	sm.AddTransition(stateInitPropose, stateAcceptPrevote, &rulePropose{RDPoS: r})
	sm.AddTransition(stateAcceptPropose, stateAcceptPrevote, &rulePrevote{RDPoS: r})
	sm.AddTransition(stateAcceptPrevote, stateAcceptVote, &ruleVote{RDPoS: r})
	sm.AddTransition(stateAcceptVote, stateStart, &ruleCommit{RDPoS: r})

	return sm
}
