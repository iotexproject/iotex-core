// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package tracker

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
)

var (
	// BalanceHistoryTableName is the table name of account history
	BalanceHistoryTableName string
)

// BalanceChange records balance change of accounts
type BalanceChange struct {
	Amount     string
	InAddr     string
	OutAddr    string
	ActionHash hash.Hash256
}

func init() {
	BalanceHistoryTableName = os.Getenv("BALANCE_HISTORY_TABLE_NAME")
	if BalanceHistoryTableName == "" {
		BalanceHistoryTableName = "balance_history"
	}
}

// Type returns the type of state change
func (b BalanceChange) Type() reflect.Type {
	return reflect.TypeOf(b)
}

func (b BalanceChange) init(db *sql.DB, tx *sql.Tx) error {
	if _, err := db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"(epoch_number DECIMAL(65, 0) NOT NULL, block_height DECIMAL(65, 0) NOT NULL, action_hash VARCHAR(64) NOT NULL, "+
		"action_type TEXT NOT NULL, `from` VARCHAR(41) NOT NULL, `to` VARCHAR(41) NOT NULL, amount DECIMAL(65, 0) NOT NULL)", BalanceHistoryTableName)); err != nil {
		return err
	}

	// Check existence
	exist, err := rowExists(db, fmt.Sprintf("SELECT * FROM %s WHERE action_hash = ?",
		BalanceHistoryTableName), hex.EncodeToString(specialActionHash[:]))
	if err != nil {
		return errors.Wrap(err, "failed to check if the row exists")
	}
	if exist {
		return nil
	}

	initBalance := initBalanceMap()
	for addr, amount := range initBalance {
		insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height, action_hash, action_type, `from`, `to`, amount) VALUES (?, ?, ?, ?, ?, ?, ?)",
			BalanceHistoryTableName)
		if _, err := tx.Exec(insertQuery, uint64(0), uint64(0), hex.EncodeToString(specialActionHash[:]), "genesis", "", addr, amount); err != nil {
			return errors.Wrapf(err, "failed to update balance history for address %s", addr)
		}
	}
	return nil
}

func (b BalanceChange) handle(tx *sql.Tx, blockHeight uint64) error {
	epochNumber := uint64(0)
	if blockHeight != 0 {
		epochNumber = (blockHeight-1)/genesis.Default.NumDelegates/genesis.Default.NumSubEpochs + 1
	}
	actionType := "execution"
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height, action_hash, action_type, `from`, `to`, amount) VALUES (?, ?, ?, ?, ?, ?, ?)",
		BalanceHistoryTableName)
	if _, err := tx.Exec(insertQuery, epochNumber, blockHeight, hex.EncodeToString(b.ActionHash[:]), actionType, b.OutAddr, b.InAddr, b.Amount); err != nil {
		return errors.Wrap(err, "failed to update balance history")
	}
	return nil
}

func initBalanceMap() map[string]string {
	return map[string]string{
		"io1uqhmnttmv0pg8prugxxn7d8ex9angrvfjfthxa": "9800000000000000000000000000",
		"io1v3gkc49d5vwtdfdka2ekjl3h468egun8e43r7z": "100000000000000000000000000",
		"io1vrl48nsdm8jaujccd9cx4ve23cskr0ys6urx92": "100000000000000000000000000",
		"io1llupp3n8q5x8usnr5w08j6hc6hn55x64l46rr7": "100000000000000000000000000",
		"io1ns7y0pxmklk8ceattty6n7makpw76u770u5avy": "100000000000000000000000000",
		"io1xuavja5dwde8pvy4yms06yyncad4yavghjhwra": "100000000000000000000000000",
		"io1cdqx6p5rquudxuewflfndpcl0l8t5aezen9slr": "100000000000000000000000000",
		"io1hh97f273nhxcq8ajzcpujtt7p9pqyndfmavn9r": "100000000000000000000000000",
		"io1yhvu38epz5vmkjaclp45a7t08r27slmcc0zjzh": "100000000000000000000000000",
		"io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng": "100000000000000000000000000",
		"io1skmqp33qme8knyw0fzgt9takwrc2nvz4sevk5c": "100000000000000000000000000",
		"io1fxzh50pa6qc6x5cprgmgw4qrp5vw97zk5pxt3q": "100000000000000000000000000",
		"io1jh0ekmccywfkmj7e8qsuzsupnlk3w5337hjjg2": "100000000000000000000000000",
		"io1juvx5g063eu4ts832nukp4vgcwk2gnc5cu9ayd": "100000000000000000000000000",
		"io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj": "100000000000000000000000000",
		"io1ed52svvdun2qv8sf2m0xnynuxfaulv6jlww7ur": "100000000000000000000000000",
		"io158hyzrmf4a8xll7gfc8xnwlv70jgp44tzy5nvd": "100000000000000000000000000",
		"io19kshh892255x4h5ularvr3q3al2v8cgl80fqrt": "100000000000000000000000000",
		"io1ph0u2psnd7muq5xv9623rmxdsxc4uapxhzpg02": "100000000000000000000000000",
		"io1znka733xefxjjw2wqddegplwtefun0mfdmz7dw": "100000000000000000000000000",
		"io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv": "100000000000000000000000000",
		"io14gnqxf9dpkn05g337rl7eyt2nxasphf5m6n0rd": "100000000000000000000000000",
		"io1l3wc0smczyay8xq747e2hw63mzg3ctp6uf8wsg": "100000000000000000000000000",
		"io1q4tdrahguffdu4e9j9aj4f38p2nee0r9vlhx7s": "100000000000000000000000000",
		"io1k9y4a9juk45zaqwvjmhtz6yjc68twqds4qcvzv": "100000000000000000000000000",
		"io15flratm0nhh5xpxz2lznrrpmnwteyd86hxdtj0": "100000000000000000000000000",
		"io1eq4ehs6xx6zj9gcsax7h3qydwlxut9xcfcjras": "100000000000000000000000000",
		"io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he": "100000000000000000000000000",
	}
}

// rowExists checks whether a row exists
func rowExists(db *sql.DB, query string, args ...interface{}) (bool, error) {
	var exists bool
	query = fmt.Sprintf("SELECT exists (%s)", query)
	stmt, err := db.Prepare(query)
	if err != nil {
		return false, errors.Wrap(err, "failed to prepare query")
	}
	defer stmt.Close()

	err = stmt.QueryRow(args...).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return false, errors.Wrap(err, "failed to query the row")
	}
	return exists, nil
}
