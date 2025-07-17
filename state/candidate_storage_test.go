package state

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestCandidateListStorageAndLoad(t *testing.T) {
	require := require.New(t)

	// Create test candidate list
	originalList := CandidateList{
		{
			Address:       identityset.Address(1).String(),
			Votes:         big.NewInt(1000),
			RewardAddress: identityset.Address(2).String(),
			CanName:       []byte("candidate1"),
		},
		{
			Address:       identityset.Address(2).String(),
			Votes:         big.NewInt(2000),
			RewardAddress: identityset.Address(3).String(),
			CanName:       []byte("candidate2"),
		},
		{
			Address:       identityset.Address(3).String(),
			Votes:         big.NewInt(3000),
			RewardAddress: "",  // empty reward address
			CanName:       nil, // empty name
		},
	}

	ns := "test"
	key := []byte("candidates")

	// Test Storage
	store := originalList.Storage(ns, key)
	require.NotEmpty(store)

	// Test Load
	var loadedList CandidateList
	err := loadedList.Load(ns, key, store)
	require.NoError(err)

	// Verify the loaded list matches the original
	require.Equal(len(originalList), len(loadedList))

	for i, original := range originalList {
		loaded := loadedList[i]
		require.Equal(original.Address, loaded.Address)
		require.Equal(original.Votes.Cmp(loaded.Votes), 0)
		require.Equal(original.RewardAddress, loaded.RewardAddress)
		require.Equal(original.CanName, loaded.CanName)
	}
}

func TestCandidateListStorageAndLoadEmpty(t *testing.T) {
	require := require.New(t)

	// Test with empty list
	originalList := CandidateList{}
	ns := "test"
	key := []byte("candidates")

	// Test Storage
	store := originalList.Storage(ns, key)
	require.NotEmpty(store) // Should at least contain count

	// Test Load
	var loadedList CandidateList
	err := loadedList.Load(ns, key, store)
	require.NoError(err)
	require.Empty(loadedList)
}

func TestCandidateListLoadFromEmptyStore(t *testing.T) {
	require := require.New(t)

	// Test Load from empty store
	var loadedList CandidateList
	emptyStore := make(map[hash.Hash256]uint256.Int)

	err := loadedList.Load("test", []byte("candidates"), emptyStore)
	require.NoError(err)
	require.Empty(loadedList)
}
