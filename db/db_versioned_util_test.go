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
		deleteVersion: 201,
	}
	data = km.serialize()
	r.Equal("0a0a0103050709020406080a1003181420c901", hex.EncodeToString(data))
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
	t.Run("update read", func(t *testing.T) {
		var km keyMeta
		for _, e := range []kmTest{
			// write at version 1
			{1, 1, 0, 0, false, ErrNotExist},
			{1, 1, 0, 1, true, nil},
			{1, 1, 0, 2, true, nil},
			// write at version 1, delete at version 1
			{1, 1, 1, 0, false, ErrNotExist},
			{1, 1, 1, 1, false, ErrDeleted},
			{1, 1, 1, 2, false, ErrDeleted},
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
			{1, 3, 3, 3, false, ErrDeleted},
			{1, 3, 3, 4, false, ErrDeleted},
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
			km.deleteVersion = e.delete
			hitLast, err := km.updateRead(e.version)
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
			km.deleteVersion = e.delete
			km0, exit := km.updateWrite(e.version, _v1)
			r.Equal(e.hitOrExit, exit)
			if !exit {
				r.Equal(_v1, km0.lastWrite)
				r.Equal(e.version, km0.lastVersion)
				r.Zero(km0.deleteVersion)
			}
		}
	})
}
