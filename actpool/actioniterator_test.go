package actpool

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/action"
	"github.com/stretchr/testify/require"
)

func TestActionIterator(t *testing.T) {
	require := require.New(t)
	accMap := make(map[string][]action.Action)
	vote1, err := action.NewVote(1, "1", "2", 0, big.NewInt(13))
	require.Nil(err)
	vote2, err := action.NewVote(2, "1", "2", 0, big.NewInt(30))
	require.Nil(err)
	accMap[vote1.SrcAddr()] = []action.Action{vote1, vote2}

	tsf1, err := action.NewTransfer(uint64(1), big.NewInt(100), "2", "3", nil, uint64(0), big.NewInt(15))
	require.NoError(err)
	tsf2, err := action.NewTransfer(uint64(2), big.NewInt(100), "2", "3", nil, uint64(0), big.NewInt(10))
	require.NoError(err)
	vote3, err := action.NewVote(3, "2", "3", 0, big.NewInt(20))
	require.NoError(err)
	accMap[tsf1.SrcAddr()] = []action.Action{tsf1, tsf2, vote3}

	tsf3, err := action.NewTransfer(uint64(1), big.NewInt(100), "3", "1", nil, uint64(0), big.NewInt(5))
	require.NoError(err)
	accMap[tsf3.SrcAddr()] = []action.Action{tsf3}

	ai := NewActionIterator(accMap)
	act := ai.TopAction()
	require.Equal(act, tsf1)
	ai.LoadNextAction()
	act = ai.TopAction()
	require.Equal(act, vote1)
	ai.PopAction()
	act = ai.TopAction()
	require.Equal(act, tsf2)
}
