// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/logger"
)

const (
	stateStart         fsm.State = "START"
	stateInitPropose   fsm.State = "INIT_PROPOSE"
	stateAcceptPropose fsm.State = "PROPOSE"
	stateAcceptPrevote fsm.State = "PREVOTE"
	stateAcceptVote    fsm.State = "VOTE"
)

func fsmCreate(r *RollDPoS) fsm.Machine {
	sm := fsm.NewMachine()

	if err := sm.SetInitialState(stateStart, &start{RollDPoS: r}); err != nil {
		logger.Error().Err(err).Msg("Error when creating fsm")
		return sm
	}
	sm.AddState(stateInitPropose, &initPropose{RollDPoS: r})
	sm.AddState(stateAcceptPropose, &acceptPropose{RollDPoS: r})
	sm.AddState(stateAcceptPrevote, &acceptPrevote{RollDPoS: r})
	sm.AddState(stateAcceptVote, &acceptVote{RollDPoS: r})

	sm.AddTransition(stateStart, stateInitPropose, &ruleIsProposer{RollDPoS: r})
	sm.AddTransition(stateStart, stateAcceptPropose, &ruleNotProposer{RollDPoS: r})
	sm.AddTransition(stateInitPropose, stateAcceptPrevote, &rulePropose{RollDPoS: r})
	sm.AddTransition(stateAcceptPropose, stateAcceptPrevote, &rulePrevote{RollDPoS: r})
	sm.AddTransition(stateAcceptPrevote, stateAcceptVote, &ruleVote{RollDPoS: r})
	sm.AddTransition(stateAcceptVote, stateStart, &ruleCommit{RollDPoS: r})

	return sm
}
