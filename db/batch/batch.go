// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package batch

import "github.com/pkg/errors"

var (
	// ErrAlreadyDeleted indicates the key has been deleted
	ErrAlreadyDeleted = errors.New("already deleted from DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrOutOfBound indicates an out of bound error
	ErrOutOfBound = errors.New("out of bound")
	// ErrUnexpectedType indicates an invalid casting
	ErrUnexpectedType = errors.New("unexpected type")
)

type (
	// KVStoreBatch defines a batch buffer interface that stages Put/Delete entries in sequential order
	// To use it, first start a new batch
	// b := NewBatch()
	// and keep batching Put/Delete operation into it
	// b.Put(bucket, k, v)
	// b.Delete(bucket, k, v)
	// once it's done, call KVStore interface's WriteBatch() to persist to underlying DB
	// KVStore.WriteBatch(b)
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
		SerializeQueue(WriteInfoSerialize, WriteInfoFilter) []byte
		// Clear clears entries staged in batch
		Clear()
		// Translate clones the batch
		Translate(WriteInfoTranslate) KVStoreBatch
		// CheckFillPercent
		CheckFillPercent(string) (float64, bool)
		// AddFillPercent
		AddFillPercent(string, float64)
	}

	// CachedBatch derives from Batch interface
	// A local cache is added to provide fast retrieval of pending Put/Delete entries
	CachedBatch interface {
		KVStoreBatch
		Snapshot
		// Get gets a record by (namespace, key)
		Get(string, []byte) ([]byte, error)
	}

	Snapshot interface {
		// Snapshot takes a snapshot of current cached batch
		Snapshot() int
		// RevertSnapshot sets the cached batch to the state at the given snapshot
		RevertSnapshot(int) error
		// ResetSnapshots() clears all snapshots
		ResetSnapshots()
	}
)
