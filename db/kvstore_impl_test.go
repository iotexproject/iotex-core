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
	_namespace = "ns"

	_k1 = []byte("key_1")
	_k2 = []byte("key_2")
	_k3 = []byte("key_3")
	_k4 = []byte("key_4")

	_v1 = []byte("value_1")
	_v2 = []byte("value_2")
	_v3 = []byte("value_3")
	_v4 = []byte("value_4")
)

func TestKVStoreImpl(t *testing.T) {
	require := require.New(t)

	s := NewMemKVStore()

	// 1. read nonexistent _namespace
	v, err := s.Get(_namespace, _k1)
	require.Error(err)
	require.Nil(v)

	// 2. read nonexistent key
	err = s.Put(_namespace, _k1, _v1)
	require.NoError(err)
	v, err = s.Get(_namespace, _k2)
	require.Error(err)
	require.Nil(v)

	// 3. write once read many times
	v, err = s.Get(_namespace, _k1)
	require.NoError(err)
	require.Equal(v, _v1)

	// 4. write the same key many times
	err = s.Put(_namespace, _k1, _v2)
	require.NoError(err)
	v, err = s.Get(_namespace, _k1)
	require.NoError(err)
	require.Equal(v, _v2)

	err = s.Put(_namespace, _k1, _v3)
	require.NoError(err)
	v, err = s.Get(_namespace, _k1)
	require.NoError(err)
	require.Equal(v, _v3)

	// 5. delete nonexistent key
	err = s.Delete(_namespace, _k2)
	require.NoError(err)

	// 6. delete once
	err = s.Delete(_namespace, _k1)
	require.NoError(err)

	// 7. read deleted key
	v, err = s.Get(_namespace, _k1)
	require.Error(err)
	require.Nil(v)

	// 8. write the same key again after deleted
	err = s.Put(_namespace, _k1, _v1)
	require.NoError(err)
	v, err = s.Get(_namespace, _k1)
	require.NoError(err)
	require.Equal(v, _v1)

	// 9. delete the same key many times
	err = s.Delete(_namespace, _k1)
	require.NoError(err)
	err = s.Delete(_namespace, _k1)
	require.NoError(err)
	err = s.Delete(_namespace, _k1)
	require.NoError(err)

	// 10. write many key-value pairs
	err = s.Put(_namespace, _k1, _v1)
	require.NoError(err)
	err = s.Put(_namespace, _k2, _v2)
	require.NoError(err)
	err = s.Put(_namespace, _k3, _v3)
	require.NoError(err)

	v, err = s.Get(_namespace, _k1)
	require.NoError(err)
	require.Equal(v, _v1)

	v, err = s.Get(_namespace, _k2)
	require.NoError(err)
	require.Equal(v, _v2)

	v, err = s.Get(_namespace, _k3)
	require.NoError(err)
	require.Equal(v, _v3)

	// 11. batch write for existing KvStore
	b := batch.NewBatch()
	b.Put(_namespace, _k1, _v1, "")
	b.Put(_namespace, _k2, _v3, "")
	b.Put(_namespace, _k3, _v2, "")
	b.Put(_namespace, _k4, _v4, "")

	err = s.WriteBatch(b)
	require.NoError(err)

	v, err = s.Get(_namespace, _k1)
	require.NoError(err)
	require.Equal(v, _v1)

	v, err = s.Get(_namespace, _k2)
	require.NoError(err)
	require.Equal(v, _v3)

	v, err = s.Get(_namespace, _k3)
	require.NoError(err)
	require.Equal(v, _v2)

	v, err = s.Get(_namespace, _k4)
	require.NoError(err)
	require.Equal(v, _v4)

	// 12. batch write for brand new KvStore
	ss := NewMemKVStore()

	bb := batch.NewBatch()
	bb.Put(_namespace, _k1, _v1, "")
	bb.Put(_namespace, _k2, _v2, "")
	bb.Put(_namespace, _k3, _v3, "")
	bb.Put(_namespace, _k4, _v4, "")

	err = ss.WriteBatch(bb)
	require.NoError(err)

	v, err = ss.Get(_namespace, _k1)
	require.NoError(err)
	require.Equal(v, _v1)

	v, err = ss.Get(_namespace, _k2)
	require.NoError(err)
	require.Equal(v, _v2)

	v, err = ss.Get(_namespace, _k3)
	require.NoError(err)
	require.Equal(v, _v3)

	v, err = ss.Get(_namespace, _k4)
	require.NoError(err)
	require.Equal(v, _v4)
}
