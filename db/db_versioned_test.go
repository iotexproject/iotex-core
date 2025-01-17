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
	_k5          = []byte("key_5")
	_k10         = []byte("key_10")
	_errNotExist = ErrNotExist.Error()
	_errDeleted  = "deleted: not exist in DB"
)

type versionTest struct {
	ns     string
	k, v   []byte
	height uint64
	err    string
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
	db := NewBoltDBVersioned(cfg, VnsOption(Namespace{_bucket1, uint32(len(_k2))}))
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	// namespace created
	vn, err := db.checkNamespace(_bucket1)
	r.NoError(err)
	r.Equal(uint32(len(_k2)), vn.keyLen)
	// write first key
	r.NoError(db.Put(0, _bucket1, _k2, _v2))
	vn, err = db.checkNamespace(_bucket1)
	r.NoError(err)
	r.EqualValues(len(_k2), vn.keyLen)
	// check more Put/Get
	err = db.Put(1, _bucket1, _k10, _v1)
	r.ErrorContains(err, "invalid key length, expecting 5, got 6: invalid input")
	r.NoError(db.Put(1, _bucket1, _k2, _v1))
	r.NoError(db.Put(3, _bucket1, _k2, _v3))
	r.NoError(db.Put(6, _bucket1, _k2, _v2))
	r.NoError(db.Put(2, _bucket1, _k4, _v2))
	r.NoError(db.Put(4, _bucket1, _k4, _v1))
	r.NoError(db.Put(7, _bucket1, _k4, _v3))
	for _, e := range []versionTest{
		{_bucket2, _k1, nil, 0, _errNotExist}, // bucket not exist
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, _v2, 0, ""},
		{_bucket1, _k2, _v1, 1, ""},
		{_bucket1, _k2, _v1, 2, ""},
		{_bucket1, _k2, _v3, 3, ""},
		{_bucket1, _k2, _v3, 5, ""},
		{_bucket1, _k2, _v2, 6, ""},
		{_bucket1, _k2, _v2, 7, ""}, // after last write version
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 1, _errNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, ""},
		{_bucket1, _k4, _v2, 3, ""},
		{_bucket1, _k4, _v1, 4, ""},
		{_bucket1, _k4, _v1, 6, ""},
		{_bucket1, _k4, _v3, 7, ""},
		{_bucket1, _k4, _v3, 8, ""}, // larger than last key in bucket
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	// overwrite the same height again
	r.NoError(db.Put(6, _bucket1, _k2, _v4))
	r.NoError(db.Put(7, _bucket1, _k4, _v4))
	// write to earlier version again is invalid
	r.ErrorContains(db.Put(3, _bucket1, _k2, _v4), "cannot write at earlier version 3: invalid input")
	r.ErrorContains(db.Put(4, _bucket1, _k4, _v4), "cannot write at earlier version 4: invalid input")
	// write with same value
	r.NoError(db.Put(9, _bucket1, _k2, _v4))
	r.NoError(db.Put(10, _bucket1, _k4, _v4))
	for _, e := range []versionTest{
		{_bucket2, _k1, nil, 0, _errNotExist}, // bucket not exist
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, _v2, 0, ""},
		{_bucket1, _k2, _v1, 1, ""},
		{_bucket1, _k2, _v1, 2, ""},
		{_bucket1, _k2, _v3, 3, ""},
		{_bucket1, _k2, _v3, 5, ""},
		{_bucket1, _k2, _v4, 6, ""},
		{_bucket1, _k2, _v4, 8, ""},
		{_bucket1, _k2, _v4, 9, ""},
		{_bucket1, _k2, _v4, 10, ""}, // after last write version
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 1, _errNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, ""},
		{_bucket1, _k4, _v2, 3, ""},
		{_bucket1, _k4, _v1, 4, ""},
		{_bucket1, _k4, _v1, 6, ""},
		{_bucket1, _k4, _v4, 7, ""},
		{_bucket1, _k4, _v4, 9, ""},
		{_bucket1, _k4, _v4, 10, ""},
		{_bucket1, _k4, _v4, 11, ""}, // larger than last key in bucket
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	// check version
	for _, e := range []versionTest{
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, nil, 9, ""},
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 10, ""},
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
	} {
		value, err := db.Version(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.height, value)
	}
	v, err := db.Version(_bucket2, _k1)
	r.Zero(v)
	r.ErrorContains(err, "namespace test_ns2 is non-versioned")
	// test delete
	for _, k := range [][]byte{_k2, _k4} {
		r.NoError(db.Delete(11, _bucket1, k))
		r.ErrorContains(db.Delete(10, _bucket1, k), "cannot delete at earlier version 10: invalid input")
	}
	for _, k := range [][]byte{_k1, _k3, _k5} {
		r.NoError(db.Delete(10, _bucket1, k))
	}
	r.Equal(ErrInvalid, errors.Cause(db.Delete(10, _bucket1, _k10)))
	// key still can be read before delete version
	for _, e := range []versionTest{
		{_bucket2, _k1, nil, 0, _errNotExist}, // bucket not exist
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, _v2, 0, ""},
		{_bucket1, _k2, _v1, 1, ""},
		{_bucket1, _k2, _v1, 2, ""},
		{_bucket1, _k2, _v3, 3, ""},
		{_bucket1, _k2, _v3, 5, ""},
		{_bucket1, _k2, _v4, 6, ""},
		{_bucket1, _k2, _v4, 8, ""},
		{_bucket1, _k2, _v4, 9, ""},
		{_bucket1, _k2, _v4, 10, ""},          // before delete version
		{_bucket1, _k2, nil, 11, _errDeleted}, // after delete version
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 1, _errNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, ""},
		{_bucket1, _k4, _v2, 3, ""},
		{_bucket1, _k4, _v1, 4, ""},
		{_bucket1, _k4, _v1, 6, ""},
		{_bucket1, _k4, _v4, 7, ""},
		{_bucket1, _k4, _v4, 10, ""},          // before delete version
		{_bucket1, _k4, nil, 11, _errDeleted}, // after delete version
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	// write before delete version is invalid
	r.ErrorContains(db.Put(9, _bucket1, _k2, _k2), "cannot write at earlier version 9: invalid input")
	r.ErrorContains(db.Put(9, _bucket1, _k4, _k4), "cannot write at earlier version 9: invalid input")
	for _, e := range []versionTest{
		{_bucket1, _k2, _v4, 10, ""},          // before delete version
		{_bucket1, _k2, nil, 11, _errDeleted}, // after delete version
		{_bucket1, _k4, _v4, 10, ""},          // before delete version
		{_bucket1, _k4, nil, 11, _errDeleted}, // after delete version
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	// write after delete version
	r.NoError(db.Put(12, _bucket1, _k2, _k2))
	r.NoError(db.Put(12, _bucket1, _k4, _k4))
	for _, e := range []versionTest{
		{_bucket2, _k1, nil, 0, _errNotExist}, // bucket not exist
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, _v2, 0, ""},
		{_bucket1, _k2, _v1, 1, ""},
		{_bucket1, _k2, _v1, 2, ""},
		{_bucket1, _k2, _v3, 3, ""},
		{_bucket1, _k2, _v3, 5, ""},
		{_bucket1, _k2, _v4, 6, ""},
		{_bucket1, _k2, _v4, 8, ""},
		{_bucket1, _k2, _v4, 10, ""},          // before delete version
		{_bucket1, _k2, nil, 11, _errDeleted}, // after delete version
		{_bucket1, _k2, _k2, 12, ""},          // after next write version
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 1, _errNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, ""},
		{_bucket1, _k4, _v2, 3, ""},
		{_bucket1, _k4, _v1, 4, ""},
		{_bucket1, _k4, _v1, 6, ""},
		{_bucket1, _k4, _v4, 7, ""},
		{_bucket1, _k4, _v4, 10, ""},          // before delete version
		{_bucket1, _k4, nil, 11, _errDeleted}, // after delete version
		{_bucket1, _k4, _k4, 12, ""},          // after next write version
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	// check version after delete
	for _, e := range []versionTest{
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, nil, 12, ""},
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 12, ""},
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
	} {
		value, err := db.Version(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.height, value)
	}
	// test filter
	r.PanicsWithValue("Filter not supported for versioned DB", func() { db.Filter(0, _bucket1, nil, nil, nil) })
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
		db := NewBoltDBVersioned(cfg, VnsOption(Namespace{_bucket1, uint32(len(_k2))}))
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
				{_bucket1, _k2, _v1, 1, ""},
				{_bucket1, _k2, _v3, 3, ""},
				{_bucket1, _k2, nil, 7, ErrDeleted.Error()},
				{_bucket1, _k2, _v2, 10, ""},
				{_bucket1, _k2, nil, 15, ErrDeleted.Error()},
				{_bucket1, _k2, _v3, 18, ErrDeleted.Error()}, // delete-after-write
				{_bucket1, _k2, _v4, 21, ""},
				{_bucket1, _k2, _k2, 25, ""}, // write-after-delete
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
				r.NoError(db.CommitBatch(e.height, b))
				b.Clear()
				v, err := db.Version(e.ns, e.k)
				if len(e.err) == 0 {
					r.NoError(err)
				} else {
					r.ErrorContains(err, e.err)
				}
				if err == nil {
					r.EqualValues(e.height, v)
				}
			}
		}
		for _, e := range []versionTest{
			{_bucket1, _k2, nil, 0, _errNotExist},
			{_bucket1, _k2, _v1, 1, ""},
			{_bucket1, _k2, _v1, 2, ""},
			{_bucket1, _k2, _v3, 3, ""},
			{_bucket1, _k2, _v3, 6, ""},
			{_bucket1, _k2, nil, 7, _errDeleted},
			{_bucket1, _k2, nil, 9, _errDeleted},
			{_bucket1, _k2, _v2, 10, ""},
			{_bucket1, _k2, _v2, 14, ""},
			{_bucket1, _k2, nil, 15, _errDeleted},
			{_bucket1, _k2, nil, 17, _errDeleted},
			{_bucket1, _k2, nil, 18, _errDeleted},
			{_bucket1, _k2, nil, 20, _errDeleted},
			{_bucket1, _k2, _v4, 21, ""},
			{_bucket1, _k2, _v4, 22, ""},
			{_bucket1, _k2, _v4, 24, ""},
			{_bucket1, _k2, _k2, 25, ""},
			{_bucket1, _k2, _k2, 26, ""},
			{_bucket1, _k2, _k2, 25000, ""},
		} {
			value, err := db.Get(e.height, e.ns, e.k)
			if len(e.err) == 0 {
				r.NoError(err)
			} else {
				r.ErrorContains(err, e.err)
			}
			r.Equal(e.v, value)
		}
		r.NoError(db.Stop(ctx))
	}
}

func TestDedup(t *testing.T) {
	r := require.New(t)

	b := batch.NewBatch()
	for _, e := range []versionTest{
		{_bucket2, _v1, _v2, 0, ""},
		{_bucket2, _v2, _v3, 9, ""},
		{_bucket2, _v3, _v4, 3, ""},
		{_bucket2, _v4, _v1, 1, ""},
		{_bucket1, _k1, _v1, 0, ""},
		{_bucket1, _k2, _v2, 9, ""},
		{_bucket1, _k3, _v3, 3, ""},
		{_bucket1, _k4, _v4, 1, ""},
	} {
		b.Put(e.ns, e.k, e.v, "test")
	}
	ve, ce, err := dedup(nil, b)
	r.NoError(err)
	r.Equal(8, len(ve))
	r.Zero(len(ce))
	for i, v := range [][]byte{_k4, _k3, _k2, _k1, _v4, _v3, _v2, _v1} {
		r.Equal(v, ve[i].Key())
	}
	// put a key with diff length into _bucket2
	b.Put(_bucket2, _k1, _v1, "test")
	// treat _bucket1 as versioned namespace still OK
	ve, ce, err = dedup(map[string]int{_bucket1: 5}, b)
	r.NoError(err)
	r.Equal(4, len(ve))
	r.Equal(5, len(ce))
	for i, v := range [][]byte{_k4, _k3, _k2, _k1} {
		r.Equal(v, ve[i].Key())
	}
	for i, v := range [][]byte{_k1, _v4, _v3, _v2, _v1} {
		r.Equal(v, ce[i].Key())
	}
	// treat _bucket2 (or both buckets) as versioned namespace hits error due to diff key size
	_, _, err = dedup(map[string]int{_bucket2: 7}, b)
	r.ErrorContains(err, "invalid key length, expecting 7, got 5: invalid input")
	_, _, err = dedup(nil, b)
	r.ErrorContains(err, "invalid key length, expecting 5, got 7: invalid input")
}

func TestCommitBatch(t *testing.T) {
	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-version")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := DefaultConfig
	cfg.DbPath = testPath
	db := NewBoltDBVersioned(cfg, VnsOption(
		Namespace{_bucket1, uint32(len(_k1))}, Namespace{_bucket2, uint32(len(_v1))}))
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	b := batch.NewBatch()
	for _, e := range []versionTest{
		{_bucket2, _v1, _k1, 0, ""},
		{_bucket2, _v2, _k2, 9, ""},
		{_bucket2, _v3, _k3, 3, ""},
		{_bucket1, _k1, _v1, 0, ""},
		{_bucket1, _k2, _v2, 9, ""},
	} {
		b.Put(e.ns, e.k, e.v, "test")
	}
	r.NoError(db.CommitBatch(1, b))
	b.Clear()
	for _, e := range []versionTest{
		{_bucket2, _v1, nil, 0, _errNotExist},
		{_bucket2, _v2, nil, 0, _errNotExist},
		{_bucket2, _v3, nil, 0, _errNotExist},
		{_bucket2, _v4, nil, 0, _errNotExist},
		{_bucket2, _v1, _k1, 1, ""},
		{_bucket2, _v2, _k2, 1, ""},
		{_bucket2, _v3, _k3, 1, ""},
		{_bucket2, _v1, _k1, 2, ""},
		{_bucket2, _v2, _k2, 2, ""},
		{_bucket2, _v3, _k3, 2, ""},
		{_bucket2, _v4, nil, 2, _errNotExist},
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, nil, 0, _errNotExist},
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 0, _errNotExist},
		{_bucket1, _k1, _v1, 1, ""},
		{_bucket1, _k2, _v2, 1, ""},
		{_bucket1, _k1, _v1, 3, ""},
		{_bucket1, _k2, _v2, 3, ""},
		{_bucket1, _k3, nil, 3, _errNotExist},
		{_bucket1, _k4, nil, 3, _errNotExist},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}

	// batch with wrong key length would fail
	b.Put(_bucket1, _v1, _k1, "test")
	r.Equal(ErrInvalid, errors.Cause(db.CommitBatch(3, b)))
	b.Clear()
	for _, e := range []versionTest{
		{_bucket1, _k1, _v1, 0, ""},
		{_bucket1, _k2, _v3, 9, ""},
		{_bucket1, _k3, _v1, 3, ""},
		{_bucket1, _k4, _v2, 1, ""},
		{_bucket2, _v1, _k3, 0, ""},
		{_bucket2, _v2, _k2, 9, ""},
		{_bucket2, _v3, _k1, 3, ""},
		{_bucket2, _v4, _k4, 1, ""},
	} {
		b.Put(e.ns, e.k, e.v, "test")
	}
	b.Delete(_bucket1, _k3, "test")
	b.Delete(_bucket2, _v3, "test")

	r.NoError(db.CommitBatch(5, b))
	b.Clear()
	for _, e := range []versionTest{
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, nil, 0, _errNotExist},
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 0, _errNotExist},
		{_bucket2, _v1, nil, 0, _errNotExist},
		{_bucket2, _v2, nil, 0, _errNotExist},
		{_bucket2, _v3, nil, 0, _errNotExist},
		{_bucket2, _v4, nil, 0, _errNotExist},
	} {
		value, err := db.Get(e.height, e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	for _, e := range []versionTest{
		{_bucket1, _k1, _v1, 1, ""},
		{_bucket1, _k2, _v2, 1, ""},
		{_bucket1, _k3, nil, 1, _errNotExist},
		{_bucket1, _k4, nil, 1, _errNotExist},
		{_bucket2, _v1, _k1, 1, ""},
		{_bucket2, _v2, _k2, 1, ""},
		{_bucket2, _v3, _k3, 1, ""},
		{_bucket2, _v4, nil, 1, _errNotExist},
	} {
		for _, h := range []uint64{1, 2, 3, 4} {
			value, err := db.Get(h, e.ns, e.k)
			if len(e.err) == 0 {
				r.NoError(err)
			} else {
				r.ErrorContains(err, e.err)
			}
			r.Equal(e.v, value)
		}
	}
	for _, e := range []versionTest{
		{_bucket1, _k1, _v1, 5, ""},
		{_bucket1, _k2, _v3, 5, ""},
		{_bucket1, _k3, nil, 5, _errDeleted},
		{_bucket1, _k4, _v2, 5, ""},
		{_bucket2, _v1, _k3, 5, ""},
		{_bucket2, _v2, _k2, 5, ""},
		{_bucket2, _v3, nil, 5, _errDeleted},
		{_bucket2, _v4, _k4, 5, ""},
	} {
		for _, h := range []uint64{5, 16, 64, 3000, math.MaxUint64} {
			value, err := db.Get(h, e.ns, e.k)
			if len(e.err) == 0 {
				r.NoError(err)
			} else {
				r.ErrorContains(err, e.err)
			}
			r.Equal(e.v, value)
		}
	}
	// cannot write to earlier version
	b.Put(_bucket1, _k1, _v2, "test")
	b.Put(_bucket1, _k2, _v1, "test")
	r.ErrorIs(db.CommitBatch(4, b), ErrInvalid)
}
