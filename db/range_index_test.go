// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestRangeIndex(t *testing.T) {
	require := require.New(t)

	rangeTests := []struct {
		k uint64
		v []byte
	}{
		{0, []byte("beyond")},
		{7, []byte("seven")},
		{29, []byte("twenty-nine")},
		{100, []byte("hundred")},
	}

	path := "test-indexer"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	cfg := config.Default.DB
	cfg.DbPath = testPath
	testutil.CleanupPath(t, testPath)
	defer testutil.CleanupPath(t, testPath)

	kv := NewBoltDB(cfg)
	require.NotNil(kv)

	require.NoError(kv.Start(context.Background()))
	defer func() {
		require.NoError(kv.Stop(context.Background()))
	}()

	index, err := kv.CreateRangeIndexNX([]byte("test"), rangeTests[0].v)
	require.NoError(err)
	v, err := index.Get(0)
	require.NoError(err)
	require.Equal(rangeTests[0].v, v)
	v, err = index.Get(1)
	require.NoError(err)
	require.Equal(rangeTests[0].v, v)

	for i, e := range rangeTests {
		if i == 0 {
			continue
		}
		require.NoError(index.Insert(e.k, e.v))
		// test 5 random keys between the new and previous insertion
		gap := e.k - rangeTests[i-1].k
		for j := 0; j < 5; j++ {
			k := rangeTests[i-1].k + uint64(rand.Intn(int(gap)))
			v, err := index.Get(k)
			require.NoError(err)
			require.Equal(rangeTests[i-1].v, v)
		}
		v, err := index.Get(e.k - 1)
		require.NoError(err)
		require.Equal(rangeTests[i-1].v, v)
		v, err = index.Get(e.k)
		require.NoError(err)
		require.Equal(e.v, v)

		// test 5 random keys beyond new insertion
		for j := 0; j < 5; j++ {
			k := e.k + uint64(rand.Int())
			v, err := index.Get(k)
			require.NoError(err)
			require.Equal(e.v, v)
		}
	}

	// delete rangeTests[1].k
	require.Equal(ErrInvalid, errors.Cause(index.Delete(0)))
	require.NoError(index.Delete(rangeTests[1].k))
	v, err = index.Get(rangeTests[1].k)
	require.NoError(err)
	require.Equal(rangeTests[0].v, v)
	for i := 2; i <= 3; i++ {
		v, err = index.Get(rangeTests[i].k)
		require.NoError(err)
		require.Equal(rangeTests[i].v, v)
		v, err = index.Get(rangeTests[i].k + 1)
		require.NoError(err)
		require.Equal(rangeTests[i].v, v)
	}

	// delete rangeTests[3].k
	require.NoError(index.Delete(rangeTests[3].k))
	v, err = index.Get(rangeTests[3].k)
	require.NoError(err)
	require.Equal(rangeTests[2].v, v)
	v, err = index.Get(rangeTests[3].k + 1)
	require.NoError(err)
	require.Equal(rangeTests[2].v, v)
}
