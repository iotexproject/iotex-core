// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"testing"

	"github.com/iotexproject/iotex-core/db/batch"

	"github.com/stretchr/testify/require"
)

var (
	namespace = "ns"

	k1 = []byte("key_1")
	k2 = []byte("key_2")
	k3 = []byte("key_3")
	k4 = []byte("key_4")

	v1 = []byte("value_1")
	v2 = []byte("value_2")
	v3 = []byte("value_3")
	v4 = []byte("value_4")
)

func TestKVStoreImpl(t *testing.T) {
	require := require.New(t)

	s := NewMemKVStore()

	// 1. read nonexistent namespace
	v, err := s.Get(namespace, k1)
	require.Error(err)
	require.Nil(v)

	// 2. read nonexistent key
	err = s.Put(namespace, k1, v1)
	require.NoError(err)
	v, err = s.Get(namespace, k2)
	require.Error(err)
	require.Nil(v)

	// 3. write once read many times
	v, err = s.Get(namespace, k1)
	require.NoError(err)
	require.Equal(v, v1)

	// 4. write the same key many times
	err = s.Put(namespace, k1, v2)
	require.NoError(err)
	v, err = s.Get(namespace, k1)
	require.NoError(err)
	require.Equal(v, v2)

	err = s.Put(namespace, k1, v3)
	require.NoError(err)
	v, err = s.Get(namespace, k1)
	require.NoError(err)
	require.Equal(v, v3)

	// 5. delete nonexistent key
	err = s.Delete(namespace, k2)
	require.NoError(err)

	// 6. delete once
	err = s.Delete(namespace, k1)
	require.NoError(err)

	// 7. read deleted key
	v, err = s.Get(namespace, k1)
	require.Error(err)
	require.Nil(v)

	// 8. write the same key again after deleted
	err = s.Put(namespace, k1, v1)
	require.NoError(err)
	v, err = s.Get(namespace, k1)
	require.NoError(err)
	require.Equal(v, v1)

	// 9. delete the same key many times
	err = s.Delete(namespace, k1)
	require.NoError(err)
	err = s.Delete(namespace, k1)
	require.NoError(err)
	err = s.Delete(namespace, k1)
	require.NoError(err)

	// 10. write many key-value pairs
	err = s.Put(namespace, k1, v1)
	require.NoError(err)
	err = s.Put(namespace, k2, v2)
	require.NoError(err)
	err = s.Put(namespace, k3, v3)
	require.NoError(err)

	v, err = s.Get(namespace, k1)
	require.NoError(err)
	require.Equal(v, v1)

	v, err = s.Get(namespace, k2)
	require.NoError(err)
	require.Equal(v, v2)

	v, err = s.Get(namespace, k3)
	require.NoError(err)
	require.Equal(v, v3)

	// 11. batch write for existing KvStore
	b := batch.NewBatch()
	b.Put(namespace, k1, v1, "")
	b.Put(namespace, k2, v3, "")
	b.Put(namespace, k3, v2, "")
	b.Put(namespace, k4, v4, "")

	err = s.WriteBatch(b)
	require.NoError(err)

	v, err = s.Get(namespace, k1)
	require.NoError(err)
	require.Equal(v, v1)

	v, err = s.Get(namespace, k2)
	require.NoError(err)
	require.Equal(v, v3)

	v, err = s.Get(namespace, k3)
	require.NoError(err)
	require.Equal(v, v2)

	v, err = s.Get(namespace, k4)
	require.NoError(err)
	require.Equal(v, v4)

	// 12. batch write for brand new KvStore
	ss := NewMemKVStore()

	bb := batch.NewBatch()
	bb.Put(namespace, k1, v1, "")
	bb.Put(namespace, k2, v2, "")
	bb.Put(namespace, k3, v3, "")
	bb.Put(namespace, k4, v4, "")

	err = ss.WriteBatch(bb)
	require.NoError(err)

	v, err = ss.Get(namespace, k1)
	require.NoError(err)
	require.Equal(v, v1)

	v, err = ss.Get(namespace, k2)
	require.NoError(err)
	require.Equal(v, v2)

	v, err = ss.Get(namespace, k3)
	require.NoError(err)
	require.Equal(v, v3)

	v, err = ss.Get(namespace, k4)
	require.NoError(err)
	require.Equal(v, v4)
}
