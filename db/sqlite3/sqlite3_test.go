// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sqlite3

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"database/sql"
	"errors"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/stretchr/testify/require"
)

var (
	cfg = &config.SQLITE3{
		SQLite3File: "explorer.db",
	}
)

type ActionHistory struct {
	NodeAddress string
	UserAddress string
	ActionHash  string
}

func TestSQLite3StorePutGet(t *testing.T) {
	testRDSStorePutGet := func(sqlite3Store Store, t *testing.T) {
		//t.Skip("Skipping when sqlite3 credentail not provided.")

		require := require.New(t)
		ctx := context.Background()

		err := sqlite3Store.Start(ctx)
		require.Nil(err)
		defer func() {
			err = sqlite3Store.Stop(ctx)
			require.Nil(err)
		}()

		db := sqlite3Store.GetDB()

		nodeAddress := "aaa"
		userAddress := "bbb"
		actionHash := hash.ZeroHash32B

		// create table
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS action_history ([node_address] TEXT NOT NULL, [user_address] TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)")
		require.Nil(err)

		// insert
		stmt, err := db.Prepare("INSERT INTO action_history (node_address,user_address,action_hash) VALUES (?, ?, ?)")
		require.Nil(err)

		res, err := stmt.Exec(nodeAddress, userAddress, actionHash[:])
		require.Nil(err)

		affect, err := res.RowsAffected()
		require.Nil(err)
		require.Equal(int64(1), affect)

		// get
		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=?")
		require.Nil(err)

		rows, err := stmt.Query(nodeAddress)
		require.Nil(err)

		var actionHistory ActionHistory
		parsedRows, err := ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		require.Equal(nodeAddress, parsedRows[0].(*ActionHistory).NodeAddress)
		require.Equal(userAddress, parsedRows[0].(*ActionHistory).UserAddress)
		require.Equal(string(actionHash[:]), parsedRows[0].(*ActionHistory).ActionHash)

		// delete
		stmt, err = db.Prepare("DELETE FROM action_history WHERE node_address=? AND user_address=? AND action_hash=?")
		require.Nil(err)

		res, err = stmt.Exec(nodeAddress, userAddress, actionHash[:])
		require.Nil(err)

		affect, err = res.RowsAffected()
		require.Nil(err)
		require.Equal(int64(1), affect)

		// get
		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=?")
		require.Nil(err)

		rows, err = stmt.Query(nodeAddress)
		require.Nil(err)

		parsedRows, err = ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(0, len(parsedRows))
	}

	path := "/tmp/test-sqlite3-store-" + strconv.Itoa(rand.Int())
	t.Run("SQLite3 Store", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testRDSStorePutGet(NewSQLite3(cfg), t)
	})
}

func TestSQLite3StoreTransaction(t *testing.T) {
	testSQLite3StoreTransaction := func(rdsStore Store, t *testing.T) {
		//t.Skip("Skipping when SQLite3 credentail not provided.")

		require := require.New(t)
		ctx := context.Background()

		err := rdsStore.Start(ctx)
		require.Nil(err)
		defer func() {
			err = rdsStore.Stop(ctx)
			require.Nil(err)
		}()

		db := rdsStore.GetDB()

		nodeAddress := "aaa"
		userAddress1 := "bbb1"
		userAddress2 := "bbb2"
		actionHash := hash.ZeroHash32B

		// create table
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS action_history ([node_address] TEXT NOT NULL, [user_address] TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)")
		require.Nil(err)

		// get
		stmt, err := db.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err := stmt.Query(nodeAddress, userAddress1)
		require.Nil(err)
		var actionHistory ActionHistory
		parsedRows, err := ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress2)
		require.Nil(err)
		parsedRows, err = ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		// insert transaction with fail
		err = rdsStore.Transact(func(tx *sql.Tx) error {
			insertQuery := "INSERT INTO action_history (node_address,user_address,action_hash) VALUES (?, ?, ?)"
			if _, err := tx.Exec(insertQuery, nodeAddress, userAddress1, actionHash[:]); err != nil {
				return err
			}
			if _, err := tx.Exec(insertQuery, nodeAddress, userAddress1, actionHash[:]); err != nil {
				return errors.New("create an error")
			}
			return errors.New("create an error")
		})
		require.NotNil(err)

		// get
		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress1)
		require.Nil(err)
		parsedRows, err = ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress2)
		require.Nil(err)
		parsedRows, err = ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		// insert
		err = rdsStore.Transact(func(tx *sql.Tx) error {
			insertQuery := "INSERT INTO action_history (node_address,user_address,action_hash) VALUES (?, ?, ?)"
			if _, err := tx.Exec(insertQuery, nodeAddress, userAddress1, actionHash[:]); err != nil {
				return err
			}
			if _, err := tx.Exec(insertQuery, nodeAddress, userAddress2, actionHash[:]); err != nil {
				return err
			}
			return nil
		})
		require.Nil(err)

		// get
		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress1)
		require.Nil(err)
		parsedRows, err = ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		require.Equal(nodeAddress, parsedRows[0].(*ActionHistory).NodeAddress)
		require.Equal(userAddress1, parsedRows[0].(*ActionHistory).UserAddress)
		require.Equal(string(actionHash[:]), parsedRows[0].(*ActionHistory).ActionHash)

		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress2)
		require.Nil(err)
		parsedRows, err = ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		require.Equal(nodeAddress, parsedRows[0].(*ActionHistory).NodeAddress)
		require.Equal(userAddress2, parsedRows[0].(*ActionHistory).UserAddress)
		require.Equal(string(actionHash[:]), parsedRows[0].(*ActionHistory).ActionHash)

		// delete
		err = rdsStore.Transact(func(tx *sql.Tx) error {
			deleteQuery := "DELETE FROM action_history WHERE node_address=? AND user_address=? AND action_hash=?"
			if _, err := tx.Exec(deleteQuery, nodeAddress, userAddress1, actionHash[:]); err != nil {
				return err
			}
			if _, err := tx.Exec(deleteQuery, nodeAddress, userAddress2, actionHash[:]); err != nil {
				return err
			}
			return nil
		})
		require.Nil(err)

		// get
		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress1)
		require.Nil(err)
		parsedRows, err = ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		stmt, err = db.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress2)
		require.Nil(err)
		parsedRows, err = ParseRows(rows, &actionHistory)
		require.Nil(err)
		require.Equal(0, len(parsedRows))
	}

	path := "/tmp/test-sqlite3-store-batch-rollback" + strconv.Itoa(rand.Int())
	t.Run("SQLite3 Store", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testSQLite3StoreTransaction(NewSQLite3(cfg), t)
	})
}
