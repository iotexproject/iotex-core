// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestTransactionLogIndexer(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	testPath, err := testutil.PathOfTempFile("test-txlog-indexer")
	r.NoError(err)
	defer testutil.CleanupPath(testPath)

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	ti := NewTransactionLogIndexer(db.NewBoltDB(cfg))
	r.NoError(ti.Start(ctx))

	corrected := func(ah byte) *iotextypes.TransactionLogs {
		return &iotextypes.TransactionLogs{Logs: []*iotextypes.TransactionLog{{
			ActionHash:      []byte{ah},
			NumTransactions: 2,
			Transactions: []*iotextypes.TransactionLog_Transaction{
				{Amount: "38500000000000000", Sender: "ioSender", Recipient: "io0000000000000000000000rewardingprotocol", Type: iotextypes.TransactionLogType_GAS_FEE},
				{Amount: "30800000000000000", Sender: "ioSender", Recipient: "io0000000000000000000000rewardingprotocol", Type: iotextypes.TransactionLogType_PRIORITY_FEE},
			},
		}}}
	}

	// sparse, non-contiguous heights
	r.NoError(ti.Put(50, corrected(0xaa)))
	r.NoError(ti.Put(90, corrected(0xbb)))

	got, err := ti.TransactionLogs(50)
	r.NoError(err)
	r.Len(got.GetLogs(), 1)
	r.Len(got.GetLogs()[0].GetTransactions(), 2)
	r.Equal(iotextypes.TransactionLogType_GAS_FEE, got.GetLogs()[0].GetTransactions()[0].GetType())
	r.Equal(iotextypes.TransactionLogType_PRIORITY_FEE, got.GetLogs()[0].GetTransactions()[1].GetType())

	// an unpatched height (including one between two patched heights) -> not-exist signal
	for _, h := range []uint64{51, 89, 1_000_000} {
		_, err = ti.TransactionLogs(h)
		c := errors.Cause(err)
		r.Truef(c == db.ErrNotExist || c == db.ErrBucketNotExist, "height %d: got %v", h, err)
	}

	// data persists across restart
	r.NoError(ti.Stop(ctx))
	r.NoError(ti.Start(ctx))
	got, err = ti.TransactionLogs(90)
	r.NoError(err)
	r.Len(got.GetLogs(), 1)
	r.NoError(ti.Stop(ctx))

	// reopen with height preloading: the in-memory set is built at Start, and the
	// same results are served (patched heights hit the DB, others short-circuit).
	tp := NewTransactionLogIndexer(db.NewBoltDB(cfg), WithPreloadHeights())
	r.NoError(tp.Start(ctx))
	r.Equal(map[uint64]struct{}{50: {}, 90: {}}, tp.heights)
	got, err = tp.TransactionLogs(50)
	r.NoError(err)
	r.Len(got.GetLogs()[0].GetTransactions(), 2)
	for _, h := range []uint64{51, 89, 1_000_000} {
		_, err = tp.TransactionLogs(h)
		r.Equal(db.ErrNotExist, errors.Cause(err))
	}
	r.NoError(tp.Stop(ctx))
}

func TestTransactionLogIndexerPreloadEmpty(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	testPath, err := testutil.PathOfTempFile("test-txlog-indexer-empty")
	r.NoError(err)
	defer testutil.CleanupPath(testPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	// preload against an empty store must not error (empty set)
	ti := NewTransactionLogIndexer(db.NewBoltDB(cfg), WithPreloadHeights())
	r.NoError(ti.Start(ctx))
	r.Empty(ti.heights)
	_, err = ti.TransactionLogs(42)
	r.Equal(db.ErrNotExist, errors.Cause(err))
	r.NoError(ti.Stop(ctx))
}
