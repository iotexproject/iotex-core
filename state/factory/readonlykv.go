package factory

import (
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
)

type readOnlyKV struct {
	db.KVStore
}

func (r *readOnlyKV) Put(string, []byte, []byte) error {
	return db.ErrNotSupported
}

func (r *readOnlyKV) Delete(string, []byte) error {
	return db.ErrNotSupported
}

func (r *readOnlyKV) WriteBatch(batch.KVStoreBatch) error {
	return db.ErrNotSupported
}
