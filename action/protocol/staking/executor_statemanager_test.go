// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
	"github.com/stretchr/testify/require"
)

func Test_newExecutorStateManager(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	msm := testdb.NewMockStateManager(ctrl)

	t.Run("load executor from state db", func(t *testing.T) {
		cases := []*Executor{
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(2),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 1,
				Amount:    big.NewInt(100),
			},
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(3),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 2,
				Amount:    big.NewInt(100),
			},
		}
		for _, e := range cases {
			_, err := msm.PutState(e, protocol.NamespaceOption(_executorStateConfig.executorNamespace), protocol.KeyOption(_executorStateConfig.executorKeygenFunc(e)))
			require.NoError(err)
		}
		esm, err := newExecutorStateManager(msm)
		require.NoError(err)
		for _, expect := range cases {
			eMap, ok := esm.executors[expect.Type]
			require.True(ok)
			e, ok := eMap[expect.Operator.String()]
			require.True(ok)
			equalExecutor(require, expect, e)
		}
	})

	t.Run("load executor from protocol view", func(t *testing.T) {
		cases := []*Executor{
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(2),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 1,
				Amount:    big.NewInt(100),
			},
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(3),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 2,
				Amount:    big.NewInt(100),
			},
		}
		esm, err := newExecutorStateManager(msm)
		require.NoError(err)
		for _, e := range cases {
			require.NoError(esm.putExecutor(e))
		}
		require.NoError(writeView(newCandidateStateManager(msm), esm))
		esm, err = newExecutorStateManager(msm)
		require.NoError(err)

		for _, expect := range cases {
			eMap, ok := esm.executors[expect.Type]
			require.True(ok)
			e, ok := eMap[expect.Operator.String()]
			require.True(ok)
			equalExecutor(require, expect, e)
		}
	})
}

func Test_putExecutor(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	t.Run("put in memory and state db", func(t *testing.T) {
		msm := testdb.NewMockStateManager(ctrl)
		executors := []*Executor{
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(2),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 1,
				Amount:    big.NewInt(100),
			},
			{
				Owner:     identityset.Address(4),
				Operator:  identityset.Address(5),
				Reward:    identityset.Address(6),
				Type:      ExecutorTypeProposer,
				BucketIdx: 2,
				Amount:    big.NewInt(100),
			},
		}
		esm, err := newExecutorStateManager(msm)
		require.NoError(err)
		for _, e := range executors {
			require.NoError(esm.putExecutor(e))
		}
		esmExecutorCheck(require, esm, executors)
	})

	t.Run("owner have multiple executor", func(t *testing.T) {
		msm := testdb.NewMockStateManager(ctrl)
		executors := []*Executor{
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(2),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 1,
				Amount:    big.NewInt(100),
			},
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(4),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 2,
				Amount:    big.NewInt(100),
			},
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(5),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 3,
				Amount:    big.NewInt(100),
			},
		}
		esm, err := newExecutorStateManager(msm)
		require.NoError(err)
		for _, e := range executors {
			require.NoError(esm.putExecutor(e))
		}
		esmExecutorCheck(require, esm, executors)
	})

	t.Run("operator has only one executor", func(t *testing.T) {
		msm := testdb.NewMockStateManager(ctrl)
		executors := []*Executor{
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(2),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 1,
				Amount:    big.NewInt(100),
			},
			{
				Owner:     identityset.Address(1),
				Operator:  identityset.Address(2),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 2,
				Amount:    big.NewInt(100),
			},
			{
				Owner:     identityset.Address(3),
				Operator:  identityset.Address(2),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 3,
				Amount:    big.NewInt(100),
			},
		}
		esm, err := newExecutorStateManager(msm)
		require.NoError(err)
		for _, e := range executors {
			require.NoError(esm.putExecutor(e))
		}
		esmExecutorCheck(require, esm, []*Executor{
			{
				Owner:     identityset.Address(3),
				Operator:  identityset.Address(2),
				Reward:    identityset.Address(3),
				Type:      ExecutorTypeProposer,
				BucketIdx: 3,
				Amount:    big.NewInt(100),
			},
		})
	})
}

func Test_putBucket(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	t.Run("put state db", func(t *testing.T) {
		msm := testdb.NewMockStateManager(ctrl)
		buckets := []*VoteBucket{
			NewVoteBucket(
				identityset.Address(1),
				identityset.Address(2),
				big.NewInt(100),
				0,
				time.Now(),
				true,
			),
			NewVoteBucket(
				identityset.Address(3),
				identityset.Address(4),
				big.NewInt(100),
				0,
				time.Now(),
				true,
			),
		}
		esm, err := newExecutorStateManager(msm)
		require.NoError(err)
		for i, b := range buckets {
			idx, err := esm.putBucket(b)
			require.NoError(err)
			require.Equal(uint64(i), idx)
		}
		esmBucketCheck(require, esm, buckets)
	})
}

func Test_view(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	msm := testdb.NewMockStateManager(ctrl)
	executors := []*Executor{
		{
			Owner:     identityset.Address(1),
			Operator:  identityset.Address(2),
			Reward:    identityset.Address(3),
			Type:      ExecutorTypeProposer,
			BucketIdx: 1,
			Amount:    big.NewInt(100),
		},
		{
			Owner:     identityset.Address(4),
			Operator:  identityset.Address(5),
			Reward:    identityset.Address(6),
			Type:      ExecutorTypeProposer,
			BucketIdx: 2,
			Amount:    big.NewInt(100),
		},
	}
	esm, err := newExecutorStateManager(msm)
	require.NoError(err)
	for _, e := range executors {
		require.NoError(esm.putExecutor(e))
	}
	view := esm.view()
	for _, e := range executors {
		eMap, ok := view.executors[e.Type]
		require.True(ok)
		e2, ok := eMap[e.Operator.String()]
		require.True(ok)
		equalExecutor(require, e, e2)
	}
}

func equalExecutor(require *require.Assertions, expect, actual *Executor) {
	require.Equal(expect.Owner.String(), actual.Owner.String())
	require.Equal(expect.Operator.String(), actual.Operator.String())
	require.Equal(expect.Reward.String(), actual.Reward.String())
	require.Equal(expect.Type, actual.Type)
	require.Equal(expect.BucketIdx, actual.BucketIdx)
	require.Equal(expect.Amount.Cmp(actual.Amount), 0)
}

func esmExecutorCheck(require *require.Assertions, esm *executorStateManager, expects []*Executor) {
	for _, expect := range expects {
		eMap, ok := esm.executors[expect.Type]
		require.True(ok)
		e, ok := eMap[expect.Operator.String()]
		require.True(ok)
		equalExecutor(require, expect, e)
	}
	count := 0
	for _, eMap := range esm.executors {
		count += len(eMap)
	}
	require.Equal(len(expects), count)
	_, iter, err := esm.States(protocol.NamespaceOption(esm.config.executorNamespace))
	require.NoError(err)
	e := &Executor{}
	require.Equal(len(expects), iter.Size())
	for err := iter.Next(e); err == nil; err = iter.Next(e) {
		eMap, ok := esm.executors[e.Type]
		require.True(ok)
		expect, ok := eMap[e.Operator.String()]
		require.True(ok)
		equalExecutor(require, expect, e)
	}
}

func esmBucketCheck(require *require.Assertions, esm *executorStateManager, expects []*VoteBucket) {
	var tc totalBucketCount
	_, err := esm.State(
		&tc,
		protocol.NamespaceOption(esm.config.bucketNamespace),
		protocol.KeyOption(esm.config.totalBucketKey))
	require.NoError(err)
	require.Equal(len(expects), int(tc.Count()))

	bucket := &VoteBucket{}
	for i := range expects {
		_, err := esm.State(bucket, protocol.NamespaceOption(esm.config.bucketNamespace), protocol.KeyOption(esm.config.bucketKeygenFunc(expects[i])))
		require.NoError(err)
		equalBucket(require, expects[i], bucket)
	}
}

func equalBucket(require *require.Assertions, expect, actual *VoteBucket) {
	require.Equal(expect.Index, actual.Index)
	require.Equal(expect.Candidate.String(), actual.Candidate.String())
	require.Equal(expect.Owner.String(), actual.Owner.String())
	require.Equal(expect.StakedAmount.Cmp(actual.StakedAmount), 0)
	require.Equal(expect.StakedDuration, actual.StakedDuration)
	require.Equal(expect.StakeStartTime, actual.StakeStartTime)
	require.Equal(expect.CreateTime, actual.CreateTime)
	require.Equal(expect.UnstakeStartTime, actual.UnstakeStartTime)
	require.Equal(expect.AutoStake, actual.AutoStake)
}
