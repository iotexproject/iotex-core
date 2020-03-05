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

func TestBucketIndices(t *testing.T) {
	require := require.New(t)

	bis := make(BucketIndices, 0)

	bis.addBucketIndex(uint64(1))
	bis.addBucketIndex(uint64(2))
	bis.addBucketIndex(uint64(3))

	data, err := bis.Serialize()
	require.NoError(err)
	bis1 := BucketIndices{}
	require.NoError(bis1.Deserialize(data))
	require.Equal(3, len(bis1))

	require.Equal(uint64(1), bis1[0])
	require.Equal(uint64(2), bis1[1])
	require.Equal(uint64(3), bis1[2])
}

func TestGetPutBucketIndex(t *testing.T) {
	testGetPut := func(t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		sm := newMockStateManager(ctrl)

		tests := []struct {
			index     uint64
			voterAddr address.Address
		}{
			{
				uint64(1),
				identityset.Address(1),
			},
			{
				uint64(2),
				identityset.Address(1),
			},
			{
				uint64(3),
				identityset.Address(1),
			},
			{
				uint64(4),
				identityset.Address(1),
			},
		}

		// put buckets and get
		for i, e := range tests {
			_, err := getVoterIndices(sm, e.voterAddr)
			if i == 0 {
				require.Equal(state.ErrStateNotExist, errors.Cause(err))
			}

			require.NoError(putVoterIndex(sm, e.voterAddr, e.index))
			bis, err := getVoterIndices(sm, e.voterAddr)
			require.NoError(err)
			bucketIndices := *bis
			require.Equal(i+1, len(bucketIndices))
			require.Equal(bucketIndices[i], e.index)
		}

		for i, e := range tests {
			require.NoError(delVoterIndex(sm, e.voterAddr, e.index))
			indices, err := getVoterIndices(sm, e.voterAddr)
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
