package state

import (
	"math/big"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCandidate(t *testing.T) {
	require := require.New(t)

	cand1 := &Candidate{
		Address: "Alpha",
		Votes:   big.NewInt(1),
	}
	cand2 := &Candidate{
		Address: "Beta",
		Votes:   big.NewInt(2),
	}
	cand3 := &Candidate{
		Address: "Theta",
		Votes:   big.NewInt(3),
	}

	candidateMap := make(map[string]*Candidate)
	candidateMap["Alpha"] = cand1
	candidateMap["Beta"] = cand2
	candidateMap["Theta"] = cand3

	candidateList, err := MapToCandidates(candidateMap)
	require.NoError(err)
	require.Equal(3, len(candidateList))
	sort.Sort(candidateList)

	require.Equal("Theta", candidateList[0].Address)
	require.Equal("Beta", candidateList[1].Address)
	require.Equal("Alpha", candidateList[2].Address)

	candidatesBytes, err := Serialize(candidateList)
	require.NoError(err)
	candidates, err := Deserialize(candidatesBytes)
	require.NoError(err)
	require.Equal(3, len(candidates))
	require.Equal("Theta", candidates[0].Address)
	require.Equal("Beta", candidates[1].Address)
	require.Equal("Alpha", candidates[2].Address)
}
