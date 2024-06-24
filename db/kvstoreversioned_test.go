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

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestKVStoreWithVersion(t *testing.T) {
	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-kversion")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := DefaultConfig
	cfg.DbPath = testPath
	db := NewKVStoreWithVersion(cfg, VersionedNamespaceOption(_bucket1, _bucket2))
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	// write first key
	r.NoError(db.Put(_bucket1, _k2, _v2))
	v, err := db.Get(_bucket1, _k2)
	r.NoError(err)
	r.Equal(_v2, v)
	n, err := db.Version(_bucket1, _k2)
	r.NoError(err)
	r.Zero(n)
	// check more Put/Get
	err = db.SetVersion(1).Put(_bucket1, _k10, _v1)
	r.Equal("invalid key length, expecting 5, got 6: invalid input", err.Error())
	r.NoError(db.SetVersion(1).Put(_bucket1, _k2, _v1))
	r.NoError(db.SetVersion(1).Put(_bucket2, _k2, _v2))
	r.NoError(db.SetVersion(3).Put(_bucket1, _k2, _v3))
	r.NoError(db.SetVersion(3).Put(_bucket2, _k2, _v1))
	r.NoError(db.SetVersion(6).Put(_bucket1, _k2, _v2))
	r.NoError(db.SetVersion(6).Put(_bucket2, _k2, _v3))
	r.NoError(db.SetVersion(2).Put(_bucket1, _k4, _v2))
	r.NoError(db.SetVersion(4).Put(_bucket1, _k4, _v1))
	r.NoError(db.SetVersion(7).Put(_bucket1, _k4, _v3))
	// non-versioned namespace
	r.NoError(db.Put(_ns, _k1, _v1))
	r.NoError(db.Put(_ns, _k2, _v2))
	for _, e := range []versionTest{
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
		// bucket2
		{_bucket2, _k1, nil, 0, ErrNotExist},
		{_bucket2, _k2, nil, 0, ErrNotExist}, // before first write version
		{_bucket2, _k2, _v2, 1, nil},
		{_bucket2, _k2, _v2, 2, nil},
		{_bucket2, _k2, _v1, 3, nil},
		{_bucket2, _k2, _v1, 5, nil},
		{_bucket2, _k2, _v3, 6, nil},
		{_bucket2, _k2, _v3, 7, nil}, // after last write version
		{_bucket2, _k3, nil, 0, ErrNotExist},
		{_bucket2, _k4, nil, 1, ErrNotExist},
		{_bucket2, _k5, nil, 0, ErrNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid},
		// non-versioned namespace
		{_ns, _k1, _v1, 0, nil},
		{_ns, _k2, _v2, 0, nil},
		{_ns, _k3, nil, 0, ErrNotExist},
		{_ns, _k4, nil, 0, ErrNotExist},
		{_ns, _k5, nil, 0, ErrNotExist},
		{_ns, _k10, nil, 0, ErrNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// overwrite the same height again
	r.NoError(db.SetVersion(6).Put(_bucket1, _k2, _v4))
	r.NoError(db.SetVersion(6).Put(_bucket2, _k2, _v4))
	r.NoError(db.SetVersion(7).Put(_bucket1, _k4, _v4))
	// write to earlier version again does nothing
	r.NoError(db.SetVersion(3).Put(_bucket1, _k2, _v4))
	r.NoError(db.SetVersion(4).Put(_bucket1, _k4, _v4))
	// write with same value
	r.NoError(db.SetVersion(9).Put(_bucket1, _k2, _v4))
	r.NoError(db.SetVersion(10).Put(_bucket1, _k4, _v4))
	for _, e := range []versionTest{
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
		// bucket2
		{_bucket2, _k1, nil, 0, ErrNotExist},
		{_bucket2, _k2, nil, 0, ErrNotExist}, // before first write version
		{_bucket2, _k2, _v2, 1, nil},
		{_bucket2, _k2, _v2, 2, nil},
		{_bucket2, _k2, _v1, 3, nil},
		{_bucket2, _k2, _v1, 5, nil},
		{_bucket2, _k2, _v4, 6, nil},
		{_bucket2, _k2, _v4, 7, nil}, // after last write version
		{_bucket2, _k3, nil, 0, ErrNotExist},
		{_bucket2, _k4, nil, 1, ErrNotExist},
		{_bucket2, _k5, nil, 0, ErrNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid},
		// non-versioned namespace
		{_ns, _k1, _v1, 0, nil},
		{_ns, _k2, _v2, 0, nil},
		{_ns, _k3, nil, 0, ErrNotExist},
		{_ns, _k4, nil, 0, ErrNotExist},
		{_ns, _k5, nil, 0, ErrNotExist},
		{_ns, _k10, nil, 0, ErrNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
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
		// bucket2
		{_bucket2, _k1, nil, 0, ErrNotExist},
		{_bucket2, _k2, nil, 6, nil},
		{_bucket2, _k3, nil, 0, ErrNotExist},
		{_bucket2, _k4, nil, 0, ErrNotExist},
		{_bucket2, _k5, nil, 0, ErrNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid},
		// non-versioned namespace
		{_ns, _k1, nil, 0, _nerr},
		{_ns, _k2, nil, 0, _nerr},
		{_ns, _k3, nil, 0, _nerr},
		{_ns, _k4, nil, 0, _nerr},
		{_ns, _k5, nil, 0, _nerr},
		{_ns, _k10, nil, 0, _nerr},
	} {
		value, err := db.Version(e.ns, e.k)
		if e.ns == _ns {
			r.Equal(e.err.Error(), err.Error())
		} else {
			r.Equal(e.err, errors.Cause(err))
			r.Equal(e.height, value)
		}
	}
	// test delete
	kv := db.SetVersion(10)
	r.NoError(kv.Delete(_bucket2, _k1))
	for _, k := range [][]byte{_k1, _k2, _k3, _k4, _k5, _k10} {
		r.NoError(kv.Delete(_bucket1, k))
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
		// bucket2
		{_bucket2, _k1, nil, 0, ErrNotExist},
		{_bucket2, _k2, nil, 0, ErrNotExist}, // before first write version
		{_bucket2, _k2, _v2, 1, nil},
		{_bucket2, _k2, _v2, 2, nil},
		{_bucket2, _k2, _v1, 3, nil},
		{_bucket2, _k2, _v1, 5, nil},
		{_bucket2, _k2, _v4, 6, nil},
		{_bucket2, _k2, _v4, 7, nil}, // after last write version
		{_bucket2, _k3, nil, 0, ErrNotExist},
		{_bucket2, _k4, nil, 1, ErrNotExist},
		{_bucket2, _k5, nil, 0, ErrNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid},
		// non-versioned namespace
		{_ns, _k1, _v1, 0, nil},
		{_ns, _k2, _v2, 0, nil},
		{_ns, _k3, nil, 0, ErrNotExist},
		{_ns, _k4, nil, 0, ErrNotExist},
		{_ns, _k5, nil, 0, ErrNotExist},
		{_ns, _k10, nil, 0, ErrNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// write before delete version does nothing
	r.NoError(db.SetVersion(9).Put(_bucket1, _k2, _k2))
	r.NoError(db.SetVersion(9).Put(_bucket1, _k4, _k4))
	for _, e := range []versionTest{
		{_bucket1, _k2, _v4, 9, nil},         // before delete version
		{_bucket1, _k2, nil, 10, ErrDeleted}, // after delete version
		{_bucket1, _k4, _v4, 9, nil},         // before delete version
		{_bucket1, _k4, nil, 10, ErrDeleted}, // after delete version
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// write after delete version
	r.NoError(db.SetVersion(10).Put(_bucket1, _k2, _k2))
	r.NoError(db.SetVersion(10).Put(_bucket1, _k4, _k4))
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
		// bucket2
		{_bucket2, _k1, nil, 0, ErrNotExist},
		{_bucket2, _k2, nil, 0, ErrNotExist}, // before first write version
		{_bucket2, _k2, _v2, 1, nil},
		{_bucket2, _k2, _v2, 2, nil},
		{_bucket2, _k2, _v1, 3, nil},
		{_bucket2, _k2, _v1, 5, nil},
		{_bucket2, _k2, _v4, 6, nil},
		{_bucket2, _k2, _v4, 7, nil}, // after last write version
		{_bucket2, _k3, nil, 0, ErrNotExist},
		{_bucket2, _k4, nil, 1, ErrNotExist},
		{_bucket2, _k5, nil, 0, ErrNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid},
		// non-versioned namespace
		{_ns, _k1, _v1, 0, nil},
		{_ns, _k2, _v2, 0, nil},
		{_ns, _k3, nil, 0, ErrNotExist},
		{_ns, _k4, nil, 0, ErrNotExist},
		{_ns, _k5, nil, 0, ErrNotExist},
		{_ns, _k10, nil, 0, ErrNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	// check version
	for _, e := range []versionTest{
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, _k2, 10, nil},
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, _k4, 10, nil},
		{_bucket1, _k5, nil, 0, ErrNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid},
		// bucket2
		{_bucket2, _k1, nil, 0, ErrNotExist},
		{_bucket2, _k2, nil, 6, nil},
		{_bucket2, _k3, nil, 0, ErrNotExist},
		{_bucket2, _k4, nil, 0, ErrNotExist},
		{_bucket2, _k5, nil, 0, ErrNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid},
		// non-versioned namespace
		{_ns, _k1, nil, 0, _nerr},
		{_ns, _k2, nil, 0, _nerr},
		{_ns, _k3, nil, 0, _nerr},
		{_ns, _k4, nil, 0, _nerr},
		{_ns, _k5, nil, 0, _nerr},
		{_ns, _k10, nil, 0, _nerr},
	} {
		value, err := db.Version(e.ns, e.k)
		if e.ns == _ns {
			r.Equal(e.err.Error(), err.Error())
		} else {
			r.Equal(e.err, errors.Cause(err))
			r.Equal(e.height, value)
		}
	}
}

func TestWriteBatch(t *testing.T) {
	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-version")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := DefaultConfig
	cfg.DbPath = testPath
	db := NewKVStoreWithVersion(cfg, VersionedNamespaceOption(_bucket1, _bucket2))
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

	r.NoError(db.SetVersion(1).WriteBatch(b))
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
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}

	// batch with wrong key length would fail
	b.Put(_bucket1, _v1, _k1, "test")
	r.Equal(ErrInvalid, errors.Cause(db.SetVersion(3).WriteBatch(b)))
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
		// non-versioned namespace
		{_ns, _k1, _v1, 1, nil},
		{_ns, _k2, _v2, 1, nil},
		{_ns, _v3, _k3, 1, nil},
		{_ns, _v4, _k4, 1, nil},
	} {
		b.Put(e.ns, e.k, e.v, "test")
	}
	b.Delete(_bucket1, _k3, "test")
	b.Delete(_bucket2, _v3, "test")
	b.Delete(_ns, _v3, "test")

	r.NoError(db.SetVersion(5).WriteBatch(b))
	b.Clear()
	for _, e := range []versionTest{
		{_bucket2, _v1, nil, 0, ErrNotExist},
		{_bucket2, _v2, nil, 0, ErrNotExist},
		{_bucket2, _v3, nil, 0, ErrNotExist},
		{_bucket2, _v4, nil, 0, ErrNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	for _, e := range []versionTest{
		{_bucket2, _v1, _k1, 1, nil},
		{_bucket2, _v2, _k2, 1, nil},
		{_bucket2, _v3, _k3, 1, nil},
		{_bucket2, _v4, nil, 1, ErrNotExist},
	} {
		for _, h := range []uint64{1, 2, 3, 4} {
			value, err := db.SetVersion(h).Get(e.ns, e.k)
			r.Equal(e.err, errors.Cause(err))
			r.Equal(e.v, value)
		}
	}
	for _, e := range []versionTest{
		{_bucket2, _v1, _k3, 5, nil},
		{_bucket2, _v2, _k2, 5, nil},
		{_bucket2, _v3, nil, 5, ErrDeleted},
		{_bucket2, _v4, _k4, 5, nil},
	} {
		for _, h := range []uint64{5, 16, 3000, math.MaxUint64} {
			value, err := db.SetVersion(h).Get(e.ns, e.k)
			r.Equal(e.err, errors.Cause(err))
			r.Equal(e.v, value)
		}
	}
	for _, e := range []versionTest{
		{_bucket1, _k1, nil, 0, ErrNotExist},
		{_bucket1, _k2, nil, 0, ErrNotExist},
		{_bucket1, _k3, nil, 0, ErrNotExist},
		{_bucket1, _k4, nil, 0, ErrNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
	for _, e := range []versionTest{
		{_bucket1, _k1, _v1, 1, nil},
		{_bucket1, _k2, _v2, 1, nil},
		{_bucket1, _k3, nil, 1, ErrNotExist},
		{_bucket1, _k4, nil, 1, ErrNotExist},
	} {
		for _, h := range []uint64{1, 2, 3, 4} {
			value, err := db.SetVersion(h).Get(e.ns, e.k)
			r.Equal(e.err, errors.Cause(err))
			r.Equal(e.v, value)
		}
	}
	for _, e := range []versionTest{
		{_bucket1, _k1, _v1, 5, nil},
		{_bucket1, _k2, _v3, 5, nil},
		{_bucket1, _k3, nil, 5, ErrDeleted},
		{_bucket1, _k4, _v2, 5, nil},
	} {
		for _, h := range []uint64{5, 64, 300, math.MaxUint64} {
			value, err := db.SetVersion(h).Get(e.ns, e.k)
			r.Equal(e.err, errors.Cause(err))
			r.Equal(e.v, value)
		}
	}
	// non-versioned namespace
	for _, e := range []versionTest{
		{_ns, _k1, _v1, 1, nil},
		{_ns, _k2, _v2, 1, nil},
		{_ns, _v3, nil, 1, ErrNotExist},
		{_ns, _v4, _k4, 1, nil},
	} {
		value, err := db.Get(e.ns, e.k)
		r.Equal(e.err, errors.Cause(err))
		r.Equal(e.v, value)
	}
}
