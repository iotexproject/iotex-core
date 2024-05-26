// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"encoding/hex"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestPb(t *testing.T) {
	r := require.New(t)

	vn := &versionedNamespace{
		keyLen: 5}
	data := vn.serialize()
	r.Equal("1005", hex.EncodeToString(data))
	vn1, err := deserializeVersionedNamespace(data)
	r.NoError(err)
	r.Equal(vn, vn1)

	km := &keyMeta{
		lastWrite:     []byte{1, 3, 5, 7, 9, 2, 4, 6, 8, 10},
		firstVersion:  3,
		lastVersion:   20,
		deleteVersion: []uint64{2, 13, 201},
	}
	data = km.serialize()
	r.Equal("0a0a0103050709020406080a100318142204020dc901", hex.EncodeToString(data))
	km1, err := deserializeKeyMeta(data)
	r.NoError(err)
	r.Equal(km, km1)
}

type kmTest struct {
	first, last, delete, version uint64
	hitOrExit                    bool
	err                          error
}

func TestKmUpdate(t *testing.T) {
	r := require.New(t)
	t.Run("check read", func(t *testing.T) {
		var km keyMeta
		for _, e := range []kmTest{
			// write at version 1
			{1, 1, 0, 0, false, ErrNotExist},
			{1, 1, 0, 1, true, nil},
			{1, 1, 0, 2, true, nil},
			// write at version 1, delete at version 1
			{1, 1, 1, 0, false, ErrNotExist},
			{1, 1, 1, 1, false, nil},
			{1, 1, 1, 2, false, nil},
			// write at version 1, delete at version 3
			{1, 1, 3, 0, false, ErrNotExist},
			{1, 1, 3, 1, true, nil},
			{1, 1, 3, 2, true, nil},
			{1, 1, 3, 3, false, ErrDeleted},
			{1, 1, 3, 4, false, ErrDeleted},
			// write at version 3
			{1, 3, 0, 0, false, ErrNotExist},
			{1, 3, 0, 1, false, nil},
			{1, 3, 0, 2, false, nil},
			{1, 3, 0, 3, true, nil},
			{1, 3, 0, 4, true, nil},
			// write at version 3, delete at version 3
			{1, 3, 3, 0, false, ErrNotExist},
			{1, 3, 3, 1, false, nil},
			{1, 3, 3, 2, false, nil},
			{1, 3, 3, 3, false, nil},
			{1, 3, 3, 4, false, nil},
			// write at version 3, delete at version 5
			{1, 3, 5, 0, false, ErrNotExist},
			{1, 3, 5, 1, false, nil},
			{1, 3, 5, 2, false, nil},
			{1, 3, 5, 3, true, nil},
			{1, 3, 5, 4, true, nil},
			{1, 3, 5, 5, false, ErrDeleted},
			{1, 3, 5, 6, false, ErrDeleted},
		} {
			km.firstVersion = e.first
			km.lastVersion = e.last
			km.deleteVersion = km.deleteVersion[:0]
			if e.delete != 0 {
				km.deleteVersion = append(km.deleteVersion, e.delete)
			}
			r.Equal(e.delete, km.lastDelete())
			hitLast, err := km.checkRead(e.version)
			r.Equal(e.hitOrExit, hitLast)
			r.Equal(e.err, errors.Cause(err))
		}
	})
	t.Run("update write", func(t *testing.T) {
		var km keyMeta
		for _, e := range []kmTest{
			// write at version 1
			{1, 1, 0, 0, true, nil},
			{1, 1, 0, 1, false, nil},
			{1, 1, 0, 2, false, nil},
			// write at version 1, delete at version 1
			{1, 1, 1, 0, true, nil},
			{1, 1, 1, 1, false, nil},
			{1, 1, 1, 2, false, nil},
			// write at version 1, delete at version 3
			{1, 1, 3, 0, true, nil},
			{1, 1, 3, 1, true, nil},
			{1, 1, 3, 2, true, nil},
			{1, 1, 3, 3, false, nil},
			{1, 1, 3, 4, false, nil},
			// write at version 3
			{1, 3, 0, 0, true, nil},
			{1, 3, 0, 1, true, nil},
			{1, 3, 0, 2, true, nil},
			{1, 3, 0, 3, false, nil},
			{1, 3, 0, 4, false, nil},
			// write at version 3, delete at version 3
			{1, 3, 3, 0, true, nil},
			{1, 3, 3, 1, true, nil},
			{1, 3, 3, 2, true, nil},
			{1, 3, 3, 3, false, nil},
			{1, 3, 3, 4, false, nil},
			// write at version 3, delete at version 5
			{1, 3, 5, 0, true, nil},
			{1, 3, 5, 1, true, nil},
			{1, 3, 5, 2, true, nil},
			{1, 3, 5, 3, true, nil},
			{1, 3, 5, 4, true, nil},
			{1, 3, 5, 5, false, nil},
			{1, 3, 5, 6, false, nil},
		} {
			km.lastWrite = nil
			km.firstVersion = e.first
			km.lastVersion = e.last
			km.deleteVersion = km.deleteVersion[:0]
			if e.delete != 0 {
				km.deleteVersion = append(km.deleteVersion, e.delete)
			}
			km0, exit := km.updateWrite(e.version, _v1)
			r.Equal(e.hitOrExit, exit)
			if !exit {
				r.Equal(_v1, km0.lastWrite)
				r.Equal(e.version, km0.lastVersion)
			}
		}
	})
	t.Run("multiple write and delete", func(t *testing.T) {
		km := keyMeta{
			firstVersion: 1,
		}
		km.updateWrite(1, nil)
		km.updateWrite(3, nil)
		r.NoError(km.updateDelete(7))
		for _, e := range []kmTest{
			{0, 0, 0, 0, false, ErrNotExist},
			{0, 0, 0, 1, false, nil},
			{0, 0, 0, 2, false, nil},
			{0, 0, 0, 3, true, nil},
			{0, 0, 0, 6, true, nil},
			{0, 0, 0, 7, false, ErrDeleted},
			{0, 0, 0, 8, false, ErrDeleted},
		} {
			hit, err := km.checkRead(e.version)
			r.Equal(e.hitOrExit, hit)
			r.Equal(e.err, err)
		}
		km.updateWrite(10, nil)
		r.NoError(km.updateDelete(15))
		r.NoError(km.updateDelete(18))
		r.Equal(3, len(km.deleteVersion))
		for _, e := range []kmTest{
			{0, 0, 0, 0, false, ErrNotExist},
			{0, 0, 0, 9, false, nil}, // need to get actual last version to determine if deleted
			{0, 0, 0, 10, true, nil},
			{0, 0, 0, 14, true, nil},
			{0, 0, 0, 15, false, ErrDeleted},
			{0, 0, 0, 17, false, ErrDeleted},
			{0, 0, 0, 18, false, ErrDeleted},
			{0, 0, 0, 21, false, ErrDeleted},
		} {
			hit, err := km.checkRead(e.version)
			r.Equal(e.hitOrExit, hit)
			r.Equal(e.err, err)
		}
		km.updateWrite(21, nil)
		km.updateWrite(25, nil)
		r.EqualValues(7, km.deleteVersion[0])
		r.EqualValues(15, km.deleteVersion[1])
		r.EqualValues(18, km.deleteVersion[2])
		for _, e := range []kmTest{
			{0, 0, 0, 0, false, ErrNotExist},
			{0, 0, 0, 24, false, nil}, // need to get actual last version to determine if deleted
			{0, 0, 0, 25, true, nil},
			{0, 0, 0, 25000, true, nil},
		} {
			hit, err := km.checkRead(e.version)
			r.Equal(e.hitOrExit, hit)
			r.Equal(e.err, err)
		}
	})
}
