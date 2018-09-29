// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rds

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"database/sql"
	"errors"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	cfg = &config.Default.RDS
)

func TestRDSStorePutGet(t *testing.T) {
	testRDSStorePutGet := func(rdsStore RDSStore, t *testing.T) {
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
		userAddress := "bbb"
		transferHash := hash.ZeroHash32B

		// insert
		stmt, err := db.Prepare("INSERT transfer_history SET node_address=?,user_address=?,transfer_hash=?")
		require.Nil(err)

		res, err := stmt.Exec(nodeAddress, userAddress, transferHash[:])
		require.Nil(err)

		affect, err := res.RowsAffected()
		require.Nil(err)
		require.Equal(int64(1), affect)

		// get
		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=?")
		require.Nil(err)

		rows, err := stmt.Query(nodeAddress)
		require.Nil(err)

		parsedRows, err := rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		require.Equal(nodeAddress, string(parsedRows[0][0]))
		require.Equal(userAddress, string(parsedRows[0][1]))
		require.Equal(string(transferHash[:]), string(parsedRows[0][2]))

		// delete
		stmt, err = db.Prepare("DELETE FROM transfer_history WHERE node_address=? AND user_address=? AND transfer_hash=?")
		require.Nil(err)

		res, err = stmt.Exec(nodeAddress, userAddress, transferHash[:])
		require.Nil(err)

		affect, err = res.RowsAffected()
		require.Nil(err)
		require.Equal(int64(1), affect)

		// get
		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=?")
		require.Nil(err)

		rows, err = stmt.Query(nodeAddress)
		require.Nil(err)

		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(0, len(parsedRows))
	}

	path := "/tmp/test-rds-store-" + strconv.Itoa(rand.Int())
	t.Run("RDS Store", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testRDSStorePutGet(NewAwsRDS(cfg), t)
	})
}

func TestRDSStoreTransaction(t *testing.T) {
	testRDSStoreTransaction := func(rdsStore RDSStore, t *testing.T) {
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
		transferHash := hash.ZeroHash32B

		// get
		stmt, err := db.Prepare("SELECT * FROM transfer_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err := stmt.Query(nodeAddress, userAddress1)
		require.Nil(err)
		parsedRows, err := rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress2)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		/*require.Equal(node_address, string(parsedRows[0][0]))
		require.Equal(user_adress1, string(parsedRows[0][1]))
		require.Equal(string(transfer_hash[:]), string(parsedRows[0][2]))*/

		// insert transaction with fail
		err = rdsStore.Transact(func(tx *sql.Tx) error {
			insertQuery := "INSERT transfer_history SET node_address=?,user_address=?,transfer_hash=?"
			if _, err := tx.Exec(insertQuery, nodeAddress, userAddress1, transferHash[:]); err != nil {
				return err
			}
			return errors.New("raise error on purpose")
			if _, err := tx.Exec(insertQuery, nodeAddress, userAddress2, transferHash[:]); err != nil {
				return err
			}
			return nil
		})
		require.NotNil(err)

		// get
		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress1)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress2)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		// insert
		err = rdsStore.Transact(func(tx *sql.Tx) error {
			insertQuery := "INSERT transfer_history SET node_address=?,user_address=?,transfer_hash=?"
			if _, err := tx.Exec(insertQuery, nodeAddress, userAddress1, transferHash[:]); err != nil {
				return err
			}
			if _, err := tx.Exec(insertQuery, nodeAddress, userAddress2, transferHash[:]); err != nil {
				return err
			}
			return nil
		})
		require.Nil(err)

		// get
		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress1)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		require.Equal(nodeAddress, string(parsedRows[0][0]))
		require.Equal(userAddress1, string(parsedRows[0][1]))
		require.Equal(string(transferHash[:]), string(parsedRows[0][2]))

		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress2)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		require.Equal(nodeAddress, string(parsedRows[0][0]))
		require.Equal(userAddress2, string(parsedRows[0][1]))
		require.Equal(string(transferHash[:]), string(parsedRows[0][2]))

		// delete
		err = rdsStore.Transact(func(tx *sql.Tx) error {
			deleteQuery := "DELETE FROM transfer_history WHERE node_address=? AND user_address=? AND transfer_hash=?"
			if _, err := tx.Exec(deleteQuery, nodeAddress, userAddress1, transferHash[:]); err != nil {
				return err
			}
			if _, err := tx.Exec(deleteQuery, nodeAddress, userAddress2, transferHash[:]); err != nil {
				return err
			}
			return nil
		})
		require.Nil(err)

		// get
		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress1)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(0, len(parsedRows))

		stmt, err = db.Prepare("SELECT * FROM transfer_history WHERE node_address=? AND user_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddress, userAddress2)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(0, len(parsedRows))
	}

	path := "/tmp/test-rds-store-batch-rollback" + strconv.Itoa(rand.Int())
	t.Run("RDS Store", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testRDSStoreTransaction(NewAwsRDS(cfg), t)
	})
}
