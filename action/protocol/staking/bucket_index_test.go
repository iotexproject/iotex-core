// Copyright (c) 2020 IoTeX Foundation
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

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
)

const (
	stateDBPath1 = "stateDB1.test"
)

func TestBucketIndex(t *testing.T) {
	require := require.New(t)

	bi := NewBucketIndex(uint64(1), fakeCanName(identityset.Address(1).String(), uint64(1)))

	data, err := bi.Serialize()
	require.NoError(err)
	bi1 := BucketIndex{}
	require.NoError(bi1.Deserialize(data))
	require.Equal(bi.Index, bi1.Index)
	require.Equal(bi.CanName, bi1.CanName)
}

func TestBucketIndices(t *testing.T) {
	require := require.New(t)

	bis := NewBucketIndices()

	bi1 := NewBucketIndex(uint64(1), fakeCanName(identityset.Address(1).String(), uint64(1)))
	bi2 := NewBucketIndex(uint64(2), fakeCanName(identityset.Address(2).String(), uint64(2)))
	bi3 := NewBucketIndex(uint64(3), fakeCanName(identityset.Address(3).String(), uint64(3)))

	bis.addBucketIndex(bi1)
	bis.addBucketIndex(bi2)
	bis.addBucketIndex(bi3)

	data, err := bis.Serialize()
	require.NoError(err)
	bis1 := BucketIndices{}
	require.NoError(bis1.Deserialize(data))
	bucketIndices := bis1.GetIndices()
	require.Equal(3, len(bucketIndices))

	require.Equal(bi1.Index, bucketIndices[0].Index)
	require.Equal(bi1.CanName, bucketIndices[0].CanName)

	require.Equal(bi2.Index, bucketIndices[1].Index)
	require.Equal(bi2.CanName, bucketIndices[1].CanName)

	require.Equal(bi3.Index, bucketIndices[2].Index)
	require.Equal(bi3.CanName, bucketIndices[2].CanName)
}

func TestGetPutBucketIndex(t *testing.T) {
	testGetPut := func(sf factory.Factory, t *testing.T) {
		require := require.New(t)
		ctx := context.Background()
		require.NoError(sf.Start(ctx))
		defer func() {
			require.NoError(sf.Stop(ctx))
		}()

		tests := []struct {
			canName   [12]byte
			index     uint64
			voterAddr string
		}{
			{
				fakeCanName(identityset.Address(1).String(), uint64(1)),
				uint64(1),
				identityset.Address(1).String(),
			},
			{
				fakeCanName(identityset.Address(2).String(), uint64(2)),
				uint64(2),
				identityset.Address(1).String(),
			},
			{
				fakeCanName(identityset.Address(3).String(), uint64(3)),
				uint64(3),
				identityset.Address(1).String(),
			},
			{
				fakeCanName(identityset.Address(4).String(), uint64(4)),
				uint64(4),
				identityset.Address(1).String(),
			},
		}

		ws, err := sf.NewWorkingSet()
		require.NoError(err)
		require.NotNil(ws)

		// put buckets and get
		for i, e := range tests {
			_, err := stakingGetBucketIndices(sf, e.voterAddr)
			if i == 0 {
				require.Equal(state.ErrStateNotExist, errors.Cause(err))
			}

			bi := NewBucketIndex(e.index, e.canName)

			require.NoError(stakingPutBucketIndex(ws, e.voterAddr, bi))
			bis, err := stakingGetBucketIndices(ws, e.voterAddr)
			require.NoError(err)
			require.Equal(i+1, len(bis.GetIndices()))
			indices := bis.GetIndices()
			require.Equal(indices[i].CanName, e.canName[:])
			require.Equal(indices[i].Index, e.index)
		}

		for i, e := range tests {
			require.NoError(stakingDelBucketIndex(ws, e.voterAddr, e.index))
			indices, err := stakingGetBucketIndices(ws, e.voterAddr)
			if i != len(tests)-1 {
				require.NoError(err)
				require.Equal(len(tests)-i-1, len(indices.GetIndices()))
				continue
			}
			require.Equal(state.ErrStateNotExist, errors.Cause(err))
		}
	}

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath1)
	testStateDBPath := testTrieFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, _ := factory.NewStateDB(cfg, factory.DefaultStateDBOption())

	t.Run("test put and get bucket index", func(t *testing.T) {
		testGetPut(sdb, t)
	})
}

func fakeCanName(addr string, index uint64) CandName {
	var name [12]byte
	copy(name[:4], addr[3:])
	copy(name[4:], byteutil.Uint64ToBytesBigEndian(index))
	return name
}
