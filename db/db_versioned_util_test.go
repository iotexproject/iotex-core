// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"encoding/hex"
	"testing"

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
}

func TestDBHash(t *testing.T) {
	r := require.New(t)

	cfg := DefaultConfig
	cfg.DbPath = "../data/archive.20k"
	db := NewBoltDBVersioned(cfg)
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	h, err := db.Hash()
	r.NoError(err)
	r.Equal("5e80b7e7199326e19a2d25beb39aa83bc3c3d68c288e7fa08dc29e6936a0cc96", h)
}

func TestCopyBucket(t *testing.T) {
	r := require.New(t)

	cfg := DefaultConfig
	cfg.DbPath = "../data/archive.20k"
	db := NewBoltDBVersioned(cfg)
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()
	r.NoError(db.CopyBucket("Contract"))
}
