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
	"reflect"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-analytics/indexprotocol/accounts"
	asql "github.com/iotexproject/iotex-analytics/sql"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

type StateTracker interface {
	lifecycle.StartStopper

	Append(StateChange)
	Snapshot()
	Recover()
	Clear()
	Commit(int) error
}

type stateTracker struct {
	store    asql.Store
	changes  []StateChange
	snapshot int
}

func New(connectStr string, dbName string) StateTracker {
	tracker := &stateTracker{}
	tracker.store = asql.NewMySQL(connectStr, dbName)
	return tracker
}

func (t *stateTracker) Start(ctx context.Context) error {
	t.changes = make([]StateChange, 0)
	t.snapshot = 0
	return t.store.Start(ctx)
}

func (t *stateTracker) Stop(ctx context.Context) error {
	return t.store.Stop(ctx)
}

func (t *stateTracker) Append(c StateChange) {
	t.changes = append(t.changes, c)
}

func (t *stateTracker) Snapshot() {
	t.snapshot = len(t.changes)
}

func (t *stateTracker) Recover() {
	t.changes = t.changes[:t.snapshot]
}

func (t *stateTracker) Clear() {
	t.changes = make([]StateChange, 0)
	t.snapshot = 0
}

func (t *stateTracker) Commit(height int) error {
	if err := t.store.Transact(func(tx *sql.Tx) error {
		for _, c := range t.changes {
			err := c.handle(tx, height)
			if err != nil {
				return errors.Wrap(err, "failed to handle state change")
			}
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to store state changes")
	}
	t.Clear()
	return nil
}

type StateChange interface {
	Type() reflect.Type
	handle(*sql.Tx, int) error
}

type BalanceChange struct {
	Amount     string
	InAddr     string
	OutAddr    string
	ActionHash hash.Hash256
}

func (b BalanceChange) Type() reflect.Type {
	return reflect.TypeOf(b)
}

func (b BalanceChange) handle(tx *sql.Tx, blockHeight int) error {
	epochNumber := 0
	if blockHeight != 0 {
		epochNumber = (blockHeight-1)/int(genesis.Default.NumDelegates)/int(genesis.Default.NumSubEpochs) + 1
	}
	if b.InAddr != "" {
		insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height, action_hash, address, `in`) VALUES (?, ?, ?, ?, ?)",
			accounts.AccountHistoryTableName)
		result, e := tx.Exec(insertQuery, epochNumber, blockHeight, hex.EncodeToString(b.ActionHash[:]), b.InAddr, b.Amount)
		if _, err := result, e; err != nil {
			return errors.Wrapf(err, "failed to update account history for address %s", b.InAddr)
		}
	}
	if b.OutAddr != "" {
		insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_number, block_height, action_hash, address, `out`) VALUES (?, ?, ?, ?, ?)",
			accounts.AccountHistoryTableName)
		if _, err := tx.Exec(insertQuery, epochNumber, blockHeight, hex.EncodeToString(b.ActionHash[:]), b.OutAddr, b.Amount); err != nil {
			return errors.Wrapf(err, "failed to update account history for address %s", b.OutAddr)
		}
	}
	return nil
}
