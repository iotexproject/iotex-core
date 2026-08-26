package db

import (
	"bytes"
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db/batch"
)

type (
	withBuffer interface {
		batch.Snapshot
		SerializeQueue(batch.WriteInfoSerialize, batch.WriteInfoFilter) []byte
		MustPut(string, []byte, []byte)
		MustDelete(string, []byte)
		Size() int
		Entry(int) (*batch.WriteInfo, error)
	}

	// KVStoreWithBuffer defines a KVStore with a buffer, which enables snapshot, revert,
	// and transaction with multiple writes
	KVStoreWithBuffer interface {
		KVStore
		withBuffer
	}

	// kvStoreWithBuffer is an implementation of KVStore, which buffers all the changes
	kvStoreWithBuffer struct {
		store  KVStore
		buffer batch.CachedBatch
	}

	// KVStoreFlusher is a wrapper of KVStoreWithBuffer, which has flush api
	KVStoreFlusher interface {
		SerializeQueue() []byte
		Flush() error
		KVStoreWithBuffer() KVStoreWithBuffer
		BaseKVStore() KVStore
	}

	flusher struct {
		kvb             *kvStoreWithBuffer
		serializeFilter batch.WriteInfoFilter
		serialize       batch.WriteInfoSerialize
		flushTranslate  batch.WriteInfoTranslate
	}

	// KVStoreFlusherOption sets option for KVStoreFlusher
	KVStoreFlusherOption func(*flusher) error
)

// SerializeFilterOption sets the filter for serialize write queue
func SerializeFilterOption(filter batch.WriteInfoFilter) KVStoreFlusherOption {
	return func(f *flusher) error {
		if filter == nil {
			return errors.New("filter cannot be nil")
		}
		f.serializeFilter = filter

		return nil
	}
}

// SerializeOption sets the serialize function for write queue
func SerializeOption(wis batch.WriteInfoSerialize) KVStoreFlusherOption {
	return func(f *flusher) error {
		if wis == nil {
			return errors.New("serialize function cannot be nil")
		}
		f.serialize = wis

		return nil
	}
}

// FlushTranslateOption sets the translate for flush
func FlushTranslateOption(wit batch.WriteInfoTranslate) KVStoreFlusherOption {
	return func(f *flusher) error {
		if wit == nil {
			return errors.New("translate cannot be nil")
		}
		f.flushTranslate = wit

		return nil
	}
}

// NewKVStoreFlusher returns kv store flusher
func NewKVStoreFlusher(store KVStore, buffer batch.CachedBatch, opts ...KVStoreFlusherOption) (KVStoreFlusher, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}
	if buffer == nil {
		return nil, errors.New("buffer cannot be nil")
	}
	f := &flusher{
		kvb: &kvStoreWithBuffer{
			store:  store,
			buffer: buffer,
		},
	}
	for _, opt := range opts {
		if err := opt(f); err != nil {
			return nil, errors.Wrap(err, "failed to apply option")
		}
	}

	return f, nil
}

func (f *flusher) Flush() error {
	if err := f.kvb.store.WriteBatch(f.kvb.buffer.Translate(f.flushTranslate)); err != nil {
		return err
	}

	f.kvb.buffer.Lock()
	f.kvb.buffer.ClearAndUnlock()

	return nil
}

func (f *flusher) SerializeQueue() []byte {
	return f.kvb.SerializeQueue(f.serialize, f.serializeFilter)
}

func (f *flusher) KVStoreWithBuffer() KVStoreWithBuffer {
	return f.kvb
}

func (f *flusher) BaseKVStore() KVStore {
	return f.kvb.store
}

func (kvb *kvStoreWithBuffer) Start(ctx context.Context) error {
	return kvb.store.Start(ctx)
}

func (kvb *kvStoreWithBuffer) Stop(ctx context.Context) error {
	return kvb.store.Stop(ctx)
}

func (kvb *kvStoreWithBuffer) Snapshot() int {
	return kvb.buffer.Snapshot()
}

func (kvb *kvStoreWithBuffer) RevertSnapshot(sid int) error {
	return kvb.buffer.RevertSnapshot(sid)
}

func (kvb *kvStoreWithBuffer) ResetSnapshots() {
	kvb.buffer.ResetSnapshots()
}

func (kvb *kvStoreWithBuffer) SerializeQueue(
	serialize batch.WriteInfoSerialize,
	filter batch.WriteInfoFilter,
) []byte {
	return kvb.buffer.SerializeQueue(serialize, filter)
}

func (kvb *kvStoreWithBuffer) Size() int {
	return kvb.buffer.Size()
}

func (kvb *kvStoreWithBuffer) Entry(i int) (*batch.WriteInfo, error) {
	return kvb.buffer.Entry(i)
}

func (kvb *kvStoreWithBuffer) Get(ns string, key []byte) ([]byte, error) {
	value, err := kvb.buffer.Get(ns, key)
	if errors.Cause(err) == batch.ErrNotExist {
		value, err = kvb.store.Get(ns, key)
	}
	if errors.Cause(err) == batch.ErrAlreadyDeleted {
		err = errors.Wrapf(ErrNotExist, "failed to get key %x in %s, deleted in buffer level", key, ns)
	}
	return value, err
}

func (kvb *kvStoreWithBuffer) Put(ns string, key, value []byte) error {
	kvb.buffer.Put(ns, key, value, fmt.Sprintf("failed to put %x in %s", key, ns))
	return nil
}

func (kvb *kvStoreWithBuffer) MustPut(ns string, key, value []byte) {
	kvb.buffer.Put(ns, key, value, fmt.Sprintf("failed to put %x in %s", key, ns))
}

func (kvb *kvStoreWithBuffer) Delete(ns string, key []byte) error {
	kvb.buffer.Delete(ns, key, fmt.Sprintf("failed to delete %x in %s", key, ns))
	return nil
}

func (kvb *kvStoreWithBuffer) MustDelete(ns string, key []byte) {
	kvb.buffer.Delete(ns, key, fmt.Sprintf("failed to delete %x in %s", key, ns))
}

func (kvb *kvStoreWithBuffer) Filter(ns string, cond Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	fk, fv, err := kvb.store.Filter(ns, cond, minKey, maxKey)
	if err != nil {
		return fk, fv, err
	}

	// filter the entries in buffer
	checkMin := len(minKey) > 0
	checkMax := len(maxKey) > 0
	for i := 0; i < kvb.buffer.Size(); i++ {
		entry, err := kvb.buffer.Entry(i)
		if err != nil {
			return nil, nil, err
		}
		if entry.Namespace() != ns {
			continue
		}
		k, v := entry.Key(), entry.Value()

		if checkMin && bytes.Compare(k, minKey) == -1 {
			continue
		}
		if checkMax && bytes.Compare(k, maxKey) == 1 {
			continue
		}

		if cond(k, v) {
			switch entry.WriteType() {
			case batch.Put:
				// if DB contains the same key, that should be obsoleted
				for i := range fk {
					if bytes.Equal(fk[i], k) {
						fk = append(fk[:i], fk[i+1:]...)
						fv = append(fv[:i], fv[i+1:]...)
						break
					}
				}
				fk = append(fk, k)
				fv = append(fv, v)
			case batch.Delete:
				for i := range fk {
					if bytes.Equal(fk[i], k) {
						fk = append(fk[:i], fk[i+1:]...)
						fv = append(fv[:i], fv[i+1:]...)
						break
					}
				}
			}
		}
	}
	return fk, fv, nil
}

// scanEntryInRange reports whether a buffer entry takes part in a ScanRange over
// ns and [min, max). It is the single definition of that predicate on purpose:
// ScanRange's merge loop and the Delete count that sizes its base scan both call
// it, so the bound handed to the base store can never drift from the set of
// entries actually replayed on top of the result.
func scanEntryInRange(entry *batch.WriteInfo, ns string, min, max []byte) bool {
	return entry.Namespace() == ns && inScanRange(entry.Key(), min, max)
}

// bufferedScanEntries snapshots only the entries relevant to one range scan
// under the CachedBatch lock, and counts its Deletes in the same pass. A range
// scan must use one immutable view for both its base-limit calculation and its
// final replay: Flush, Clear, or RevertSnapshot may otherwise shrink the queue
// between Size and Entry and either lose writes or return batch.ErrOutOfBound.
// Filtering while locked also avoids allocating a queue-sized pointer slice for
// every narrow shard scan.
func (kvb *kvStoreWithBuffer) bufferedScanEntries(
	ns string,
	min, max []byte,
) ([]*batch.WriteInfo, int, error) {
	kvb.buffer.Lock()
	defer kvb.buffer.Unlock()

	var (
		entries []*batch.WriteInfo
		deletes int
	)
	for i, size := 0, kvb.buffer.Size(); i < size; i++ {
		entry, err := kvb.buffer.Entry(i)
		if err != nil {
			return nil, 0, err
		}
		if !scanEntryInRange(entry, ns, min, max) {
			continue
		}
		entries = append(entries, entry)
		if entry.WriteType() == batch.Delete {
			deletes++
		}
	}
	return entries, deletes, nil
}

// ScanRange returns up to limit <k, v> pairs in [min, max), ascending by bytes.Compare(k),
// merging the pending write buffer on top of the base store.
// See KVStoreWithRangeScan for the exact semantics, which must stay identical across engines.
func (kvb *kvStoreWithBuffer) ScanRange(ns string, min, max []byte, limit int) ([][]byte, [][]byte, error) {
	// The capability check comes BEFORE the empty-range fast path on purpose. An
	// unscannable base is a node-local fact about the binary/config, not about
	// chain state, so it must surface identically no matter which bounds the
	// caller happened to pass. Answering "empty, no error" for a provably empty
	// interval would hide a broken node until the first non-empty query, and the
	// nodes that hide it are not the same nodes that hit it.
	scanner, ok := kvb.store.(KVStoreWithRangeScan)
	if !ok {
		return nil, nil, errors.Wrapf(ErrNotSupported, "base store %T does not support ScanRange", kvb.store)
	}
	if emptyScanRange(min, max) {
		return nil, nil, nil
	}
	entries, deletes, err := kvb.bufferedScanEntries(ns, min, max)
	if err != nil {
		return nil, nil, err
	}
	// DO NOT push the caller's raw limit down to the base scan. The buffer can
	// contain keys that sort before a truncated base scan's cutoff, and each of
	// those displaces a base key that must then be handed back instead -- a base
	// scan cut at exactly limit would have already thrown those keys away.
	// Truncation of the RESULT still happens exactly once, after the merge and
	// the sort.
	//
	// What is safe is a strictly larger bound. Let d be the number of Delete
	// entries in the buffer that this scan would replay (same namespace, key in
	// [min, max)), and scan the base with M = limit + d:
	//
	//  1. A base scan truncated at M returns the first M base keys of the range
	//     in ascending order; call the last one b_M. Every base key it dropped
	//     is > b_M, and the buffer replay is identical either way, so the
	//     truncated and the untruncated merges are byte-identical on every key
	//     <= b_M.
	//  2. At least M - d merged keys are <= b_M: we started from M base keys,
	//     only a Delete can remove one, and a Put either overrides an existing
	//     key (count unchanged) or adds a new one (count grows).
	//  3. With M = limit + d that leaves at least limit correct keys at or below
	//     b_M, so the first limit results equal the unlimited computation. Keys
	//     the buffer adds above b_M exist in the truncated merge but can never
	//     reach the first limit slots.
	//  4. If the base returns fewer than M keys the range was exhausted, so the
	//     two computations are trivially identical.
	//
	// Over-counting d is always safe -- it only fetches a few extra base keys.
	// Under-counting it is a correctness bug. So do NOT "optimize" the count by
	// de-duplicating repeated Deletes of the same key, by skipping Deletes for
	// keys the base may not hold, or by collapsing a Delete-then-Put pair: every
	// qualifying Delete entry must be counted, once per entry.
	baseLimit := 0
	if limit > 0 {
		baseLimit = limit + deletes
	}
	baseKeys, baseValues, err := scanner.ScanRange(ns, min, max, baseLimit)
	if err != nil {
		return nil, nil, err
	}

	merged := make(map[string][]byte, len(baseKeys))
	for i, k := range baseKeys {
		merged[string(k)] = baseValues[i]
	}
	// replay the buffer in write order, so the last write to a key wins
	for _, entry := range entries {
		ks := string(entry.Key())
		switch entry.WriteType() {
		case batch.Put:
			// a Put overrides the base value, or adds a key the base does not have
			merged[ks] = copyBytes(entry.Value())
		case batch.Delete:
			// a Delete removes the key regardless of whether the base has it
			delete(merged, ks)
		}
	}

	// map iteration order is random, but the keys of a map are unique, so the
	// subsequent sort by bytes.Compare is a total order and the output is
	// deterministic.
	keys := make([][]byte, 0, len(merged))
	values := make([][]byte, 0, len(merged))
	for ks, v := range merged {
		keys = append(keys, []byte(ks))
		values = append(values, v)
	}
	k, v := sortAndTruncateScan(keys, values, limit)
	return k, v, nil
}

func (kvb *kvStoreWithBuffer) WriteBatch(b batch.KVStoreBatch) (err error) {
	kvb.buffer.Append(b)
	return nil
}
