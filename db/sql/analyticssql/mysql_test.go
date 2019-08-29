// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"database/sql"
	"fmt"
	"testing"
)

const (
	connectStr = "ba8df54bd3754e:9cd1f263@tcp(us-cdbr-iron-east-02.cleardb.net:3306)/"
	dbName     = "heroku_7fed0b046078f80"
)

func TestMySQLStorePutGet(t *testing.T) {
	CleanupDatabase(t, connectStr, dbName)
	testRDSStorePutGet := TestStorePutGet
	t.Run("MySQL Store", func(t *testing.T) {
		testRDSStorePutGet(NewMySQL(connectStr, dbName), t)
	})
	CleanupDatabase(t, connectStr, dbName)
}

func TestMySQLStoreTransaction(t *testing.T) {
	CleanupDatabase(t, connectStr, dbName)
	testSQLite3StoreTransaction := TestStoreTransaction
	t.Run("MySQL Store", func(t *testing.T) {
		testSQLite3StoreTransaction(NewMySQL(connectStr, dbName), t)
	})
	CleanupDatabase(t, connectStr, dbName)
}

// CleanupDatabase detects the existence of a MySQL database and drops it if found
func CleanupDatabase(t *testing.T, connectStr string, dbName string) {
	db, err := sql.Open("mysql", connectStr)
	if err != nil {
		t.Error("Failed to open the database")
	}
	if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
		fmt.Println(err)
		t.Error("Failed to drop the database")
	}
	if err := db.Close(); err != nil {
		t.Error("Failed to close the database")
	}
}
