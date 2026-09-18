package factory

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
)

type readOnlyKV struct {
	db.KVStore
}

var _ db.KVStoreWithRangeScan = (*readOnlyKV)(nil)

func (r *readOnlyKV) Put(string, []byte, []byte) error {
	return db.ErrNotSupported
}

func (r *readOnlyKV) Delete(string, []byte) error {
	return db.ErrNotSupported
}

func (r *readOnlyKV) WriteBatch(batch.KVStoreBatch) error {
	return db.ErrNotSupported
}

// ScanRange forwards to the wrapped store.
//
// Embedding db.KVStore only promotes the KVStore method set, so without this the
// wrapper silently drops the ScanRange capability of the store it wraps: a
// read-only working set (WorkingSet / WorkingSetAtHeight / WorkingSetAtTransaction,
// i.e. every API read and simulation) would fail any ordered range scan that the
// very same DAO answers fine for a block-producing working set. Range scans are
// reads, so there is nothing here to make read-only.
func (r *readOnlyKV) ScanRange(ns string, min, max []byte, limit int) ([][]byte, [][]byte, error) {
	scanner, ok := r.KVStore.(db.KVStoreWithRangeScan)
	if !ok {
		return nil, nil, errors.Wrapf(db.ErrNotSupported, "kvstore %T does not support ScanRange", r.KVStore)
	}
	return scanner.ScanRange(ns, min, max, limit)
}
