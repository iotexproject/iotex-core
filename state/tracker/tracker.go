// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package tracker

import (
	"database/sql"
	"reflect"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"

	asql "github.com/iotexproject/iotex-core/db/sql/analyticssql"
)

var specialActionHash = hash.ZeroHash256

// StateChange represents state change of state db
type StateChange interface {
	Type() reflect.Type
	init(*sql.DB, *sql.Tx) error
	handle(*sql.Tx, uint64) error
}

// StateTracker defines an interface for state change track
type StateTracker interface {
	Append(StateChange)
	Snapshot()
	Revert(int) error
	Commit(uint64) error
}

type stateTracker struct {
	store     asql.Store
	changes   []StateChange
	snapshots []int
	mutex     sync.RWMutex
}

// InitStore initializes state tracker store
func InitStore(store asql.Store) error {
	if err := store.Transact(func(tx *sql.Tx) error {
		// TODO: we may need other state changes' initializations later
		return BalanceChange{}.init(store.GetDB(), tx)
	}); err != nil {
		return errors.Wrap(err, "failed to init balance change tracker")
	}
	return nil
}

// New creates a state tracker
func New(store asql.Store) StateTracker {
	return &stateTracker{store: store,
		changes:   make([]StateChange, 0),
		snapshots: make([]int, 0),
	}
}

// Append appends new state change
func (t *stateTracker) Append(c StateChange) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.changes = append(t.changes, c)
}

// Snapshot records current status of changes
func (t *stateTracker) Snapshot() {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	t.snapshots = append(t.snapshots, len(t.changes))
}

// Recover recovers state change to snapshot
func (t *stateTracker) Revert(snapshot int) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if snapshot < 0 || snapshot >= len(t.snapshots) {
		return errors.Errorf("invalid state tracker snapshot number = %d", snapshot)
	}
	t.snapshots = t.snapshots[:snapshot+1]
	t.changes = t.changes[:t.snapshots[snapshot]]
	return nil
}

// Commit stores all state changes into db
func (t *stateTracker) Commit(height uint64) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if err := t.store.Transact(func(tx *sql.Tx) error {
		for _, c := range t.changes {
			if err := c.handle(tx, height); err != nil {
				return errors.Wrap(err, "failed to handle state change")
			}
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to store state changes")
	}
	t.changes = make([]StateChange, 0)
	t.snapshots = make([]int, 0)
	return nil
}
