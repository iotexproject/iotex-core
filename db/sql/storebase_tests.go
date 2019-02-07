// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sql

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/hash"
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

	err := sqlStore.Start(ctx)
	require.Nil(err)
	defer func() {
		err = sqlStore.Stop(ctx)
		require.Nil(err)
	}()

	dbinstance := sqlStore.GetDB()

	nodeAddress := "aaa"
	userAddress := "bbb"
	actionHash := hash.ZeroHash256

	// create table
	_, err = dbinstance.Exec("CREATE TABLE IF NOT EXISTS action_history ([node_address] TEXT NOT NULL, [user_address] " +
		"TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)")
	require.Nil(err)

	// insert
	stmt, err := dbinstance.Prepare("INSERT INTO action_history (node_address,user_address,action_hash) VALUES (?, ?, ?)")
	require.Nil(err)

	res, err := stmt.Exec(nodeAddress, userAddress, actionHash[:])
	require.Nil(err)

	affect, err := res.RowsAffected()
	require.Nil(err)
	require.Equal(int64(1), affect)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=?")
	require.Nil(err)

	rows, err := stmt.Query(nodeAddress)
	require.Nil(err)

	var actionHistory ActionHistory
	parsedRows, err := ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
	require.Equal(1, len(parsedRows))
	require.Equal(nodeAddress, parsedRows[0].(*ActionHistory).NodeAddress)
	require.Equal(userAddress, parsedRows[0].(*ActionHistory).UserAddress)
	require.Equal(string(actionHash[:]), parsedRows[0].(*ActionHistory).ActionHash)

	// delete
	stmt, err = dbinstance.Prepare("DELETE FROM action_history WHERE node_address=? AND user_address=? AND action_hash=?")
	require.Nil(err)

	res, err = stmt.Exec(nodeAddress, userAddress, actionHash[:])
	require.Nil(err)

	affect, err = res.RowsAffected()
	require.Nil(err)
	require.Equal(int64(1), affect)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=?")
	require.Nil(err)

	rows, err = stmt.Query(nodeAddress)
	require.Nil(err)

	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
	require.Equal(0, len(parsedRows))
}

// TestStoreTransaction define the common test cases for transaction
func TestStoreTransaction(sqlStore Store, t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	err := sqlStore.Start(ctx)
	require.Nil(err)
	defer func() {
		err = sqlStore.Stop(ctx)
		require.Nil(err)
	}()

	dbinstance := sqlStore.GetDB()

	nodeAddress := "aaa"
	userAddress1 := "bbb1"
	userAddress2 := "bbb2"
	actionHash := hash.ZeroHash256

	// create table
	_, err = dbinstance.Exec("CREATE TABLE IF NOT EXISTS action_history ([node_address] TEXT NOT NULL, [user_address] " +
		"TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)")
	require.Nil(err)

	// get
	stmt, err := dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.Nil(err)
	rows, err := stmt.Query(nodeAddress, userAddress1)
	require.Nil(err)
	var actionHistory ActionHistory
	parsedRows, err := ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
	require.Equal(0, len(parsedRows))

	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.Nil(err)
	rows, err = stmt.Query(nodeAddress, userAddress2)
	require.Nil(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
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
	require.NotNil(err)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.Nil(err)
	rows, err = stmt.Query(nodeAddress, userAddress1)
	require.Nil(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
	require.Equal(0, len(parsedRows))

	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.Nil(err)
	rows, err = stmt.Query(nodeAddress, userAddress2)
	require.Nil(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
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
	require.Nil(err)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.Nil(err)
	rows, err = stmt.Query(nodeAddress, userAddress1)
	require.Nil(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
	require.Equal(1, len(parsedRows))
	require.Equal(nodeAddress, parsedRows[0].(*ActionHistory).NodeAddress)
	require.Equal(userAddress1, parsedRows[0].(*ActionHistory).UserAddress)
	require.Equal(string(actionHash[:]), parsedRows[0].(*ActionHistory).ActionHash)

	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.Nil(err)
	rows, err = stmt.Query(nodeAddress, userAddress2)
	require.Nil(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
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
	require.Nil(err)

	// get
	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.Nil(err)
	rows, err = stmt.Query(nodeAddress, userAddress1)
	require.Nil(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
	require.Equal(0, len(parsedRows))

	stmt, err = dbinstance.Prepare("SELECT * FROM action_history WHERE node_address=? AND user_address=?")
	require.Nil(err)
	rows, err = stmt.Query(nodeAddress, userAddress2)
	require.Nil(err)
	parsedRows, err = ParseSQLRows(rows, &actionHistory)
	require.Nil(err)
	require.Equal(0, len(parsedRows))
}
