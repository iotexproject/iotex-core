// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package tracker

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"

	asql "github.com/iotexproject/iotex-core/db/sql/analyticssql"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var specialActionHash = hash.ZeroHash256

// StateTracker defines an interface for state change track
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

// New creates a state tracker
func New(connectStr string, dbName string) StateTracker {
	tracker := &stateTracker{}
	tracker.store = asql.NewMySQL(connectStr, dbName)
	return tracker
}

// Start starts state tracker
func (t *stateTracker) Start(ctx context.Context) error {
	t.changes = make([]StateChange, 0)
	t.snapshot = 0

	if err := t.store.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start store")
	}
	if err := t.store.Transact(func(tx *sql.Tx) error {
		return BalanceChange{}.init(t.store.GetDB(), tx)
	}); err != nil {
		return errors.Wrap(err, "failed to init balance change tracker")
	}
	return nil
}

// Stop stops state tracker
func (t *stateTracker) Stop(ctx context.Context) error {
	return t.store.Stop(ctx)
}

// Append appends new state change
func (t *stateTracker) Append(c StateChange) {
	t.changes = append(t.changes, c)
}

// Snapshot records current status of changes
func (t *stateTracker) Snapshot() {
	t.snapshot = len(t.changes)
}

// Recover recovers state change to snapshot
func (t *stateTracker) Recover() {
	t.changes = t.changes[:t.snapshot]
}

// Clear deletes all state changes
func (t *stateTracker) Clear() {
	t.changes = make([]StateChange, 0)
	t.snapshot = 0
}

// Commit stores all state changes into db
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

// StateChange represents state change of state db
type StateChange interface {
	Type() reflect.Type
	init(*sql.DB) error
	handle(*sql.Tx, int) error
}
