// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package batch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	k1 = &kvCacheKey{"ns", "key"}
	k2 = &kvCacheKey{"nsk", "ey"}
	k3 = &kvCacheKey{"n", "skey"}

	v1 = []byte("value_1")
	v2 = []byte("value_2")
	v3 = []byte("value_3")
)

func TestKvCache(t *testing.T) {
	require := require.New(t)

	c := NewKVCache()

	// 1. read nonexistent key
	v, err := c.Read(k1)
	require.Equal(err, ErrNotExist)
	require.Nil(v)

	// 2. write once read many times
	c.Write(k1, v1)
	v, err = c.Read(k1)
	require.NoError(err)
	require.Equal(v, v1)

	v, err = c.Read(k1)
	require.NoError(err)
	require.Equal(v, v1)

	// 3. write the same key many times
	err = c.WriteIfNotExist(k1, v1)
	require.Equal(err, ErrAlreadyExist)

	c.Write(k1, v2)
	v, err = c.Read(k1)
	require.NoError(err)
	require.Equal(v, v2)

	c.Write(k1, v3)
	v, err = c.Read(k1)
	require.NoError(err)
	require.Equal(v, v3)

	// 4. delete nonexistent key
	c.Evict(k2)

	// 5. delete once
	c.Evict(k1)

	// 6. read deleted key
	v, err = c.Read(k1)
	require.Equal(err, ErrAlreadyDeleted)
	require.Nil(v)

	// 7. write the same key again after deleted
	c.Write(k1, v1)
	v, err = c.Read(k1)
	require.NoError(err)
	require.Equal(v, v1)

	// 8. delete the same key many times
	c.Evict(k1)
	c.Evict(k1)
	c.Evict(k1)

	// 9. write many key-value pairs
	c.Write(k1, v1)
	c.Write(k2, v2)
	c.Write(k3, v3)

	v, err = c.Read(k1)
	require.NoError(err)
	require.Equal(v, v1)

	v, err = c.Read(k2)
	require.NoError(err)
	require.Equal(v, v2)

	v, err = c.Read(k3)
	require.NoError(err)
	require.Equal(v, v3)
}

func TestWriteIfNotExist(t *testing.T) {
	require := require.New(t)

	c := NewKVCache()

	v, err := c.Read(k1)
	require.Equal(err, ErrNotExist)
	require.Nil(v)

	err = c.WriteIfNotExist(k1, v1)
	require.NoError(err)

	err = c.WriteIfNotExist(k1, v1)
	require.Equal(err, ErrAlreadyExist)

	c.Evict(k1)
	err = c.WriteIfNotExist(k1, v1)
	require.NoError(err)
}
