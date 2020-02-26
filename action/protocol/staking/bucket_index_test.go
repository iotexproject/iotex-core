// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

const (
	stateDBPath1 = "stateDB1.test"
)

func TestBucketIndex(t *testing.T) {
	require := require.New(t)

	bi, err := NewBucketIndex(uint64(1), identityset.Address(1).String())
	require.NoError(err)

	data, err := bi.Serialize()
	require.NoError(err)
	bi1 := BucketIndex{}
	require.NoError(bi1.Deserialize(data))
	require.Equal(bi.Index, bi1.Index)
	require.Equal(bi.CandAddress, bi1.CandAddress)
}

func TestBucketIndices(t *testing.T) {
	require := require.New(t)

	bis := make(BucketIndices, 0)

	bi1, err := NewBucketIndex(uint64(1), identityset.Address(1).String())
	require.NoError(err)
	bi2, err := NewBucketIndex(uint64(2), identityset.Address(2).String())
	require.NoError(err)
	bi3, err := NewBucketIndex(uint64(3), identityset.Address(3).String())
	require.NoError(err)

	bis.addBucketIndex(bi1)
	bis.addBucketIndex(bi2)
	bis.addBucketIndex(bi3)

	data, err := bis.Serialize()
	require.NoError(err)
	bis1 := BucketIndices{}
	require.NoError(bis1.Deserialize(data))
	require.Equal(3, len(bis1))

	require.Equal(bi1.Index, bis1[0].Index)
	require.Equal(bi1.CandAddress, bis1[0].CandAddress)

	require.Equal(bi2.Index, bis1[1].Index)
	require.Equal(bi2.CandAddress, bis1[1].CandAddress)

	require.Equal(bi3.Index, bis1[2].Index)
	require.Equal(bi3.CandAddress, bis1[2].CandAddress)
}

func TestGetPutBucketIndex(t *testing.T) {
	testGetPut := func(t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		sm := newMockStateManager(ctrl)

		tests := []struct {
			canAddress address.Address
			index      uint64
			voterAddr  address.Address
		}{
			{
				identityset.Address(2),
				uint64(1),
				identityset.Address(1),
			},
			{
				identityset.Address(3),
				uint64(2),
				identityset.Address(1),
			},
			{
				identityset.Address(4),
				uint64(3),
				identityset.Address(1),
			},
			{
				identityset.Address(5),
				uint64(4),
				identityset.Address(1),
			},
		}

		// put buckets and get
		for i, e := range tests {
			_, err := stakingGetBucketIndices(sm, e.voterAddr)
			if i == 0 {
				require.Equal(state.ErrStateNotExist, errors.Cause(err))
			}

			bi, err := NewBucketIndex(e.index, e.canAddress.String())

			require.NoError(stakingPutBucketIndex(sm, e.voterAddr, bi))
			bis, err := stakingGetBucketIndices(sm, e.voterAddr)
			require.NoError(err)
			bucketIndices := *bis
			require.Equal(i+1, len(bucketIndices))
			require.Equal(bucketIndices[i].CandAddress, e.canAddress)
			require.Equal(bucketIndices[i].Index, e.index)
		}

		for i, e := range tests {
			require.NoError(stakingDelBucketIndex(sm, e.voterAddr, e.index))
			indices, err := stakingGetBucketIndices(sm, e.voterAddr)
			if i != len(tests)-1 {
				require.NoError(err)
				require.Equal(len(tests)-i-1, len(*indices))
				continue
			}
			require.Equal(state.ErrStateNotExist, errors.Cause(err))
		}
	}

	t.Run("test put and get bucket index", testGetPut)
}
