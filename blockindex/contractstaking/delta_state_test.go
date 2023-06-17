package contractstaking

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeltaState_Transfer(t *testing.T) {
	require := require.New(t)

	cases := []struct {
		name     string
		state    deltaState
		action   deltaAction
		expected deltaState
		err      string
	}{
		{"unchanged->add", deltaStateUnchanged, deltaActionAdd, deltaStateAdded, ""},
		{"unchanged->remove", deltaStateUnchanged, deltaActionRemove, deltaStateRemoved, ""},
		{"unchanged->modify", deltaStateUnchanged, deltaActionModify, deltaStateModified, ""},
		{"added->add", deltaStateAdded, deltaActionAdd, deltaStateUnchanged, "invalid delta action 0 on state 1"},
		{"added->remove", deltaStateAdded, deltaActionRemove, deltaStateUnchanged, "invalid delta action 1 on state 1"},
		{"added->modify", deltaStateAdded, deltaActionModify, deltaStateAdded, ""},
		{"removed->add", deltaStateRemoved, deltaActionAdd, deltaStateUnchanged, "invalid delta state 2"},
		{"removed->remove", deltaStateRemoved, deltaActionRemove, deltaStateUnchanged, "invalid delta state 2"},
		{"removed->modify", deltaStateRemoved, deltaActionModify, deltaStateUnchanged, "invalid delta state 2"},
		{"modified->add", deltaStateModified, deltaActionAdd, deltaStateUnchanged, "invalid delta action 0 on state 3"},
		{"modified->remove", deltaStateModified, deltaActionRemove, deltaStateRemoved, ""},
		{"modified->modify", deltaStateModified, deltaActionModify, deltaStateModified, ""},
		{"invalid state", deltaState(100), deltaActionAdd, deltaState(100), "invalid delta state 100"},
		{"invalid action", deltaStateUnchanged, deltaAction(100), deltaStateUnchanged, "invalid delta action 100 on state 0"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := c.state.Transfer(c.action)
			if len(c.err) > 0 {
				require.Error(err)
				require.Contains(err.Error(), c.err)
			} else {
				require.NoError(err)
				require.Equal(c.expected, s)
			}
		})
	}
}

func TestDeltaState_ZeroValue(t *testing.T) {
	require := require.New(t)

	var state deltaState
	require.Equal(deltaStateUnchanged, state)
}
