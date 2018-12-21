// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCachedBatch(t *testing.T) {
	require := require.New(t)

	cb := NewCachedBatch()
	cb.Put(bucket1, testK1[0], testV1[0], "")
	v, err := cb.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[0], v)
	require.Equal(ErrAlreadyExist, cb.PutIfNotExists(bucket1, testK1[0], testV1[0], ""))
	v, err = cb.Get(bucket1, testK2[0])
	require.Equal(ErrNotExist, err)
	require.Equal([]byte(nil), v)

	cb.Delete(bucket1, testK2[0], "")
	cb.Delete(bucket1, testK1[0], "")
	_, err = cb.Get(bucket1, testK1[0])
	require.Equal(ErrAlreadyDeleted, errors.Cause(err))
	require.NoError(cb.PutIfNotExists(bucket1, testK1[0], testV1[0], ""))
	v, err = cb.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[0], v)

	w, err := cb.Entry(1)
	require.NoError(err)
	require.Equal(bucket1, w.namespace)
	require.Equal(testK2[0], w.key)
	require.Equal([]byte(nil), w.value)
	require.Equal(Delete, w.writeType)

	w, err = cb.Entry(3)
	require.NoError(err)
	require.Equal(bucket1, w.namespace)
	require.Equal(testK1[0], w.key)
	require.Equal(testV1[0], w.value)
	require.Equal(PutIfNotExists, w.writeType)

	// test clone
	c := cb.clone()
	v, err = c.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[0], v)

	w, err = c.Entry(0)
	require.NoError(err)
	require.Equal(bucket1, w.namespace)
	require.Equal(testK1[0], w.key)
	require.Equal(testV1[0], w.value)
	require.Equal(Put, w.writeType)

	w, err = c.Entry(2)
	require.NoError(err)
	require.Equal(bucket1, w.namespace)
	require.Equal(testK1[0], w.key)
	require.Equal([]byte(nil), w.value)
	require.Equal(Delete, w.writeType)
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)

	cb := NewCachedBatch()
	cb.Put(bucket1, testK1[0], testV1[0], "")
	cb.Put(bucket1, testK1[1], testV1[1], "")
	s0 := cb.Snapshot()
	require.Equal(0, s0)
	require.Equal(2, cb.Size())

	cb.Put(bucket1, testK2[0], testV2[0], "")
	cb.Put(bucket1, testK2[1], testV2[1], "")
	cb.Delete(bucket1, testK1[0], "")
	v, err := cb.Get(bucket1, testK1[0])
	require.Equal(ErrAlreadyDeleted, err)
	require.Nil(v)
	s1 := cb.Snapshot()
	require.Equal(1, s1)
	require.Equal(5, cb.Size())

	cb.Put(bucket1, testK1[2], testV1[2], "")
	cb.Put(bucket1, testK2[2], testV2[2], "")
	require.NoError(cb.PutIfNotExists(bucket1, testK1[0], testV1[0], ""))
	cb.Delete(bucket1, testK2[0], "")
	_, err = cb.Get(bucket1, testK2[0])
	require.Equal(ErrAlreadyDeleted, err)
	s2 := cb.Snapshot()
	require.Equal(2, s2)
	require.Equal(9, cb.Size())

	// snapshot 2
	require.Error(cb.Revert(3))
	require.Error(cb.Revert(-1))
	require.NoError(cb.Revert(2))
	_, err = cb.Get(bucket1, testK2[0])
	require.Equal(ErrAlreadyDeleted, err)
	v, err = cb.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[0], v)
	v, err = cb.Get(bucket1, testK1[1])
	require.NoError(err)
	require.Equal(testV1[1], v)
	v, err = cb.Get(bucket1, testK2[1])
	require.NoError(err)
	require.Equal(testV2[1], v)
	v, err = cb.Get(bucket1, testK1[2])
	require.NoError(err)
	require.Equal(testV1[2], v)
	v, err = cb.Get(bucket1, testK2[2])
	require.NoError(err)
	require.Equal(testV2[2], v)

	// snapshot 1
	require.NoError(cb.Revert(2))
	require.NoError(cb.Revert(1))
	_, err = cb.Get(bucket1, testK1[0])
	require.Equal(ErrAlreadyDeleted, err)
	v, err = cb.Get(bucket1, testK1[1])
	require.NoError(err)
	require.Equal(testV1[1], v)
	v, err = cb.Get(bucket1, testK2[0])
	require.NoError(err)
	require.Equal(testV2[0], v)
	v, err = cb.Get(bucket1, testK2[1])
	require.NoError(err)
	require.Equal(testV2[1], v)
	_, err = cb.Get(bucket1, testK2[2])
	require.Equal(ErrNotExist, err)

	// snapshot 0
	require.Error(cb.Revert(2))
	require.NoError(cb.Revert(0))
	v, err = cb.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[0], v)
	v, err = cb.Get(bucket1, testK1[1])
	require.NoError(err)
	require.Equal(testV1[1], v)
	_, err = cb.Get(bucket1, testK2[0])
	require.Equal(ErrNotExist, err)
	_, err = cb.Get(bucket1, testK1[2])
	require.Equal(ErrNotExist, err)
}
