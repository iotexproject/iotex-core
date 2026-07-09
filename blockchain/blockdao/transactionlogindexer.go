// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// TransactionLogIndexer is a read-only, startup-loaded override for block
// transaction (system) logs. It stores corrected iotextypes.TransactionLogs keyed
// by block height; only the patched heights are present (the set is sparse, heights
// need not be contiguous). Transaction logs are not part of any receipt or state
// root, so overriding them for the patched heights does not affect consensus.
type TransactionLogIndexer struct {
	kvstore db.KVStore
	// heights is the in-memory set of patched heights, populated at Start when the
	// preload option is enabled (nil otherwise). It lets TransactionLogs answer the
	// common "height is not patched" query without touching the DB. It is built once
	// at Start and read-only afterwards (the node opens the store read-only).
	heights map[uint64]struct{}
	preload bool
}

const _txLogsNS = "txlogs"

// TransactionLogIndexerOption configures a TransactionLogIndexer.
type TransactionLogIndexerOption func(*TransactionLogIndexer)

// WithPreloadHeights preloads, at Start, the set of heights that have patched data
// into memory. Queries for heights not in the set then return without a DB lookup.
// This is worthwhile for a small, sparse patch where the vast majority of queries
// are for unpatched heights. Assumes the store is not written after Start (true for
// the read-only node patch).
func WithPreloadHeights() TransactionLogIndexerOption {
	return func(ti *TransactionLogIndexer) { ti.preload = true }
}

// NewTransactionLogIndexer creates a new transaction log indexer
func NewTransactionLogIndexer(kvstore db.KVStore, opts ...TransactionLogIndexerOption) *TransactionLogIndexer {
	ti := &TransactionLogIndexer{kvstore: kvstore}
	for _, opt := range opts {
		opt(ti)
	}
	return ti
}

// Start starts the transaction log indexer
func (ti *TransactionLogIndexer) Start(ctx context.Context) error {
	if err := ti.kvstore.Start(ctx); err != nil {
		return err
	}
	if !ti.preload {
		return nil
	}
	heights := make(map[uint64]struct{})
	keys, _, err := ti.kvstore.Filter(_txLogsNS, func(k, v []byte) bool { return true }, nil, nil)
	switch errors.Cause(err) {
	case nil:
		for _, k := range keys {
			heights[byteutil.BytesToUint64(k)] = struct{}{}
		}
	case db.ErrNotExist, db.ErrBucketNotExist:
		// empty patch -> empty set
	default:
		return err
	}
	ti.heights = heights
	return nil
}

// Stop stops the transaction log indexer
func (ti *TransactionLogIndexer) Stop(ctx context.Context) error {
	return ti.kvstore.Stop(ctx)
}

// Put stores the corrected transaction logs for a block height. Used by the patch
// generator; must not be called after Start when preloading is enabled.
func (ti *TransactionLogIndexer) Put(height uint64, logs *iotextypes.TransactionLogs) error {
	if logs == nil {
		logs = &iotextypes.TransactionLogs{}
	}
	value, err := proto.Marshal(logs)
	if err != nil {
		return err
	}
	return ti.kvstore.Put(_txLogsNS, byteutil.Uint64ToBytes(height), value)
}

// TransactionLogs returns the corrected transaction logs at the given height.
// It returns db.ErrNotExist / db.ErrBucketNotExist when the height is not patched;
// callers use that to fall back to the main block store.
func (ti *TransactionLogIndexer) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	// fast path: when the patched-height set is preloaded, a height that is not in
	// it is answered without a DB lookup (the common case for a sparse patch).
	if ti.heights != nil {
		if _, ok := ti.heights[height]; !ok {
			return nil, db.ErrNotExist
		}
	}
	value, err := ti.kvstore.Get(_txLogsNS, byteutil.Uint64ToBytes(height))
	if err != nil {
		return nil, err
	}
	logs := &iotextypes.TransactionLogs{}
	if err := proto.Unmarshal(value, logs); err != nil {
		return nil, err
	}
	return logs, nil
}
