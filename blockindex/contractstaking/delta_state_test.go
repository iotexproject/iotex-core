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
		isError  bool
	}{
		{"unchanged->add", deltaStateUnchanged, deltaActionAdd, deltaStateAdded, false},
		{"unchanged->remove", deltaStateUnchanged, deltaActionRemove, deltaStateRemoved, false},
		{"unchanged->modify", deltaStateUnchanged, deltaActionModify, deltaStateModified, false},
		{"added->add", deltaStateAdded, deltaActionAdd, deltaStateUnchanged, true},
		{"added->remove", deltaStateAdded, deltaActionRemove, deltaStateUnchanged, true},
		{"added->modify", deltaStateAdded, deltaActionModify, deltaStateAdded, false},
		{"removed->add", deltaStateRemoved, deltaActionAdd, deltaStateUnchanged, true},
		{"removed->remove", deltaStateRemoved, deltaActionRemove, deltaStateUnchanged, true},
		{"removed->modify", deltaStateRemoved, deltaActionModify, deltaStateUnchanged, true},
		{"modified->add", deltaStateModified, deltaActionAdd, deltaStateUnchanged, true},
		{"modified->remove", deltaStateModified, deltaActionRemove, deltaStateRemoved, false},
		{"modified->modify", deltaStateModified, deltaActionModify, deltaStateModified, false},
		{"invalid state", deltaState(100), deltaActionAdd, deltaState(100), true},
		{"invalid action", deltaStateUnchanged, deltaAction(100), deltaStateUnchanged, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := c.state.Transfer(c.action)
			if c.isError {
				require.Error(err)
			} else {
				require.NoError(err)
				require.Equal(c.expected, s)
			}
		})
	}
}
