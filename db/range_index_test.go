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
		{999, []byte("nine-nine-nine")},
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
	for i := 2; i < len(rangeTests); i++ {
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
	for i := 2; i <= 3; i++ {
		v, err = index.Get(rangeTests[i].k)
		require.NoError(err)
		require.Equal(rangeTests[2].v, v)
		v, err = index.Get(rangeTests[i].k + 1)
		require.NoError(err)
		require.Equal(rangeTests[2].v, v)
	}

	// key 4 not affected
	v, err = index.Get(rangeTests[4].k)
	require.NoError(err)
	require.Equal(rangeTests[4].v, v)
	v, err = index.Get(rangeTests[4].k + 1)
	require.NoError(err)
	require.Equal(rangeTests[4].v, v)

	// add rangeTests[3].k back with a diff value
	rangeTests[3].v = []byte("not-hundred")
	require.NoError(index.Insert(rangeTests[3].k, rangeTests[3].v))
	for i := 2; i < len(rangeTests); i++ {
		v, err = index.Get(rangeTests[i].k)
		require.NoError(err)
		require.Equal(rangeTests[i].v, v)
		v, err = index.Get(rangeTests[i].k + 1)
		require.NoError(err)
		require.Equal(rangeTests[i].v, v)
	}

	// purge rangeTests[3].k
	require.NoError(index.Purge(rangeTests[3].k))
	for i := 1; i <= 3; i++ {
		v, err = index.Get(rangeTests[i].k)
		require.NoError(err)
		require.Equal(NotExist, v)
		v, err = index.Get(rangeTests[i].k + 1)
		require.NoError(err)
		require.Equal(NotExist, v)
	}

	// key 4 not affected
	v, err = index.Get(rangeTests[4].k)
	require.NoError(err)
	require.Equal(rangeTests[4].v, v)
	v, err = index.Get(rangeTests[4].k + 1)
	require.NoError(err)
	require.Equal(rangeTests[4].v, v)
}

func TestRangeIndex2(t *testing.T) {
	require := require.New(t)

	path := "test-ranger"
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

	testNS := []byte("test")
	index, err := kv.CreateRangeIndexNX(testNS, NotExist)
	require.NoError(err)
	// special case: insert 1
	err = index.Insert(1, []byte("1"))
	require.NoError(err)
	v, err := index.Get(5)
	require.NoError(err)
	require.Equal([]byte("1"), v)

	err = index.Purge(1)
	require.NoError(err)

	err = index.Insert(7, []byte("7"))
	require.NoError(err)
	// Case I: key before 7
	for i := uint64(1); i < 6; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		require.NoError(err)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal(v, NotExist)
	}
	// Case II: key is 7 and greater than 7
	for i := uint64(7); i < 10; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		require.NoError(err)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal([]byte("7"), v)
	}
	// Case III: duplicate key
	index, err = kv.CreateRangeIndexNX(testNS, NotExist)
	require.NoError(err)
	err = index.Insert(7, []byte("7777"))
	require.NoError(err)
	for i := uint64(7); i < 10; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal([]byte("7777"), v)
	}
	// Case IV: delete key less than 7
	index, err = kv.CreateRangeIndexNX(testNS, NotExist)
	require.NoError(err)
	err = index.Insert(66, []byte("66"))
	require.NoError(err)
	for i := uint64(1); i < 7; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		err = index.Delete(i)
		require.NoError(err)
	}
	index, err = kv.CreateRangeIndexNX(testNS, NotExist)
	require.NoError(err)
	v, err = index.Get(7)
	require.NoError(err)
	require.Equal([]byte("7777"), v)
	// Case V: delete key 7
	index, err = kv.CreateRangeIndexNX(testNS, NotExist)
	err = index.Purge(10)
	require.NoError(err)
	for i := uint64(1); i < 66; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		require.NoError(err)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal(v, NotExist)
	}
	for i := uint64(66); i < 70; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		require.NoError(err)
		v, err = index.Get(i)
		require.Equal([]byte("66"), v)
	}
	// Case VI: delete key before 80,all keys deleted
	index, err = kv.CreateRangeIndexNX(testNS, NotExist)
	err = index.Insert(70, []byte("70"))
	require.NoError(err)
	index, err = kv.CreateRangeIndexNX(testNS, NotExist)
	err = index.Insert(80, []byte("80"))
	require.NoError(err)
	err = index.Insert(91, []byte("91"))
	require.NoError(err)
	index, err = kv.CreateRangeIndexNX(testNS, NotExist)
	err = index.Purge(79)
	require.NoError(err)
	for i := uint64(1); i < 80; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		require.NoError(err)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal(v, NotExist)
	}
	for i := uint64(80); i < 91; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		require.NoError(err)
		v, err = index.Get(i)
		require.NoError(err)
		require.Equal([]byte("80"), v)
	}
	for i := uint64(91); i < 100; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, NotExist)
		require.NoError(err)
		v, err = index.Get(i)
		require.NoError(err)
		require.Equal([]byte("91"), v)
	}
}
