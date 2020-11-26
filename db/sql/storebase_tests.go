// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
)

// ActionHistory define the schema for action history
type ActionHistory struct {
	NodeAddress string
	UserAddress string
	ActionHash  string
}

// TestStorePutGet define the common test cases for put and get
func TestStorePutGet(sqlStore Store, t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	require.NoError(sqlStore.Start(ctx))
	defer func() {
		require.NoError(sqlStore.Stop(ctx))
	}()

	dbinstance := sqlStore.GetDB()

	nodeAddress := "aaa"
	userAddress := "bbb"
	actionHash := hash.ZeroHash256

	// create table
	_, err := dbinstance.Exec("CREATE TABLE IF NOT EXISTS action_history ([node_address] TEXT NOT NULL, [user_address] " +
		"TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)")
	require.NoError(err)

	// insert
	stmt, err := dbinstance.Prepare("INSERT INTO action_history (node_address,user_address,action_hash) VALUES (?, ?, ?)")
	require.NoError(err)

	res, err := stmt.Exec(nodeAddress, userAddress, actionHash[:])
	require.NoError(err)

	affect, err := res.RowsAffected()
	require.NoError(err)
	require.Equal(int64(1), affect)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=?")
	require.NoError(err)

	rows, err := stmt.Query(nodeAddress)
	require.NoError(err)

	var actionHistory ActionHistory
	parsedRows, err := ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(1, len(parsedRows))
	require.Equal(nodeAddress, parsedRows[0].(*ActionHistory).NodeAddress)
	require.Equal(userAddress, parsedRows[0].(*ActionHistory).UserAddress)
	require.Equal(string(actionHash[:]), parsedRows[0].(*ActionHistory).ActionHash)

	// delete
	stmt, err = dbinstance.Prepare("DELETE FROM action_history WHERE node_address=? AND user_address=? AND action_hash=?")
	require.NoError(err)

	res, err = stmt.Exec(nodeAddress, userAddress, actionHash[:])
	require.NoError(err)

	affect, err = res.RowsAffected()
	require.NoError(err)
	require.Equal(int64(1), affect)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=?")
	require.NoError(err)

	rows, err = stmt.Query(nodeAddress)
	require.NoError(err)

	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(0, len(parsedRows))
}

// TestStoreTransaction define the common test cases for transaction
func TestStoreTransaction(sqlStore Store, t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	require.NoError(sqlStore.Start(ctx))
	defer func() {
		require.NoError(sqlStore.Stop(ctx))
	}()

	dbinstance := sqlStore.GetDB()

	nodeAddress := "aaa"
	userAddress1 := "bbb1"
	userAddress2 := "bbb2"
	actionHash := hash.ZeroHash256

	// create table
	_, err := dbinstance.Exec("CREATE TABLE IF NOT EXISTS action_history ([node_address] TEXT NOT NULL, [user_address] " +
		"TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)")
	require.NoError(err)

	// get
	stmt, err := dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.NoError(err)
	rows, err := stmt.Query(nodeAddress, userAddress1)
	require.NoError(err)
	var actionHistory ActionHistory
	parsedRows, err := ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(0, len(parsedRows))

	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.NoError(err)
	rows, err = stmt.Query(nodeAddress, userAddress2)
	require.NoError(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(0, len(parsedRows))

	// insert transaction with fail
	err = sqlStore.Transact(func(tx *sql.Tx) error {
		insertQuery := "INSERT INTO action_history (node_address,user_address,action_hash) VALUES (?, ?, ?)"
		if _, err := tx.Exec(insertQuery, nodeAddress, userAddress1, actionHash[:]); err != nil {
			return err
		}
		if _, err := tx.Exec(insertQuery, nodeAddress, userAddress1, actionHash[:]); err != nil {
			return errors.New("create an error")
		}
		return errors.New("create an error")
	})
	println(err)
	require.Error(err)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.NoError(err)
	rows, err = stmt.Query(nodeAddress, userAddress1)
	require.NoError(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(0, len(parsedRows))

	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.NoError(err)
	rows, err = stmt.Query(nodeAddress, userAddress2)
	require.NoError(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(0, len(parsedRows))

	// insert
	err = sqlStore.Transact(func(tx *sql.Tx) error {
		insertQuery := "INSERT INTO action_history (node_address,user_address,action_hash) VALUES (?, ?, ?)"
		if _, err := tx.Exec(insertQuery, nodeAddress, userAddress1, actionHash[:]); err != nil {
			return err
		}
		if _, err := tx.Exec(insertQuery, nodeAddress, userAddress2, actionHash[:]); err != nil {
			return err
		}
		return nil
	})
	require.NoError(err)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.NoError(err)
	rows, err = stmt.Query(nodeAddress, userAddress1)
	require.NoError(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(1, len(parsedRows))
	require.Equal(nodeAddress, parsedRows[0].(*ActionHistory).NodeAddress)
	require.Equal(userAddress1, parsedRows[0].(*ActionHistory).UserAddress)
	require.Equal(string(actionHash[:]), parsedRows[0].(*ActionHistory).ActionHash)

	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.NoError(err)
	rows, err = stmt.Query(nodeAddress, userAddress2)
	require.NoError(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(1, len(parsedRows))
	require.Equal(nodeAddress, parsedRows[0].(*ActionHistory).NodeAddress)
	require.Equal(userAddress2, parsedRows[0].(*ActionHistory).UserAddress)
	require.Equal(string(actionHash[:]), parsedRows[0].(*ActionHistory).ActionHash)

	// delete
	err = sqlStore.Transact(func(tx *sql.Tx) error {
		deleteQuery := "DELETE FROM action_history WHERE node_address=? AND user_address=? AND action_hash=?"
		if _, err := tx.Exec(deleteQuery, nodeAddress, userAddress1, actionHash[:]); err != nil {
			return err
		}
		if _, err := tx.Exec(deleteQuery, nodeAddress, userAddress2, actionHash[:]); err != nil {
			return err
		}
		return nil
	})
	require.NoError(err)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.NoError(err)
	rows, err = stmt.Query(nodeAddress, userAddress1)
	require.NoError(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(0, len(parsedRows))

	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.NoError(err)
	rows, err = stmt.Query(nodeAddress, userAddress2)
	require.NoError(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.NoError(err)
	require.Equal(0, len(parsedRows))
}
