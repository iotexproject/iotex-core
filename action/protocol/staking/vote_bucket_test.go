// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

const (
	_stateDBPath = "stateDB.test"
)

func TestGetPutStaking(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)

	tests := []struct {
		name  hash.Hash160
		index uint64
	}{
		{
			hash.BytesToHash160([]byte{1, 2, 3, 4}),
			0,
		},
		{
			hash.BytesToHash160([]byte{1, 2, 3, 4}),
			1,
		},
		{
			hash.BytesToHash160([]byte{2, 3, 4, 5}),
			2,
		},
		{
			hash.BytesToHash160([]byte{2, 3, 4, 5}),
			3,
		},
	}

	// put buckets and get
	for _, e := range tests {
		addr, _ := address.FromBytes(e.name[:])
		_, err := getBucket(sm, e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))

		vb := NewVoteBucket(addr, identityset.Address(1), big.NewInt(2100000000), 21*uint32(e.index+1), time.Now(), true)

		count, err := getTotalBucketCount(sm)
		require.NoError(err)
		require.Equal(e.index, count)
		count, err = putBucket(sm, vb)
		require.NoError(err)
		require.Equal(e.index, count)
		count, err = getTotalBucketCount(sm)
		require.NoError(err)
		require.Equal(e.index+1, count)
		vb1, err := getBucket(sm, e.index)
		require.NoError(err)
		require.Equal(e.index, vb1.Index)
		require.Equal(vb, vb1)
	}

	vb, err := getBucket(sm, 2)
	require.NoError(err)
	vb.AutoStake = false
	vb.StakedAmount.Sub(vb.StakedAmount, big.NewInt(100))
	vb.UnstakeStartTime = time.Now().UTC()
	require.True(vb.isUnstaked())
	require.NoError(updateBucket(sm, 2, vb))
	vb1, err := getBucket(sm, 2)
	require.NoError(err)
	require.Equal(vb, vb1)

	// delete buckets and get
	for _, e := range tests {
		require.NoError(delBucket(sm, e.index))
		_, err := getBucket(sm, e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}
