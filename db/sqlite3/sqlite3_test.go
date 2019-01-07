package sqlite3

import (
	"testing"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestSQLite3StorePutGet(t *testing.T) {
	testRDSStorePutGet := db.TestStorePutGet

	path := "explorer.db"
	cfg := &config.SQLITE3{
		SQLite3File: path,
	}
	t.Run("SQLite3 Store", func(t *testing.T) {
		testRDSStorePutGet(NewSQLite3(cfg), t)
	})
}

func TestSQLite3StoreTransaction(t *testing.T) {
	testSQLite3StoreTransaction := db.TestStoreTransaction

	path := "explorer.db"
	cfg := &config.SQLITE3{
		SQLite3File: path,
	}
	t.Run("SQLite3 Store", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testSQLite3StoreTransaction(NewSQLite3(cfg), t)
	})
}
