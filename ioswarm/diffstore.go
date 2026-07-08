package ioswarm

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

var diffBucket = []byte("diffs")

// DiffStore persists state diffs to a BoltDB file so that StreamStateDiffs
// can serve any historical range (not just the last N blocks from the ring buffer).
type DiffStore struct {
	db     *bolt.DB
	logger *zap.Logger

	mu           sync.RWMutex
	latestHeight uint64
	oldestHeight uint64
}

// OpenDiffStore opens or creates a DiffStore at the given path.
func OpenDiffStore(path string, logger *zap.Logger) (*DiffStore, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		NoSync:         false,
		NoFreelistSync: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open diffstore %s: %w", path, err)
	}

	// Ensure bucket exists
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(diffBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("create diffstore bucket: %w", err)
	}

	ds := &DiffStore{db: db, logger: logger}

	// Scan for oldest/latest heights
	ds.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(diffBucket)
		c := b.Cursor()
		if k, _ := c.First(); k != nil {
			ds.oldestHeight = binary.BigEndian.Uint64(k)
		}
		if k, _ := c.Last(); k != nil {
			ds.latestHeight = binary.BigEndian.Uint64(k)
		}
		return nil
	})

	logger.Info("diffstore opened",
		zap.String("path", path),
		zap.Uint64("oldest", ds.oldestHeight),
		zap.Uint64("latest", ds.latestHeight))

	return ds, nil
}

// heightKey encodes a block height as 8-byte big-endian for ordered iteration.
func heightKey(h uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, h)
	return key
}

// compressDiff gzip-compresses a JSON-encoded StateDiff.
func compressDiff(diff *StateDiff) ([]byte, error) {
	raw, err := json.Marshal(diff)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decompressDiff decompresses a gzip+JSON-encoded StateDiff.
func decompressDiff(data []byte) (*StateDiff, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var diff StateDiff
	if err := json.Unmarshal(raw, &diff); err != nil {
		return nil, err
	}
	return &diff, nil
}

// Append writes a state diff to the store. Idempotent by height —
// if a diff for this height already exists, it is silently overwritten.
func (ds *DiffStore) Append(diff *StateDiff) error {
	data, err := compressDiff(diff)
	if err != nil {
		return fmt.Errorf("compress diff at height %d: %w", diff.Height, err)
	}

	key := heightKey(diff.Height)
	if err := ds.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(diffBucket).Put(key, data)
	}); err != nil {
		return fmt.Errorf("write diff at height %d: %w", diff.Height, err)
	}

	ds.mu.Lock()
	if ds.oldestHeight == 0 || diff.Height < ds.oldestHeight {
		ds.oldestHeight = diff.Height
	}
	if diff.Height > ds.latestHeight {
		ds.latestHeight = diff.Height
	}
	ds.mu.Unlock()

	return nil
}

// Get retrieves a single state diff by height.
func (ds *DiffStore) Get(height uint64) (*StateDiff, error) {
	var data []byte
	if err := ds.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(diffBucket).Get(heightKey(height))
		if v == nil {
			return fmt.Errorf("diff at height %d not found", height)
		}
		data = make([]byte, len(v))
		copy(data, v)
		return nil
	}); err != nil {
		return nil, err
	}
	return decompressDiff(data)
}

// GetRange returns state diffs for heights [from, to] inclusive, ordered by height.
// Uses cursor seeking for efficient range scans.
func (ds *DiffStore) GetRange(from, to uint64) ([]*StateDiff, error) {
	var diffs []*StateDiff
	fromKey := heightKey(from)
	toKey := heightKey(to)

	if err := ds.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(diffBucket).Cursor()
		for k, v := c.Seek(fromKey); k != nil && bytes.Compare(k, toKey) <= 0; k, v = c.Next() {
			data := make([]byte, len(v))
			copy(data, v)
			diff, err := decompressDiff(data)
			if err != nil {
				return fmt.Errorf("decompress diff at key %x: %w", k, err)
			}
			diffs = append(diffs, diff)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return diffs, nil
}

// LatestHeight returns the highest stored height, or 0 if empty.
func (ds *DiffStore) LatestHeight() uint64 {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.latestHeight
}

// OldestHeight returns the lowest stored height, or 0 if empty.
func (ds *DiffStore) OldestHeight() uint64 {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.oldestHeight
}

// Prune deletes all diffs with height < keepAfter.
// Returns the number of entries deleted.
func (ds *DiffStore) Prune(keepAfter uint64) (int, error) {
	if keepAfter == 0 {
		return 0, nil
	}
	deleted := 0
	maxKey := heightKey(keepAfter - 1)

	if err := ds.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(diffBucket)
		c := b.Cursor()
		for k, _ := c.First(); k != nil && bytes.Compare(k, maxKey) <= 0; k, _ = c.Next() {
			if err := b.Delete(k); err != nil {
				return err
			}
			deleted++
		}
		return nil
	}); err != nil {
		return deleted, fmt.Errorf("prune diffstore: %w", err)
	}

	// Update oldest height
	ds.mu.Lock()
	if deleted > 0 {
		ds.db.View(func(tx *bolt.Tx) error {
			if k, _ := tx.Bucket(diffBucket).Cursor().First(); k != nil {
				ds.oldestHeight = binary.BigEndian.Uint64(k)
			} else {
				ds.oldestHeight = 0
				ds.latestHeight = 0
			}
			return nil
		})
	}
	ds.mu.Unlock()

	return deleted, nil
}

// Close closes the underlying BoltDB.
func (ds *DiffStore) Close() error {
	return ds.db.Close()
}
