// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/consensus/fsm"
)

func TestRuleRoundStart_Condition(t *testing.T) {
	t.Parallel()

	rs := ruleRoundStart{
		RollDPoS: &RollDPoS{
			eventChan: make(chan *fsm.Event, 1),
		},
	}

	require.True(t, rs.Condition(&fsm.Event{State: stateRoundStart}))
	require.False(t, rs.Condition(&fsm.Event{State: stateEpochStart}))

	// Trigger going back to epoch start if timeout on round start
	evt := &fsm.Event{State: stateRoundStart, StateTimedOut: true}
	require.True(t, rs.Condition(evt))
	require.Equal(t, &fsm.Event{State: stateEpochStart}, <-rs.RollDPoS.eventChan)
}
