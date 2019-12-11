// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package tracker

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"os"
	"reflect"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
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

func (b BalanceChange) init(ctx context.Context, db *sql.DB, tx *sql.Tx) error {
	trackerCtx := MustGetTrackerCtx(ctx)
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

	initBalance := trackerCtx.Genesis.InitBalanceMap
	for addr, amount := range initBalance {
		insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height, action_hash, action_type, `from`, `to`, amount) VALUES (?, ?, ?, ?, ?, ?, ?)",
			BalanceHistoryTableName)
		if _, err := tx.Exec(insertQuery, uint64(0), uint64(0), hex.EncodeToString(specialActionHash[:]), "genesis", "", addr, amount); err != nil {
			return errors.Wrapf(err, "failed to update balance history for address %s", addr)
		}
	}
	return nil
}

func (b BalanceChange) handle(ctx context.Context, tx *sql.Tx, blockHeight uint64) error {
	trackerCtx := MustGetTrackerCtx(ctx)
	epochNumber := getEpochNumber(trackerCtx.Genesis.Blockchain, blockHeight)
	actionType := "execution"
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height, action_hash, action_type, `from`, `to`, amount) VALUES (?, ?, ?, ?, ?, ?, ?)",
		BalanceHistoryTableName)
	if _, err := tx.Exec(insertQuery, epochNumber, blockHeight, hex.EncodeToString(b.ActionHash[:]), actionType, b.OutAddr, b.InAddr, b.Amount); err != nil {
		return errors.Wrap(err, "failed to update balance history")
	}
	return nil
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

func getEpochNumber(cfg genesis.Blockchain, height uint64) uint64 {
	if height == 0 {
		return 0
	}
	if height <= cfg.DardanellesBlockHeight {
		return (height-1)/cfg.NumDelegates/cfg.NumSubEpochs + 1
	}
	dardanellesEpoch := getEpochNumber(cfg, cfg.DardanellesBlockHeight)
	dardanellesEpochHeight := getEpochHeight(cfg, dardanellesEpoch)
	return dardanellesEpoch + (height-dardanellesEpochHeight)/cfg.NumDelegates/cfg.DardanellesNumSubEpochs
}

func getEpochHeight(cfg genesis.Blockchain, epochNum uint64) uint64 {
	if epochNum == 0 {
		return 0
	}
	dardanellesEpoch := getEpochNumber(cfg, cfg.DardanellesBlockHeight)
	if epochNum <= dardanellesEpoch {
		return (epochNum-1)*cfg.NumDelegates*cfg.NumSubEpochs + 1
	}
	dardanellesEpochHeight := getEpochHeight(cfg, dardanellesEpoch)
	return dardanellesEpochHeight + (epochNum-dardanellesEpoch)*cfg.NumDelegates*cfg.DardanellesNumSubEpochs
}
