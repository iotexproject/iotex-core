// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckNS(t *testing.T) {
	r := require.New(t)
	cfg := DefaultConfig
	cfg.DbPath = "../data/trie.135"
	db := NewKVStoreWithVersion(cfg, VersionedNamespaceOption("Account", "Contract"))
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	total, _, err := db.db.CheckAccount("Account")
	r.NoError(err)
	println("total address =", total)
	total, _, err = db.db.CheckContract("Contract")
	r.NoError(err)
	println("total contract =", total)
}

func TestVerNS(t *testing.T) {
	r := require.New(t)
	cfg := DefaultConfig
	cfg.DbPath = "../data/trie.135"
	db := NewKVStoreWithVersion(cfg, VersionedNamespaceOption("Account", "Contract"))
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	total, count, h, err := db.db.AllKeys("Account")
	r.NoError(err)
	println("total key =", total)
	println("total address =", count)
	println("hash =", h)
	total, count, h, err = db.db.AllKeys("Contract")
	r.NoError(err)
	println("total key =", total)
	println("total contract =", count)
	println("hash =", h)
}

func TestConvert(t *testing.T) {
	r := require.New(t)
	cfg := DefaultConfig
	cfg.DbPath = "../data/trie.135"
	db := NewKVStoreWithVersion(cfg, VersionedNamespaceOption("Account", "Contract"))
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	r.NoError(db.db.TransformToVersioned(8000000, "Account"))
	r.NoError(db.db.TransformToVersioned(8000000, "Contract"))
}

func TestContract(t *testing.T) {
	r := require.New(t)
	cfg := DefaultConfig
	cfg.DbPath = "../data/archive.8m"
	db := NewKVStoreWithVersion(cfg, VersionedNamespaceOption("Account", "Contract"))
	ctx := context.Background()
	r.NoError(db.Start(ctx))
	defer func() {
		db.Stop(ctx)
	}()

	total, count, del, err := db.db.CheckContractVersioned("Contract")
	r.NoError(err)
	println("total key =", total)
	println("total address =", count)
	println("total delete =", del)
}
