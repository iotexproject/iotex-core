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

	asql "github.com/iotexproject/iotex-analytics/sql"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

type StateTracker interface {
	lifecycle.StartStopper

	Append(change StateChange)
	Clear()
	Commit() error
}

type stateTracker struct {
	store   asql.Store
	changes []StateChange
}

func New(connectStr string, dbName string) StateTracker {
	tracker := &stateTracker{}
	tracker.store = asql.NewMySQL(connectStr, dbName)
	return tracker
}

func (t *stateTracker) Start(ctx context.Context) error {
	return t.store.Start(ctx)
}

func (t *stateTracker) Stop(ctx context.Context) error {
	return t.store.Stop(ctx)
}

func (t *stateTracker) Append(c StateChange) {
	t.changes = append(t.changes, c)
}

func (t *stateTracker) Clear() {
	t.changes = make([]StateChange, 0)
}

func (t *stateTracker) Commit() error {
	db := t.store.GetDB()
	for _, c := range t.changes {
		err := c.handle(db)
		if err != nil {
			return err
		}
	}
	return nil
}

type StateChange interface {
	Type() reflect.Type
	handle(db *sql.DB) error
}

type BalanceChange struct {
	address   string
	inAmount  string
	outAmount string
}

func (b *BalanceChange) Type() reflect.Type {
	return reflect.TypeOf(b)
}

func (b *BalanceChange) handle(db *sql.DB) error {
	// TODO: implement Handle()
	return nil
}
