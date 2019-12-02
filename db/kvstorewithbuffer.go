package db

import (
	"context"

	"github.com/pkg/errors"
)

type (
	withBuffer interface {
		Snapshot() int
		Revert(int) error
		SerializeQueue() []byte
		MustPut(string, []byte, []byte)
		MustDelete(string, []byte)
		Size() int
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
		buffer CachedBatch
	}

	// KVStoreFlusher is a wrapper of KVStoreWithBuffer, which has flush api
	KVStoreFlusher interface {
		withBuffer
		Get(string, []byte) ([]byte, error)
		Flush() error
		KVStoreWithBuffer() KVStoreWithBuffer
	}

	flusher struct {
		kvb *kvStoreWithBuffer
	}
)

// NewKVStoreFlusher returns kv store flusher
func NewKVStoreFlusher(store KVStore, buffer CachedBatch) (KVStoreFlusher, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}
	if buffer == nil {
		return nil, errors.New("buffer cannot be nil")
	}

	return &flusher{
		kvb: &kvStoreWithBuffer{
			store:  store,
			buffer: buffer,
		},
	}, nil
}

func (f *flusher) Flush() error {
	return f.kvb.store.WriteBatch(f.kvb.buffer)
}

func (f *flusher) SerializeQueue() []byte {
	return f.kvb.SerializeQueue()
}

func (f *flusher) MustPut(ns string, key, value []byte) {
	f.kvb.MustPut(ns, key, value)
}

func (f *flusher) MustDelete(ns string, key []byte) {
	f.kvb.MustDelete(ns, key)
}

func (f *flusher) Get(ns string, key []byte) ([]byte, error) {
	return f.kvb.Get(ns, key)
}

func (f *flusher) Snapshot() int {
	return f.kvb.Snapshot()
}

func (f *flusher) Revert(si int) error {
	return f.kvb.Revert(si)
}

func (f *flusher) Size() int {
	return f.kvb.Size()
}

func (f *flusher) KVStoreWithBuffer() KVStoreWithBuffer {
	return f.kvb
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

func (kvb *kvStoreWithBuffer) Revert(sid int) error {
	return kvb.buffer.Revert(sid)
}

func (kvb *kvStoreWithBuffer) SerializeQueue() []byte {
	return kvb.buffer.SerializeQueue()
}

func (kvb *kvStoreWithBuffer) Size() int {
	return kvb.buffer.Size()
}

func (kvb *kvStoreWithBuffer) Get(ns string, key []byte) ([]byte, error) {
	value, err := kvb.buffer.Get(ns, key)
	if errors.Cause(err) == ErrNotExist {
		value, err = kvb.store.Get(ns, key)
	}
	if errors.Cause(err) == ErrAlreadyDeleted {
		err = errors.Wrapf(ErrNotExist, "failed to get key %x in %s, deleted in buffer level", key, ns)
	}
	return value, err
}

func (kvb *kvStoreWithBuffer) Put(ns string, key, value []byte) error {
	kvb.buffer.Put(ns, key, value, "faild to put %x in %s", key, ns)
	return nil
}

func (kvb *kvStoreWithBuffer) MustPut(ns string, key, value []byte) {
	kvb.buffer.Put(ns, key, value, "faild to put %x in %s", key, ns)
}

func (kvb *kvStoreWithBuffer) Delete(ns string, key []byte) error {
	kvb.buffer.Delete(ns, key, "failed to delete %x in %s", key, ns)
	return nil
}

func (kvb *kvStoreWithBuffer) MustDelete(ns string, key []byte) {
	kvb.buffer.Delete(ns, key, "failed to delete %x in %s", key, ns)
}

func (kvb *kvStoreWithBuffer) WriteBatch(b KVStoreBatch) (err error) {
	b.Lock()
	defer func() {
		if err == nil {
			// clear the batch if commit succeeds
			b.ClearAndUnlock()
		} else {
			b.Unlock()
		}
	}()
	writes := make([]*WriteInfo, b.Size())
	for i := 0; i < b.Size(); i++ {
		write, e := b.Entry(i)
		if e != nil {
			return e
		}
		if write.writeType != Put && write.writeType != Delete {
			return errors.Errorf("invalid write type %d", write.writeType)
		}
		writes[i] = write
	}
	kvb.buffer.Lock()
	defer kvb.buffer.Unlock()
	for _, write := range writes {
		kvb.buffer.batch(write.writeType, write.namespace, write.key, write.value, write.errorFormat, write.errorArgs)
	}

	return nil
}
