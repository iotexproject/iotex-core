// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"database/sql"

	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-election/types"
)

type electionTx struct {
	op committee.Operator
	tx *sql.Tx
}

func newElectionTx(op committee.Operator, tx *sql.Tx) *electionTx {
	return &electionTx{op: op, tx: tx}
}

func (etx *electionTx) Commit() error {
	return etx.tx.Commit()
}

func (etx *electionTx) Rollback() error {
	return etx.tx.Rollback()
}

func (etx *electionTx) StoreBuckets(height uint64, buckets []*types.Bucket) error {
	return etx.op.Put(height, buckets, etx.tx)
}
