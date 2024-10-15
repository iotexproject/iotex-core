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
	for _, path := range [][2]string{
		{"../data/archive.20k", "5e80b7e7199326e19a2d25beb39aa83bc3c3d68c288e7fa08dc29e6936a0cc96"},
		{"../data/archive.40k", "b39c39f23b68d26d82856ab6f270114bb134d91a06b40028d923185373eeb52f"},
		{"../data/archive.80k", "6e5929b2267a0c7584c332c203ad745d51524a51ec320731e99d82a480aa6428"},
	} {
		cfg.DbPath = path[0]
		db := NewBoltDBVersioned(cfg)
		ctx := context.Background()
		r.NoError(db.Start(ctx))
		defer func() {
			db.Stop(ctx)
		}()
		println("======= check file:", path[0])
		h, err := db.Hash()
		r.NoError(err)
		r.Equal(path[1], h)
	}
}

func TestListBucket(t *testing.T) {
	r := require.New(t)

	cfg := DefaultConfig
	cfg.DbPath = "../data/archive.db"
	db := NewBoltDBVersioned(cfg)
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()
	println("======= check file:", cfg.DbPath)
	r.NoError(db.listBucket("CandsMap"))
}

func TestCountHeightKey(t *testing.T) {
	r := require.New(t)

	cfg := DefaultConfig
	cfg.DbPath = "../data/archive.20to80k"
	db := NewBoltDBVersioned(cfg)
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()
	println("======= check file:", cfg.DbPath)
	n, err := db.countHeightKey("Account", 20000)
	r.NoError(err)
	r.Equal(20326, n)
}

func TestCopyBucket(t *testing.T) {
	r := require.New(t)

	cfg := DefaultConfig
	cfg.DbPath = "../data/archive.db"
	db := NewBoltDBVersioned(cfg)
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()
	cfg.DbPath = "../data/archive.db2"
	db2 := NewBoltDBVersioned(cfg)
	r.NoError(db2.Start(ctx))
	defer func() {
		db2.Stop(ctx)
	}()
	buckets, err := db2.Buckets()
	r.NoError(err)
	for _, v := range buckets {
		if v == "Account" || v == "Contract" {
			r.NoError(db2.PurgeVersion(v, 20000))
		} else {
			r.NoError(db.PurgeVersion(v, 20000))
		}
		r.NoError(db.CopyBucket(v, db2))
	}
}
