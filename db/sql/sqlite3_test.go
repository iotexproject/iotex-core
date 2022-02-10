// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"testing"

	"github.com/iotexproject/iotex-core/testutil"

	"github.com/stretchr/testify/require"
)

const (
	path = "explorer.db"
)

func TestSQLite3StorePutGet(t *testing.T) {
	testRDSStorePutGet := TestStorePutGet
	testPath, err := testutil.PathOfTempFile(path)
	defer testutil.CleanupPathV2(testPath)
	require.NoError(t, err)
	cfg := CQLITE3{
		SQLite3File: testPath,
	}
	t.Run("SQLite3 Store", func(t *testing.T) {
		testRDSStorePutGet(NewSQLite3(cfg), t)
	})
}

func TestSQLite3StoreTransaction(t *testing.T) {
	testSQLite3StoreTransaction := TestStoreTransaction
	testPath, err := testutil.PathOfTempFile(path)
	defer testutil.CleanupPathV2(testPath)
	require.NoError(t, err)
	cfg := CQLITE3{
		SQLite3File: testPath,
	}
	t.Run("SQLite3 Store", func(t *testing.T) {
		testSQLite3StoreTransaction(NewSQLite3(cfg), t)
	})
}
