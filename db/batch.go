// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

const (
	// Put indicate the type of write operation to be Put
	Put WriteType = iota
	// Delete indicate the type of write operation to be Delete
	Delete
)

type (
	// WriteType is the type of write
	WriteType int32

	// WriteInfo is the struct to store Put/Delete operation info
	WriteInfo struct {
		writeType   WriteType
		namespace   string
		key         []byte
		value       []byte
		errorFormat string
		errorArgs   interface{}
	}

	// KVStoreBatch defines a batch buffer interface that stages Put/Delete entries in sequential order
	// To use it, first start a new batch
	// b := NewBatch()
	// and keep batching Put/Delete operation into it
	// b.Put(bucket, k, v)
	// b.Delete(bucket, k, v)
	// once it's done, call KVStore interface's Commit() to persist to underlying DB
	// KVStore.Commit(b)
	// if commit succeeds, the batch is cleared
	// otherwise the batch is kept intact (so batch user can figure out what’s wrong and attempt re-commit later)
	KVStoreBatch interface {
		// Lock locks the batch
		Lock()
		// Unlock unlocks the batch
		Unlock()
		// ClearAndUnlock clears the write queue and unlocks the batch
		ClearAndUnlock()
		// Put insert or update a record identified by (namespace, key)
		Put(string, []byte, []byte, string, ...interface{})
		// Delete deletes a record by (namespace, key)
		Delete(string, []byte, string, ...interface{})
		// Size returns the size of batch
		Size() int
		// Entry returns the entry at the index
		Entry(int) (*WriteInfo, error)
		// SerializeQueue serialize the writes in queue
		SerializeQueue() []byte
		// ExcludeEntries returns copy of batch with certain entries excluded
		ExcludeEntries(string, WriteType) KVStoreBatch
		// Clear clears entries staged in batch
		Clear()
		// CloneBatch clones the batch
		CloneBatch() KVStoreBatch
		// batch puts an entry into the write queue
		batch(op WriteType, namespace string, key, value []byte, errorFormat string, errorArgs ...interface{})
		// truncate the write queue
		truncate(int)
	}

	// CachedBatch derives from Batch interface
	// A local cache is added to provide fast retrieval of pending Put/Delete entries
	CachedBatch interface {
		KVStoreBatch
		// Get gets a record by (namespace, key)
		Get(string, []byte) ([]byte, error)
		// Snapshot takes a snapshot of current cached batch
		Snapshot() int
		// Revert sets the cached batch to the state at the given snapshot
		Revert(int) error
	}
)

func (wi *WriteInfo) serialize() []byte {
	bytes := make([]byte, 0)
	bytes = append(bytes, []byte(wi.namespace)...)
	bytes = append(bytes, wi.key...)
	bytes = append(bytes, wi.value...)
	return bytes
}
