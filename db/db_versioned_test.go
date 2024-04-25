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

	"github.com/iotexproject/iotex-core/testutil"
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
	km, err := db.checkKey(_bucket1, _k1)
	r.Nil(km)
	r.Nil(err)
	// write first key, namespace and key now exist
	r.NoError(db.Put(0, _bucket1, _k2, _v2))
	vn, err = db.checkNamespace(_bucket1)
	r.NoError(err)
	r.EqualValues(len(_k2), vn.keyLen)
	km, err = db.checkKey(_bucket1, _k2)
	r.Zero(km.firstVersion)
	r.Zero(km.lastVersion)
	r.Zero(km.deleteVersion)
	r.Equal(_v2, km.lastWrite)
	r.NoError(err)
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
	// write to earlier version again does nothing
	r.NoError(db.Put(3, _bucket1, _k2, _v4))
	r.NoError(db.Put(4, _bucket1, _k4, _v4))
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
	r.NoError(db.Delete(10, _bucket2, _k1))
	for _, k := range [][]byte{_k1, _k2, _k3, _k4, _k5, _k10} {
		r.NoError(db.Delete(10, _bucket1, k))
	}
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
		{_bucket1, _k2, _v4, 9, nil},         // before delete version
		{_bucket1, _k2, nil, 10, ErrDeleted}, // after delete version
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 1, ErrNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, nil},
		{_bucket1, _k4, _v2, 3, nil},
		{_bucket1, _k4, _v1, 4, nil},
		{_bucket1, _k4, _v1, 6, nil},
		{_bucket1, _k4, _v4, 7, nil},
		{_bucket1, _k4, _v4, 9, nil},         // before delete version
		{_bucket1, _k4, nil, 10, ErrDeleted}, // after delete version
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// write before delete version does nothing
	r.NoError(db.Put(9, _bucket1, _k2, _k2))
	r.NoError(db.Put(9, _bucket1, _k4, _k4))
	for _, e := range []versionTest{
		{_bucket1, _k2, _v4, 9, nil},         // before delete version
		{_bucket1, _k2, nil, 10, ErrDeleted}, // after delete version
		{_bucket1, _k4, _v4, 9, nil},         // before delete version
		{_bucket1, _k4, nil, 10, ErrDeleted}, // after delete version
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// write after delete version
	r.NoError(db.Put(10, _bucket1, _k2, _k2))
	r.NoError(db.Put(10, _bucket1, _k4, _k4))
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
		{_bucket1, _k2, _v4, 9, nil},  // before delete version
		{_bucket1, _k2, _k2, 10, nil}, // after delete version
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 1, ErrNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, nil},
		{_bucket1, _k4, _v2, 3, nil},
		{_bucket1, _k4, _v1, 4, nil},
		{_bucket1, _k4, _v1, 6, nil},
		{_bucket1, _k4, _v4, 7, nil},
		{_bucket1, _k4, _v4, 9, nil},  // before delete version
		{_bucket1, _k4, _k4, 10, nil}, // after delete version
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
		{_bucket1, _k2, nil, 10, nil},
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 10, nil},
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
	} {
		value, err := db.Version(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.height, value)
	}
}
