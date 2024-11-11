// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"math"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

var (
	_k5  = []byte("key_5")
	_k10 = []byte("key_10")
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
	for i := 0; i < 2; i++ {
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

		if i == 0 {
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
			r.NoError(db.Delete(18, _bucket1, _k2))
			r.NoError(db.Put(18, _bucket1, _k2, _v3))
			r.NoError(db.Delete(18, _bucket1, _k2)) // delete-after-write
			_, err = db.Version(_bucket1, _k2)
			r.Equal(ErrDeleted, errors.Cause(err))
			r.NoError(db.Put(21, _bucket1, _k2, _v4))
			v, err = db.Version(_bucket1, _k2)
			r.NoError(err)
			r.EqualValues(21, v)
			r.NoError(db.Delete(25, _bucket1, _k2))
			r.NoError(db.Put(25, _bucket1, _k2, _k2))
			r.NoError(db.Delete(25, _bucket1, _k2))
			r.NoError(db.Put(25, _bucket1, _k2, _k2)) // write-after-delete
			v, err = db.Version(_bucket1, _k2)
			r.NoError(err)
			r.EqualValues(25, v)
		} else {
			// multiple writes and deletes using commitToDB
			b := batch.NewBatch()
			for _, e := range []versionTest{
				{_bucket1, _k2, _v1, 1, nil},
				{_bucket1, _k2, _v3, 3, nil},
				{_bucket1, _k2, nil, 7, ErrDeleted},
				{_bucket1, _k2, _v2, 10, nil},
				{_bucket1, _k2, nil, 15, ErrDeleted},
				{_bucket1, _k2, _v3, 18, ErrDeleted}, // delete-after-write
				{_bucket1, _k2, _v4, 21, nil},
				{_bucket1, _k2, _k2, 25, nil}, // write-after-delete
			} {
				if e.height == 7 || e.height == 15 {
					b.Delete(e.ns, e.k, "test")
				} else if e.height == 18 {
					b.Put(e.ns, e.k, e.v, "test")
					b.Delete(e.ns, e.k, "test")
					b.Put(e.ns, e.k, e.v, "test")
					b.Delete(e.ns, e.k, "test")
				} else if e.height == 25 {
					b.Delete(e.ns, e.k, "test")
					b.Put(e.ns, e.k, e.v, "test")
					b.Delete(e.ns, e.k, "test")
					b.Put(e.ns, e.k, e.v, "test")
				} else {
					b.Put(e.ns, e.k, e.v, "test")
				}
				r.NoError(db.CommitToDB(e.height, nil, b))
				b.Clear()
				v, err := db.Version(e.ns, e.k)
				r.Equal(e.err, errors.Cause(err))
				if err == nil {
					r.EqualValues(e.height, v)
				}
			}
		}
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
		r.NoError(db.Stop(ctx))
	}
}

func TestDedup(t *testing.T) {
	r := require.New(t)

	b := batch.NewBatch()
	for _, e := range []versionTest{
		{_bucket2, _v1, _v2, 0, nil},
		{_bucket2, _v2, _v3, 9, nil},
		{_bucket2, _v3, _v4, 3, nil},
		{_bucket2, _v4, _v1, 1, nil},
		{_bucket1, _k1, _v1, 0, nil},
		{_bucket1, _k2, _v2, 9, nil},
		{_bucket1, _k3, _v3, 3, nil},
		{_bucket1, _k4, _v4, 1, nil},
	} {
		b.Put(e.ns, e.k, e.v, "test")
	}
	keySize, ve, ce, err := dedup(nil, b)
	r.NoError(err)
	r.Equal(2, len(keySize))
	r.Equal(5, keySize[_bucket1])
	r.Equal(7, keySize[_bucket2])
	r.Equal(8, len(ve))
	r.Zero(len(ce))
	for i, v := range [][]byte{_k4, _k3, _k2, _k1, _v4, _v3, _v2, _v1} {
		r.Equal(v, ve[i].Key())
	}
	// put a key with diff length into _bucket2
	b.Put(_bucket2, _k1, _v1, "test")
	// treat _bucket1 as versioned namespace still OK
	keySize, ve, ce, err = dedup(map[string]bool{
		_bucket1: true,
	}, b)
	r.NoError(err)
	r.Equal(1, len(keySize))
	r.Equal(5, keySize[_bucket1])
	r.Equal(4, len(ve))
	r.Equal(5, len(ce))
	for i, v := range [][]byte{_k4, _k3, _k2, _k1} {
		r.Equal(v, ve[i].Key())
	}
	for i, v := range [][]byte{_k1, _v4, _v3, _v2, _v1} {
		r.Equal(v, ce[i].Key())
	}
	// treat _bucket2 (or both buckets) as versioned namespace hits error due to diff key size
	for _, v := range []map[string]bool{
		{_bucket2: true}, nil,
	} {
		_, _, _, err = dedup(v, b)
		r.Equal("invalid key length, expecting 5, got 7: invalid input", err.Error())
	}
}

func TestCommitToDB(t *testing.T) {
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

	b := batch.NewBatch()
	for _, e := range []versionTest{
		{_bucket2, _v1, _k1, 0, nil},
		{_bucket2, _v2, _k2, 9, nil},
		{_bucket2, _v3, _k3, 3, nil},
		{_bucket1, _k1, _v1, 0, nil},
		{_bucket1, _k2, _v2, 9, nil},
	} {
		b.Put(e.ns, e.k, e.v, "test")
	}

	r.NoError(db.CommitToDB(1, nil, b))
	b.Clear()
	for _, e := range []versionTest{
		{_bucket2, _v1, nil, 0, ErrNotExist},
		{_bucket2, _v2, nil, 0, ErrNotExist},
		{_bucket2, _v3, nil, 0, ErrNotExist},
		{_bucket2, _v4, nil, 0, ErrNotExist},
		{_bucket2, _v1, _k1, 1, nil},
		{_bucket2, _v2, _k2, 1, nil},
		{_bucket2, _v3, _k3, 1, nil},
		{_bucket2, _v1, _k1, 2, nil},
		{_bucket2, _v2, _k2, 2, nil},
		{_bucket2, _v3, _k3, 2, nil},
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, nil, 0, ErrNotExist},
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 0, ErrNotExist},
		{_bucket1, _k1, _v1, 1, nil},
		{_bucket1, _k2, _v2, 1, nil},
		{_bucket1, _k1, _v1, 3, nil},
		{_bucket1, _k2, _v2, 3, nil},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}

	// batch with wrong key length would fail
	b.Put(_bucket1, _v1, _k1, "test")
	r.Equal(ErrInvalid, errors.Cause(db.CommitToDB(3, nil, b)))
	b.Clear()
	for _, e := range []versionTest{
		{_bucket1, _k1, _v1, 0, nil},
		{_bucket1, _k2, _v3, 9, nil},
		{_bucket1, _k3, _v1, 3, nil},
		{_bucket1, _k4, _v2, 1, nil},
		{_bucket2, _v1, _k3, 0, nil},
		{_bucket2, _v2, _k2, 9, nil},
		{_bucket2, _v3, _k1, 3, nil},
		{_bucket2, _v4, _k4, 1, nil},
	} {
		b.Put(e.ns, e.k, e.v, "test")
	}
	b.Delete(_bucket1, _k3, "test")
	b.Delete(_bucket2, _v3, "test")

	r.NoError(db.CommitToDB(5, nil, b))
	b.Clear()
	for _, e := range []versionTest{
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, nil, 0, ErrNotExist},
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 0, ErrNotExist},
		{_bucket2, _v1, nil, 0, ErrNotExist},
		{_bucket2, _v2, nil, 0, ErrNotExist},
		{_bucket2, _v3, nil, 0, ErrNotExist},
		{_bucket2, _v4, nil, 0, ErrNotExist},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	for _, e := range []versionTest{
		{_bucket1, _k1, _v1, 1, nil},
		{_bucket1, _k2, _v2, 1, nil},
		{_bucket1, _k3, nil, 1, ErrNotExist},
		{_bucket1, _k4, nil, 1, ErrNotExist},
		{_bucket2, _v1, _k1, 1, nil},
		{_bucket2, _v2, _k2, 1, nil},
		{_bucket2, _v3, _k3, 1, nil},
		{_bucket2, _v4, nil, 1, ErrNotExist},
	} {
		for _, h := range []uint64{1, 2, 3, 4} {
			value, err := db.Get(h, e.ns, e.k)
			r.Equal(e.err, errors.Cause(err))
			r.Equal(e.v, value)
		}
	}
	for _, e := range []versionTest{
		{_bucket1, _k1, _v1, 5, nil},
		{_bucket1, _k2, _v3, 5, nil},
		{_bucket1, _k3, nil, 5, ErrDeleted},
		{_bucket1, _k4, _v2, 5, nil},
		{_bucket2, _v1, _k3, 5, nil},
		{_bucket2, _v2, _k2, 5, nil},
		{_bucket2, _v3, nil, 5, ErrDeleted},
		{_bucket2, _v4, _k4, 5, nil},
	} {
		for _, h := range []uint64{5, 16, 64, 3000, math.MaxUint64} {
			value, err := db.Get(h, e.ns, e.k)
			r.Equal(e.err, errors.Cause(err))
			r.Equal(e.v, value)
		}
	}
}
