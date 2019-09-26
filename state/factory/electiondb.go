// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"database/sql"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-election/committee"
)

type electionDB struct {
	op committee.Operator
	db *sql.DB
}

func newElectionDB(tableName string, interval uint64, db *sql.DB) (BranchDB, error) {
	op, err := committee.NewBucketTableOperator(tableName, interval)
	if err != nil {
		return nil, err
	}
	return &electionDB{op: op, db: db}, nil
}

func (edb *electionDB) NewTransaction() (protocol.Transaction, error) {
	tx, err := edb.db.Begin()
	if err != nil {
		return nil, err
	}
	return newElectionTx(edb.op, tx), nil
}