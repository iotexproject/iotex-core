// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

const (
	stateDBPath = "stateDB.test"
)

func TestBucket(t *testing.T) {
	require := require.New(t)

	vb, err := NewVoteBucket("", "d390*jk jh{}", "a2100000000", 21, time.Now(), true)
	require.Equal("empty candidate name", err.Error())
	vb, err = NewVoteBucket("testname", "d390*jk jh{}", "a2100000000", 21, time.Now(), true)
	require.Equal("failed to cast amount", err.Error())
	vb, err = NewVoteBucket("testname", "d390*jk jh{}", "2100000000", 21, time.Now(), true)
	require.Error(err)
	vb, err = NewVoteBucket("testname", "io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "2100000000", 21, time.Now(), true)
	require.NoError(err)

	data, err := vb.Serialize()
	require.NoError(err)
	vb1 := VoteBucket{}
	require.NoError(vb1.Deserialize(data))
	require.Equal(vb.CandidateName, vb1.CandidateName)
	require.Equal(vb.StakedAmount, vb1.StakedAmount)
	require.Equal(vb.StakedDuration, vb1.StakedDuration)
	require.Equal(vb.CreateTime.Seconds, vb1.CreateTime.Seconds)
	require.Equal(vb.CreateTime.Nanos, vb1.CreateTime.Nanos)
	require.Equal(vb.StakeStartTime.Seconds, vb1.StakeStartTime.Seconds)
	require.Equal(vb.StakeStartTime.Nanos, vb1.StakeStartTime.Nanos)
	require.Equal(vb.UnstakeStartTime.Seconds, vb1.UnstakeStartTime.Seconds)
	require.Equal(vb.UnstakeStartTime.Nanos, vb1.UnstakeStartTime.Nanos)
	require.Equal(vb.NonDecay, vb1.NonDecay)
	require.Equal(vb.Owner, vb1.Owner)
}

func TestGetPutStaking(t *testing.T) {
	testGetPut := func(sf factory.Factory, t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		require.NoError(sf.Start(ctx))
		defer func() {
			require.NoError(sf.Stop(ctx))
		}()

		tests := []struct {
			name  CandName
			index uint64
		}{
			{
				CandName{1, 2, 3, 4},
				0,
			},
			{
				CandName{1, 2, 3, 4},
				1,
			},
			{
				CandName{2, 3, 4, 5},
				2,
			},
			{
				CandName{2, 3, 4, 5},
				3,
			},
		}

		// put buckets and get
		for _, e := range tests {
			_, err := stakingGetBucket(sf, e.name, e.index)
			require.Equal(state.ErrStateNotExist, errors.Cause(err))

			vb, err := NewVoteBucket("testname", "io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "2100000000", 21, time.Now(), true)
			require.NoError(err)
			ws, err := sf.NewWorkingSet()
			require.NoError(err)
			require.NotNil(ws)

			count, err := stakingGetTotalCount(sf)
			require.NoError(err)
			require.Equal(e.index, count)
			require.NoError(stakingPutBucket(ws, e.name, vb))
			count, err = stakingGetTotalCount(ws)
			require.NoError(err)
			require.Equal(e.index+1, count)
			require.NoError(ws.Finalize())
			require.NoError(ws.Commit())
			count, err = stakingGetTotalCount(sf)
			require.NoError(err)
			require.Equal(e.index+1, count)

			vb1, err := stakingGetBucket(sf, e.name, e.index)
			require.NoError(err)
			require.Equal(vb.CandidateName, vb1.CandidateName)
			require.Equal(vb.StakedAmount, vb1.StakedAmount)
			require.Equal(vb.StakedDuration, vb1.StakedDuration)
			require.Equal(vb.CreateTime.Seconds, vb1.CreateTime.Seconds)
			require.Equal(vb.CreateTime.Nanos, vb1.CreateTime.Nanos)
			require.Equal(vb.StakeStartTime.Seconds, vb1.StakeStartTime.Seconds)
			require.Equal(vb.StakeStartTime.Nanos, vb1.StakeStartTime.Nanos)
			require.Equal(vb.UnstakeStartTime.Seconds, vb1.UnstakeStartTime.Seconds)
			require.Equal(vb.UnstakeStartTime.Nanos, vb1.UnstakeStartTime.Nanos)
			require.Equal(vb.NonDecay, vb1.NonDecay)
			require.Equal(vb.Owner, vb1.Owner)
		}

		// delete buckets and get
		ws, err := sf.NewWorkingSet()
		require.NoError(err)
		require.NotNil(ws)
		for _, e := range tests {
			require.NoError(stakingDelBucket(ws, e.name, e.index))
		}
		require.NoError(ws.Finalize())
		require.NoError(ws.Commit())

		for _, e := range tests {
			_, err = stakingGetBucket(sf, e.name, e.index)
			require.Equal(state.ErrStateNotExist, errors.Cause(err))
		}
	}

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testStateDBPath := testTrieFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := factory.NewStateDB(cfg, factory.DefaultStateDBOption())
	require.NoError(t, err)

	t.Run("test stateDB staking API", func(t *testing.T) {
		testGetPut(sdb, t)
	})
}
