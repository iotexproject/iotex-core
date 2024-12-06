// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/testutil"
)

type versionTest struct {
	ns     string
	k, v   []byte
	height uint64
	err    error
}

func TestVersionedDB(t *testing.T) {
	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-version")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := DefaultConfig
	cfg.DbPath = testPath
	db := NewBoltDBVersioned(cfg)
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	// namespace and key does not exist
	vn, err := db.checkNamespace(_bucket1)
	r.Nil(vn)
	r.Nil(err)
	// write first key, namespace and key now exist
	r.NoError(db.Put(0, _bucket1, _k2, _v2))
	vn, err = db.checkNamespace(_bucket1)
	r.NoError(err)
	r.EqualValues(len(_k2), vn.keyLen)
	// check more Put/Get
	var (
		_k5  = []byte("key_5")
		_k10 = []byte("key_10")
	)
	err = db.Put(1, _bucket1, _k10, _v1)
	r.Equal("invalid key length, expecting 5, got 6: invalid input", err.Error())
	r.NoError(db.Put(1, _bucket1, _k2, _v1))
	r.NoError(db.Put(3, _bucket1, _k2, _v3))
	r.NoError(db.Put(6, _bucket1, _k2, _v2))
	r.NoError(db.Put(2, _bucket1, _k4, _v2))
	r.NoError(db.Put(4, _bucket1, _k4, _v1))
	r.NoError(db.Put(7, _bucket1, _k4, _v3))
	for _, e := range []versionTest{
		{_bucket2, _k1, nil, 0, ErrNotExist}, // bucket not exist
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, _v2, 0, nil},
		{_bucket1, _k2, _v1, 1, nil},
		{_bucket1, _k2, _v1, 2, nil},
		{_bucket1, _k2, _v3, 3, nil},
		{_bucket1, _k2, _v3, 5, nil},
		{_bucket1, _k2, _v2, 6, nil},
		{_bucket1, _k2, _v2, 7, nil}, // after last write version
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 1, ErrNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, nil},
		{_bucket1, _k4, _v2, 3, nil},
		{_bucket1, _k4, _v1, 4, nil},
		{_bucket1, _k4, _v1, 6, nil},
		{_bucket1, _k4, _v3, 7, nil},
		{_bucket1, _k4, _v3, 8, nil}, // larger than last key in bucket
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// overwrite the same height again
	r.NoError(db.Put(6, _bucket1, _k2, _v4))
	r.NoError(db.Put(7, _bucket1, _k4, _v4))
	// write to earlier version again is invalid
	r.Equal(ErrInvalid, db.Put(3, _bucket1, _k2, _v4))
	r.Equal(ErrInvalid, db.Put(4, _bucket1, _k4, _v4))
	// write with same value
	r.NoError(db.Put(9, _bucket1, _k2, _v4))
	r.NoError(db.Put(10, _bucket1, _k4, _v4))
	for _, e := range []versionTest{
		{_bucket2, _k1, nil, 0, ErrNotExist}, // bucket not exist
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, _v2, 0, nil},
		{_bucket1, _k2, _v1, 1, nil},
		{_bucket1, _k2, _v1, 2, nil},
		{_bucket1, _k2, _v3, 3, nil},
		{_bucket1, _k2, _v3, 5, nil},
		{_bucket1, _k2, _v4, 6, nil},
		{_bucket1, _k2, _v4, 8, nil},
		{_bucket1, _k2, _v4, 9, nil},
		{_bucket1, _k2, _v4, 10, nil}, // after last write version
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 1, ErrNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, nil},
		{_bucket1, _k4, _v2, 3, nil},
		{_bucket1, _k4, _v1, 4, nil},
		{_bucket1, _k4, _v1, 6, nil},
		{_bucket1, _k4, _v4, 7, nil},
		{_bucket1, _k4, _v4, 9, nil},
		{_bucket1, _k4, _v4, 10, nil},
		{_bucket1, _k4, _v4, 11, nil}, // larger than last key in bucket
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// check version
	for _, e := range []versionTest{
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, nil, 9, nil},
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 10, nil},
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
	} {
		value, err := db.Version(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.height, value)
	}
	// test delete
	r.Equal(ErrNotExist, errors.Cause(db.Delete(10, _bucket2, _k1)))
	for _, k := range [][]byte{_k2, _k4} {
		r.NoError(db.Delete(11, _bucket1, k))
	}
	for _, k := range [][]byte{_k1, _k3, _k5} {
		r.Equal(ErrNotExist, errors.Cause(db.Delete(10, _bucket1, k)))
	}
	r.Equal(ErrInvalid, errors.Cause(db.Delete(10, _bucket1, _k10)))
	// key still can be read before delete version
	for _, e := range []versionTest{
		{_bucket2, _k1, nil, 0, ErrNotExist}, // bucket not exist
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, _v2, 0, nil},
		{_bucket1, _k2, _v1, 1, nil},
		{_bucket1, _k2, _v1, 2, nil},
		{_bucket1, _k2, _v3, 3, nil},
		{_bucket1, _k2, _v3, 5, nil},
		{_bucket1, _k2, _v4, 6, nil},
		{_bucket1, _k2, _v4, 8, nil},
		{_bucket1, _k2, _v4, 9, nil},
		{_bucket1, _k2, _v4, 10, nil},        // before delete version
		{_bucket1, _k2, nil, 11, ErrDeleted}, // after delete version
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 1, ErrNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, nil},
		{_bucket1, _k4, _v2, 3, nil},
		{_bucket1, _k4, _v1, 4, nil},
		{_bucket1, _k4, _v1, 6, nil},
		{_bucket1, _k4, _v4, 7, nil},
		{_bucket1, _k4, _v4, 10, nil},        // before delete version
		{_bucket1, _k4, nil, 11, ErrDeleted}, // after delete version
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// write before delete version is invalid
	r.Equal(ErrInvalid, db.Put(9, _bucket1, _k2, _k2))
	r.Equal(ErrInvalid, db.Put(9, _bucket1, _k4, _k4))
	for _, e := range []versionTest{
		{_bucket1, _k2, _v4, 10, nil},        // before delete version
		{_bucket1, _k2, nil, 11, ErrDeleted}, // after delete version
		{_bucket1, _k4, _v4, 10, nil},        // before delete version
		{_bucket1, _k4, nil, 11, ErrDeleted}, // after delete version
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// write after delete version
	r.NoError(db.Put(12, _bucket1, _k2, _k2))
	r.NoError(db.Put(12, _bucket1, _k4, _k4))
	for _, e := range []versionTest{
		{_bucket2, _k1, nil, 0, ErrNotExist}, // bucket not exist
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, _v2, 0, nil},
		{_bucket1, _k2, _v1, 1, nil},
		{_bucket1, _k2, _v1, 2, nil},
		{_bucket1, _k2, _v3, 3, nil},
		{_bucket1, _k2, _v3, 5, nil},
		{_bucket1, _k2, _v4, 6, nil},
		{_bucket1, _k2, _v4, 8, nil},
		{_bucket1, _k2, _v4, 10, nil},        // before delete version
		{_bucket1, _k2, nil, 11, ErrDeleted}, // after delete version
		{_bucket1, _k2, _k2, 12, nil},        // after next write version
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 1, ErrNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, nil},
		{_bucket1, _k4, _v2, 3, nil},
		{_bucket1, _k4, _v1, 4, nil},
		{_bucket1, _k4, _v1, 6, nil},
		{_bucket1, _k4, _v4, 7, nil},
		{_bucket1, _k4, _v4, 10, nil},        // before delete version
		{_bucket1, _k4, nil, 11, ErrDeleted}, // after delete version
		{_bucket1, _k4, _k4, 12, nil},        // after next write version
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// check version after delete
	for _, e := range []versionTest{
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, nil, 12, nil},
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 12, nil},
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
	} {
		value, err := db.Version(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.height, value)
	}
}

func TestMultipleWriteDelete(t *testing.T) {
	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-version")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := DefaultConfig
	cfg.DbPath = testPath
	db := NewBoltDBVersioned(cfg)
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	// multiple writes and deletes
	r.NoError(db.Put(1, _bucket1, _k2, _v1))
	r.NoError(db.Put(3, _bucket1, _k2, _v3))
	v, err := db.Version(_bucket1, _k2)
	r.NoError(err)
	r.EqualValues(3, v)
	r.NoError(db.Delete(7, _bucket1, _k2))
	_, err = db.Version(_bucket1, _k2)
	r.Equal(ErrDeleted, errors.Cause(err))
	r.NoError(db.Put(10, _bucket1, _k2, _v2))
	v, err = db.Version(_bucket1, _k2)
	r.NoError(err)
	r.EqualValues(10, v)
	r.NoError(db.Delete(15, _bucket1, _k2))
	_, err = db.Version(_bucket1, _k2)
	r.Equal(ErrDeleted, errors.Cause(err))
	r.NoError(db.Put(18, _bucket1, _k2, _v3))
	r.NoError(db.Delete(18, _bucket1, _k2)) // delete-after-write
	_, err = db.Version(_bucket1, _k2)
	r.Equal(ErrDeleted, errors.Cause(err))
	r.NoError(db.Put(18, _bucket1, _k2, _v3)) // write again
	value, err := db.Get(18, _bucket1, _k2)
	r.NoError(err)
	r.Equal(_v3, value)
	v, err = db.Version(_bucket1, _k2)
	r.NoError(err)
	r.EqualValues(18, v)
	r.NoError(db.Delete(18, _bucket1, _k2)) // delete-after-write
	_, err = db.Version(_bucket1, _k2)
	r.Equal(ErrDeleted, errors.Cause(err))
	r.NoError(db.Put(21, _bucket1, _k2, _v4))
	v, err = db.Version(_bucket1, _k2)
	r.NoError(err)
	r.EqualValues(21, v)
	r.NoError(db.Delete(25, _bucket1, _k2))
	r.NoError(db.Put(25, _bucket1, _k2, _k2)) // write-after-delete
	v, err = db.Version(_bucket1, _k2)
	r.NoError(err)
	r.EqualValues(25, v)
	for _, e := range []versionTest{
		{_bucket1, _k2, nil, 0, ErrNotExist},
		{_bucket1, _k2, _v1, 1, nil},
		{_bucket1, _k2, _v1, 2, nil},
		{_bucket1, _k2, _v3, 3, nil},
		{_bucket1, _k2, _v3, 6, nil},
		{_bucket1, _k2, nil, 7, ErrDeleted},
		{_bucket1, _k2, nil, 9, ErrDeleted},
		{_bucket1, _k2, _v2, 10, nil},
		{_bucket1, _k2, _v2, 14, nil},
		{_bucket1, _k2, nil, 15, ErrDeleted},
		{_bucket1, _k2, nil, 17, ErrDeleted},
		{_bucket1, _k2, nil, 18, ErrDeleted},
		{_bucket1, _k2, nil, 20, ErrDeleted},
		{_bucket1, _k2, _v4, 21, nil},
		{_bucket1, _k2, _v4, 22, nil},
		{_bucket1, _k2, _v4, 24, nil},
		{_bucket1, _k2, _k2, 25, nil},
		{_bucket1, _k2, _k2, 26, nil},
		{_bucket1, _k2, _k2, 25000, nil},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
}
