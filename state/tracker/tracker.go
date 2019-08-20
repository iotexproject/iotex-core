// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package tracker

import (
	"database/sql"
	"reflect"

	asql "github.com/iotexproject/iotex-core/db/sql"
)

type StateTracker struct {
	store   asql.Store
	changes []StateChange
}

//func (t *StateTracker) Start()

func (t *StateTracker) Commit(db *sql.DB) error {
	for _, c := range t.changes {
		err := c.Handle(db)
		if err != nil {
			return err
		}
	}
	return nil
}

type StateChange interface {
	Type() reflect.Type
	Handle(db *sql.DB) error
}

type BalanceChange string

func (b *BalanceChange) Type() reflect.Type {
	return reflect.TypeOf(b)
}

func (b *BalanceChange) Handle(db *sql.DB) error {
	// TODO: implement Handle()
	return nil
}
