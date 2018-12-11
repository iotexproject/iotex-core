package actioniterator

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/action"
	"github.com/stretchr/testify/require"
)

type actionValidator struct {
	actionCount  uint64
	maxAction    uint64
	errorActions []action.Action
}

// Next load next action of account of top action
func (av *actionValidator) Validate(bestAction action.Action) error {
	if av.actionCount >= av.maxAction {
		return action.ErrOutOfGas
	}

	av.actionCount++
	for _, headAction := range av.errorActions {
		if headAction == bestAction {
			return action.ErrInsufficientBalanceForGas
		}
	}
	return nil
}

func TestActionIterator(t *testing.T) {
	require := require.New(t)
	vote1, err := action.NewVote(1, "1", "2", 0, big.NewInt(13))
	require.Nil(err)
	vote2, err := action.NewVote(2, "1", "2", 0, big.NewInt(30))
	require.Nil(err)

	tsf1, err := action.NewTransfer(uint64(1), big.NewInt(100), "2", "3", nil, uint64(0), big.NewInt(15))
	require.NoError(err)
	tsf2, err := action.NewTransfer(uint64(2), big.NewInt(100), "2", "3", nil, uint64(0), big.NewInt(10))
	require.NoError(err)
	vote3, err := action.NewVote(3, "2", "3", 0, big.NewInt(20))
	require.NoError(err)

	tsf3, err := action.NewTransfer(uint64(1), big.NewInt(100), "3", "1", nil, uint64(0), big.NewInt(5))
	require.NoError(err)
	accMap1 := make(map[string][]action.Action)
	accMap1[vote1.SrcAddr()] = []action.Action{vote1, vote2}
	accMap1[tsf1.SrcAddr()] = []action.Action{tsf1, tsf2, vote3}
	accMap1[tsf3.SrcAddr()] = []action.Action{tsf3}

	ai := NewActionIterator(accMap1)
	appliedActionList := make([]action.Action, 0)
	for {
		bestAction := ai.Next()
		if bestAction == nil {
			break
		}
		appliedActionList = append(appliedActionList, bestAction)
	}
	require.Equal(appliedActionList, []action.Action{tsf1, vote1, vote2, tsf2, vote3, tsf3})
}
