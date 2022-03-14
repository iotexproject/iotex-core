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
	"github.com/iotexproject/iotex-core/testutil/testdb"
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
		sm := testdb.NewMockStateManager(ctrl)
		csm := smToCsm(sm)
		csr := srToCsr(sm)

		tests := []struct {
			index          uint64
			voterAddr      address.Address
			candAddr       address.Address
			voterIndexSize int
			candIndexSize  int
		}{
			{
				uint64(1),
				identityset.Address(1),
				identityset.Address(1),
				1,
				1,
			},
			{
				uint64(2),
				identityset.Address(1),
				identityset.Address(2),
				2,
				1,
			},
			{
				uint64(3),
				identityset.Address(1),
				identityset.Address(3),
				3,
				1,
			},
			{
				uint64(4),
				identityset.Address(2),
				identityset.Address(1),
				1,
				2,
			},
			{
				uint64(5),
				identityset.Address(2),
				identityset.Address(2),
				2,
				2,
			},
			{
				uint64(6),
				identityset.Address(2),
				identityset.Address(3),
				3,
				2,
			},
			{
				uint64(7),
				identityset.Address(3),
				identityset.Address(1),
				1,
				3,
			},
			{
				uint64(8),
				identityset.Address(3),
				identityset.Address(2),
				2,
				3,
			},
			{
				uint64(9),
				identityset.Address(3),
				identityset.Address(3),
				3,
				3,
			},
		}
		// after adding above, each voter and candidate will have a total of 3 indices
		indexSize := 3

		// put buckets and get
		for i, e := range tests {
			_, _, err := getVoterBucketIndices(csr, e.voterAddr)
			if i == 0 {
				require.Equal(state.ErrStateNotExist, errors.Cause(err))
			}
			_, _, err = getCandBucketIndices(csr, e.candAddr)
			if i == 0 {
				require.Equal(state.ErrStateNotExist, errors.Cause(err))
			}

			// put voter bucket index
			require.NoError(putVoterBucketIndex(csm, e.voterAddr, e.index))
			bis, _, err := getVoterBucketIndices(csr, e.voterAddr)
			require.NoError(err)
			bucketIndices := *bis
			require.Equal(e.voterIndexSize, len(bucketIndices))
			require.Equal(bucketIndices[e.voterIndexSize-1], e.index)

			// put candidate bucket index
			require.NoError(putCandBucketIndex(csm, e.candAddr, e.index))
			bis, _, err = getCandBucketIndices(csr, e.candAddr)
			require.NoError(err)
			bucketIndices = *bis
			require.Equal(e.candIndexSize, len(bucketIndices))
			require.Equal(bucketIndices[e.candIndexSize-1], e.index)
		}

		for _, e := range tests {
			// delete voter bucket index
			require.NoError(delVoterBucketIndex(csm, e.voterAddr, e.index))
			bis, _, err := getVoterBucketIndices(csr, e.voterAddr)
			if e.voterIndexSize != indexSize {
				bucketIndices := *bis
				require.Equal(indexSize-e.voterIndexSize, len(bucketIndices))
			} else {
				require.Equal(state.ErrStateNotExist, errors.Cause(err))
			}

			// delete candidate bucket index
			require.NoError(delCandBucketIndex(csm, e.candAddr, e.index))
			bis, _, err = getCandBucketIndices(csr, e.candAddr)
			if e.candIndexSize != indexSize {
				bucketIndices := *bis
				require.Equal(indexSize-e.candIndexSize, len(bucketIndices))
			} else {
				require.Equal(state.ErrStateNotExist, errors.Cause(err))
			}
		}
	}

	t.Run("test put and get bucket index", testGetPut)
}
