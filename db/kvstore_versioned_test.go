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
	_ns              = "ns"
	_errNonVersioned = "namespace ns is non-versioned"
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
	db := NewKVStoreWithVersion(cfg)
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
	r.ErrorContains(err, "invalid key length, expecting 5, got 6: invalid input")
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
		// bucket2
		{_bucket2, _k1, nil, 0, _errNotExist},
		{_bucket2, _k2, nil, 0, _errNotExist}, // before first write version
		{_bucket2, _k2, _v2, 1, ""},
		{_bucket2, _k2, _v2, 2, ""},
		{_bucket2, _k2, _v1, 3, ""},
		{_bucket2, _k2, _v1, 5, ""},
		{_bucket2, _k2, _v3, 6, ""},
		{_bucket2, _k2, _v3, 7, ""}, // after last write version
		{_bucket2, _k3, nil, 0, _errNotExist},
		{_bucket2, _k4, nil, 1, _errNotExist},
		{_bucket2, _k5, nil, 0, _errNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid.Error()},
		// non-versioned namespace
		{_ns, _k1, _v1, 0, ""},
		{_ns, _k2, _v2, 0, ""},
		{_ns, _k3, nil, 0, _errNotExist},
		{_ns, _k4, nil, 0, _errNotExist},
		{_ns, _k5, nil, 0, _errNotExist},
		{_ns, _k10, nil, 0, _errNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	// overwrite the same height again
	r.NoError(db.SetVersion(6).Put(_bucket1, _k2, _v4))
	r.NoError(db.SetVersion(6).Put(_bucket2, _k2, _v4))
	r.NoError(db.SetVersion(7).Put(_bucket1, _k4, _v4))
	// write to earlier version again is invalid
	r.Equal(ErrInvalid, db.SetVersion(3).Put(_bucket1, _k2, _v4))
	r.Equal(ErrInvalid, db.SetVersion(4).Put(_bucket1, _k4, _v4))
	// write with same value
	r.NoError(db.SetVersion(9).Put(_bucket1, _k2, _v4))
	r.NoError(db.SetVersion(10).Put(_bucket1, _k4, _v4))
	for _, e := range []versionTest{
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
		// bucket2
		{_bucket2, _k1, nil, 0, _errNotExist},
		{_bucket2, _k2, nil, 0, _errNotExist}, // before first write version
		{_bucket2, _k2, _v2, 1, ""},
		{_bucket2, _k2, _v2, 2, ""},
		{_bucket2, _k2, _v1, 3, ""},
		{_bucket2, _k2, _v1, 5, ""},
		{_bucket2, _k2, _v4, 6, ""},
		{_bucket2, _k2, _v4, 7, ""}, // after last write version
		{_bucket2, _k3, nil, 0, _errNotExist},
		{_bucket2, _k4, nil, 1, _errNotExist},
		{_bucket2, _k5, nil, 0, _errNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid.Error()},
		// non-versioned namespace
		{_ns, _k1, _v1, 0, ""},
		{_ns, _k2, _v2, 0, ""},
		{_ns, _k3, nil, 0, _errNotExist},
		{_ns, _k4, nil, 0, _errNotExist},
		{_ns, _k5, nil, 0, _errNotExist},
		{_ns, _k10, nil, 0, _errNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
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
		// bucket2
		{_bucket2, _k1, nil, 0, _errNotExist},
		{_bucket2, _k2, nil, 6, ""},
		{_bucket2, _k3, nil, 0, _errNotExist},
		{_bucket2, _k4, nil, 0, _errNotExist},
		{_bucket2, _k5, nil, 0, _errNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid.Error()},
		// non-versioned namespace
		{_ns, _k1, nil, 0, _errNonVersioned},
		{_ns, _k2, nil, 0, _errNonVersioned},
		{_ns, _k3, nil, 0, _errNonVersioned},
		{_ns, _k4, nil, 0, _errNonVersioned},
		{_ns, _k5, nil, 0, _errNonVersioned},
		{_ns, _k10, nil, 0, _errNonVersioned},
	} {
		value, err := db.Version(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.height, value)
	}
	// test delete
	kv := db.SetVersion(10)
	r.Equal(ErrNotExist, errors.Cause(kv.Delete(_bucket2, _k1)))
	for _, k := range [][]byte{_k2, _k4} {
		r.NoError(kv.Delete(_bucket1, k))
	}
	for _, k := range [][]byte{_k1, _k3, _k5} {
		r.Equal(ErrNotExist, errors.Cause(kv.Delete(_bucket1, k)))
	}
	r.Equal(ErrInvalid, errors.Cause(kv.Delete(_bucket1, _k10)))
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
		{_bucket1, _k2, _v4, 9, ""},           // before delete version
		{_bucket1, _k2, nil, 10, _errDeleted}, // after delete version
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 1, _errNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, ""},
		{_bucket1, _k4, _v2, 3, ""},
		{_bucket1, _k4, _v1, 4, ""},
		{_bucket1, _k4, _v1, 6, ""},
		{_bucket1, _k4, _v4, 7, ""},
		{_bucket1, _k4, _v4, 9, ""},           // before delete version
		{_bucket1, _k4, nil, 10, _errDeleted}, // after delete version
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
		// bucket2
		{_bucket2, _k1, nil, 0, _errNotExist},
		{_bucket2, _k2, nil, 0, _errNotExist}, // before first write version
		{_bucket2, _k2, _v2, 1, ""},
		{_bucket2, _k2, _v2, 2, ""},
		{_bucket2, _k2, _v1, 3, ""},
		{_bucket2, _k2, _v1, 5, ""},
		{_bucket2, _k2, _v4, 6, ""},
		{_bucket2, _k2, _v4, 7, ""}, // after last write version
		{_bucket2, _k3, nil, 0, _errNotExist},
		{_bucket2, _k4, nil, 1, _errNotExist},
		{_bucket2, _k5, nil, 0, _errNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid.Error()},
		// non-versioned namespace
		{_ns, _k1, _v1, 0, ""},
		{_ns, _k2, _v2, 0, ""},
		{_ns, _k3, nil, 0, _errNotExist},
		{_ns, _k4, nil, 0, _errNotExist},
		{_ns, _k5, nil, 0, _errNotExist},
		{_ns, _k10, nil, 0, _errNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	// write before delete version is invalid
	r.Equal(ErrInvalid, errors.Cause(db.SetVersion(9).Put(_bucket1, _k2, _k2)))
	r.Equal(ErrInvalid, errors.Cause(db.SetVersion(9).Put(_bucket1, _k4, _k4)))
	for _, e := range []versionTest{
		{_bucket1, _k2, _v4, 9, ""},           // before delete version
		{_bucket1, _k2, nil, 10, _errDeleted}, // after delete version
		{_bucket1, _k4, _v4, 9, ""},           // before delete version
		{_bucket1, _k4, nil, 10, _errDeleted}, // after delete version
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
	// write after delete version
	r.NoError(db.SetVersion(10).Put(_bucket1, _k2, _k2))
	r.NoError(db.SetVersion(10).Put(_bucket1, _k4, _k4))
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
		{_bucket1, _k2, _v4, 9, ""},  // before delete version
		{_bucket1, _k2, _k2, 10, ""}, // after delete version
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 1, _errNotExist}, // before first write version
		{_bucket1, _k4, _v2, 2, ""},
		{_bucket1, _k4, _v2, 3, ""},
		{_bucket1, _k4, _v1, 4, ""},
		{_bucket1, _k4, _v1, 6, ""},
		{_bucket1, _k4, _v4, 7, ""},
		{_bucket1, _k4, _v4, 9, ""},  // before delete version
		{_bucket1, _k4, _k4, 10, ""}, // after delete version
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
		// bucket2
		{_bucket2, _k1, nil, 0, _errNotExist},
		{_bucket2, _k2, nil, 0, _errNotExist}, // before first write version
		{_bucket2, _k2, _v2, 1, ""},
		{_bucket2, _k2, _v2, 2, ""},
		{_bucket2, _k2, _v1, 3, ""},
		{_bucket2, _k2, _v1, 5, ""},
		{_bucket2, _k2, _v4, 6, ""},
		{_bucket2, _k2, _v4, 7, ""}, // after last write version
		{_bucket2, _k3, nil, 0, _errNotExist},
		{_bucket2, _k4, nil, 1, _errNotExist},
		{_bucket2, _k5, nil, 0, _errNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid.Error()},
		// non-versioned namespace
		{_ns, _k1, _v1, 0, ""},
		{_ns, _k2, _v2, 0, ""},
		{_ns, _k3, nil, 0, _errNotExist},
		{_ns, _k4, nil, 0, _errNotExist},
		{_ns, _k5, nil, 0, _errNotExist},
		{_ns, _k10, nil, 0, _errNotExist},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
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
		{_bucket1, _k2, _k2, 10, ""},
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, _k4, 10, ""},
		{_bucket1, _k5, nil, 0, _errNotExist},
		{_bucket1, _k10, nil, 0, ErrInvalid.Error()},
		// bucket2
		{_bucket2, _k1, nil, 0, _errNotExist},
		{_bucket2, _k2, nil, 6, ""},
		{_bucket2, _k3, nil, 0, _errNotExist},
		{_bucket2, _k4, nil, 0, _errNotExist},
		{_bucket2, _k5, nil, 0, _errNotExist},
		{_bucket2, _k10, nil, 0, ErrInvalid.Error()},
		// non-versioned namespace
		{_ns, _k1, nil, 0, _errNonVersioned},
		{_ns, _k2, nil, 0, _errNonVersioned},
		{_ns, _k3, nil, 0, _errNonVersioned},
		{_ns, _k4, nil, 0, _errNonVersioned},
		{_ns, _k5, nil, 0, _errNonVersioned},
		{_ns, _k10, nil, 0, _errNonVersioned},
	} {
		value, err := db.Version(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.height, value)
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
	db := NewKVStoreWithVersion(cfg)
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

	r.NoError(db.SetVersion(1).WriteBatch(b))
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
		{_bucket1, _k1, nil, 0, _errNotExist},
		{_bucket1, _k2, nil, 0, _errNotExist},
		{_bucket1, _k3, nil, 0, _errNotExist},
		{_bucket1, _k4, nil, 0, _errNotExist},
		{_bucket1, _k1, _v1, 1, ""},
		{_bucket1, _k2, _v2, 1, ""},
		{_bucket1, _k1, _v1, 3, ""},
		{_bucket1, _k2, _v2, 3, ""},
	} {
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}

	// batch with wrong key length would fail
	b.Put(_bucket1, _v1, _k1, "test")
	r.Equal(ErrInvalid, errors.Cause(db.SetVersion(3).WriteBatch(b)))
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
		// non-versioned namespace
		{_ns, _k1, _v1, 1, ""},
		{_ns, _k2, _v2, 1, ""},
		{_ns, _v3, _k3, 1, ""},
		{_ns, _v4, _k4, 1, ""},
	} {
		b.Put(e.ns, e.k, e.v, "test")
	}
	b.Delete(_bucket1, _k3, "test")
	b.Delete(_bucket2, _v3, "test")
	b.Delete(_ns, _v3, "test")

	r.NoError(db.SetVersion(5).WriteBatch(b))
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
		value, err := db.SetVersion(e.height).Get(e.ns, e.k)
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
			value, err := db.SetVersion(h).Get(e.ns, e.k)
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
			value, err := db.SetVersion(h).Get(e.ns, e.k)
			if len(e.err) == 0 {
				r.NoError(err)
			} else {
				r.ErrorContains(err, e.err)
			}
			r.Equal(e.v, value)
		}
	}
	// non-versioned namespace
	for _, e := range []versionTest{
		{_ns, _k1, _v1, 1, ""},
		{_ns, _k2, _v2, 1, ""},
		{_ns, _v3, nil, 1, _errNotExist},
		{_ns, _v4, _k4, 1, ""},
	} {
		value, err := db.Get(e.ns, e.k)
		if len(e.err) == 0 {
			r.NoError(err)
		} else {
			r.ErrorContains(err, e.err)
		}
		r.Equal(e.v, value)
	}
}
