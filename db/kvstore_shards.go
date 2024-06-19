package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/batch"
)

type (
	// KVStoreShards is the interface of sharded KV store
	KVStoreShards interface {
		KVStore
		Shard(bucketPrefix string) KVStore
	}
	kvStoreShards struct {
		KVStore
		once sync.Once
	}
	kvStorePrefix struct {
		kv           KVStore
		bucketPrefix string
	}
)

// NewKVStoreShards creates a new KVStoreSharding
func NewKVStoreShards(kv KVStore) KVStoreShards {
	return &kvStoreShards{
		KVStore: kv,
	}
}

func (k *kvStoreShards) Start(ctx context.Context) error {
	k.once.Do(func() {
		if err := k.KVStore.Start(ctx); err != nil {
			panic(fmt.Sprintf("failed to start underlying kvstore: %v", err))
		}
	})
	return nil
}

func (k *kvStoreShards) Stop(ctx context.Context) error {
	k.once.Do(func() {
		if err := k.KVStore.Stop(ctx); err != nil {
			panic(fmt.Sprintf("failed to stop underlying kvstore: %v", err))
		}
	})
	return nil
}

func (k *kvStoreShards) Shard(bucketPrefix string) KVStore {
	return NewKVStoreWithPrefix(k, bucketPrefix)
}

// NewKVStoreWithPrefix creates a new KVStore with bucket prefix
func NewKVStoreWithPrefix(kv KVStoreShards, bucketPrefix string) KVStore {
	return &kvStorePrefix{
		kv:           kv,
		bucketPrefix: bucketPrefix,
	}
}

func (k *kvStorePrefix) Start(ctx context.Context) error {
	return k.kv.Start(ctx)
}

func (k *kvStorePrefix) Stop(ctx context.Context) error {
	return k.kv.Stop(ctx)
}

func (k *kvStorePrefix) Put(bucket string, key, value []byte) error {
	return k.kv.Put(k.innerBucket(bucket), key, value)
}

func (k *kvStorePrefix) Get(bucket string, key []byte) ([]byte, error) {
	return k.kv.Get(k.innerBucket(bucket), key)
}

func (k *kvStorePrefix) Delete(bucket string, key []byte) error {
	return k.kv.Delete(k.innerBucket(bucket), key)
}

func (k *kvStorePrefix) WriteBatch(kvsb batch.KVStoreBatch) error {
	innerKvsb := batch.NewBatch()
	for i := 0; i < kvsb.Size(); i++ {
		wi, err := kvsb.Entry(i)
		if err != nil {
			return err
		}
		switch wi.WriteType() {
		case batch.Put:
			innerKvsb.Put(k.innerBucket(wi.Namespace()), wi.Key(), wi.Value(), wi.Error())
		case batch.Delete:
			innerKvsb.Delete(k.innerBucket(wi.Namespace()), wi.Key(), wi.Error())
		default:
			return errors.Errorf("unsupported write type %d", wi.WriteType())
		}
	}
	return k.kv.WriteBatch(innerKvsb)
}

func (k *kvStorePrefix) Filter(bucket string, condition Condition, start, limit []byte) ([][]byte, [][]byte, error) {
	return k.kv.Filter(k.innerBucket(bucket), condition, start, limit)
}

func (k *kvStorePrefix) innerBucket(bucket string) string {
	return k.bucketPrefix + bucket
}
