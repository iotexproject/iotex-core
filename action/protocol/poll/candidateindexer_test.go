// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestCandidateIndexer(t *testing.T) {
	require := require.New(t)
	indexer, err := NewCandidateIndexer(db.NewMemKVStore())
	require.NoError(err)
	require.NoError(indexer.Start(context.Background()))
	// PutCandidates and Candidates with height 1
	candidates := state.CandidateList{
		{
			Address:       identityset.Address(1).String(),
			Votes:         big.NewInt(30),
			RewardAddress: "rewardAddress1",
		},
		{
			Address:       identityset.Address(2).String(),
			Votes:         big.NewInt(22),
			RewardAddress: "rewardAddress2",
		},
		{
			Address:       identityset.Address(3).String(),
			Votes:         big.NewInt(20),
			RewardAddress: "rewardAddress3",
		},
		{
			Address:       identityset.Address(4).String(),
			Votes:         big.NewInt(10),
			RewardAddress: "rewardAddress4",
		},
	}
	require.NoError(indexer.PutCandidateList(uint64(1), &candidates))
	candidatesFromDB, err := indexer.CandidateList(uint64(1))
	require.NoError(err)
	require.Equal(len(candidatesFromDB), len(candidates))
	for i, cand := range candidates {
		require.True(cand.Equal(candidatesFromDB[i]))
	}

	// try to put again
	require.NoError(indexer.PutCandidateList(uint64(1), &candidates))
	candidatesFromDB, err = indexer.CandidateList(uint64(1))
	require.NoError(err)
	require.Equal(len(candidatesFromDB), len(candidates))
	for i, cand := range candidates {
		require.True(cand.Equal(candidatesFromDB[i]))
	}

	// PutCandidates and Candidates with height 2
	candidates2 := state.CandidateList{
		{
			Address:       identityset.Address(1).String(),
			Votes:         big.NewInt(30),
			RewardAddress: "rewardAddress1",
		},
		{
			Address:       identityset.Address(2).String(),
			Votes:         big.NewInt(22),
			RewardAddress: "rewardAddress2",
		},
	}
	require.NoError(indexer.PutCandidateList(uint64(2), &candidates2))
	candidatesFromDB, err = indexer.CandidateList(uint64(2))
	require.NoError(err)
	require.Equal(len(candidatesFromDB), len(candidates2))
	for i, cand := range candidates2 {
		require.True(cand.Equal(candidatesFromDB[i]))
	}

	// PutKickoutList and KickoutList with height 1
	blackListMap := map[string]uint32{
		identityset.Address(1).String(): 1,
		identityset.Address(2).String(): 1,
	}

	blackList := &vote.Blacklist{
		BlacklistInfos: blackListMap,
		IntensityRate:  50,
	}
	require.NoError(indexer.PutKickoutList(uint64(1), blackList))
	blackList2, err := indexer.KickoutList(uint64(1))
	require.NoError(err)

	require.Equal(blackList.IntensityRate, blackList2.IntensityRate)
	require.Equal(len(blackList.BlacklistInfos), len(blackList.BlacklistInfos))
	for str, count := range blackList.BlacklistInfos {
		require.Equal(blackList2.BlacklistInfos[str], count)
	}
}
