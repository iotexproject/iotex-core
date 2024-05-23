// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/zap"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

var bloomfilterNamespace = ArchiveNamespacePrefix + "-bloomfilter"

type factoryWorkingSetStore struct {
	expire    uint64
	tip       uint64
	bf        bloom.BloomFilter
	view      protocol.View
	flusher   db.KVStoreFlusher
	tlt       trie.TwoLayerTrie
	trieRoots map[int][]byte
}

var _DELETE_ []byte = []byte("delete")
var _PUT_ []byte = []byte("put")

func newFactoryWorkingSetStore(
	ctx context.Context,
	height uint64,
	historyWindowSize uint64,
	tipHeight uint64,
	view protocol.View,
	kvstore db.KVStore,
) (workingSetStore, error) {
	g := genesis.MustExtractGenesisContext(ctx)
	preEaster := !g.IsEaster(height)
	opts := []db.KVStoreFlusherOption{
		db.SerializeFilterOption(func(wi *batch.WriteInfo) bool {
			if wi.Namespace() == ArchiveTrieNamespace {
				return true
			}
			if wi.Namespace() != evm.CodeKVNameSpace && wi.Namespace() != staking.CandsMapNS {
				return false
			}
			return preEaster
		}),
		db.SerializeOption(func(wi *batch.WriteInfo) []byte {
			if preEaster {
				return wi.SerializeWithoutWriteType()
			}
			return wi.Serialize()
		}),
	}
	var expire uint64
	var bf bloom.BloomFilter
	var rootKey string
	switch historyWindowSize {
	case 1:
		if height < tipHeight {
			return nil, errors.Errorf("height %d state does not exist", height)
		}
		rootKey = ArchiveTrieRootKey
	case 0:
		opts = append(opts, db.FlushTranslateOption(func(wi *batch.WriteInfo) []*batch.WriteInfo {
			if wi.WriteType() == batch.Delete && wi.Namespace() == ArchiveTrieNamespace {
				return nil
			}
			return []*batch.WriteInfo{wi}
		}))
		if height >= tipHeight {
			rootKey = ArchiveTrieRootKey
		} else {
			rootKey = fmt.Sprintf("%s-%d", ArchiveTrieRootKey, height)
		}
	default:
		if height <= tipHeight-historyWindowSize {
			return nil, errors.Errorf("height %d state does not exist", height)
		} else if height < tipHeight {
			rootKey = fmt.Sprintf("%s-%d", ArchiveTrieRootKey, height)
		} else {
			rootKey = ArchiveTrieRootKey
		}
		var err error
		bf, err = bloom.NewBloomFilter(2048, 4)
		if err != nil {
			log.L().Panic("failed to create bloom filter", zap.Error(err))
		}
		namespace := fmt.Sprintf("%s-%d", ArchiveNamespacePrefix, height)
		opts = append(opts, db.FlushTranslateOption(func(wi *batch.WriteInfo) []*batch.WriteInfo {
			if wi.Namespace() != ArchiveTrieNamespace {
				return []*batch.WriteInfo{wi}
			}
			switch wi.WriteType() {
			case batch.Delete:
				return []*batch.WriteInfo{
					batch.NewWriteInfo(
						batch.Put,
						namespace,
						wi.Key(),
						_DELETE_,
						"failed to record deleted keys in archive mode",
					),
				}
			case batch.Put:
				bf.Add(wi.Key())
				return []*batch.WriteInfo{
					wi,
					batch.NewWriteInfo(
						batch.Put,
						namespace,
						wi.Key(),
						_PUT_,
						"failed to record new keys in archive mode",
					),
				}
			default:
				panic("invalid write type")
			}
		}))
		if height > historyWindowSize {
			expire = height - historyWindowSize
		}
	}

	flusher, err := db.NewKVStoreFlusher(
		kvstore,
		batch.NewCachedBatch(),
		opts...,
	)
	if err != nil {
		return nil, err
	}
	tlt, err := newTwoLayerTrie(ArchiveTrieNamespace, flusher.KVStoreWithBuffer(), rootKey, true)
	if err != nil {
		return nil, err
	}

	return &factoryWorkingSetStore{
		tip:       height,
		expire:    expire,
		bf:        bf,
		flusher:   flusher,
		view:      view,
		tlt:       tlt,
		trieRoots: make(map[int][]byte),
	}, nil
}

func (store *factoryWorkingSetStore) trimExpiredStates() error {
	if store.expire == 0 {
		return nil
	}
	filters := []bloom.BloomFilter{}
	// TODO: preload bloomfilters in factory, and update the bloomfilters in memory on commit
	for i := store.expire + 1; i <= store.tip; i++ {
		v, err := store.flusher.KVStoreWithBuffer().Get(bloomfilterNamespace, new(big.Int).SetUint64(i).Bytes())
		if err != nil {
			return err
		}
		filter, err := bloom.NewBloomFilter(2048, 4)
		if err != nil {
			log.L().Panic("failed to create bloom filter", zap.Error(err))
		}
		// TODO: refactor bf's interface, `FromBytes`` shouldn't be an function of the interface but a way to create such an instance
		if err := filter.FromBytes(v); err != nil {
			return err
		}
		filters = append(filters, filter)
	}
	expiredNamespace := fmt.Sprintf("%s-%d", ArchiveNamespacePrefix, store.expire)
	keys := map[string]struct{}{}
	// TODO: as the following example show, `Filter`` is not a function commonly needed in KVStore, but `Range`
	_, _, err := store.flusher.KVStoreWithBuffer().Filter(
		expiredNamespace,
		func(k, v []byte) bool {
			if bytes.Equal(v, _DELETE_) {
				keys[hex.EncodeToString(k)] = struct{}{}
			} else if bytes.Equal(v, _PUT_) {
				delete(keys, hex.EncodeToString(k))
			}
			return false
		},
		nil,
		nil,
	)
	b := batch.NewBatch()
	switch errors.Cause(err) {
	case db.ErrNotExist, db.ErrBucketNotExist: // do nothing
	case nil:
		for key := range keys {
			bk, err := hex.DecodeString(key)
			if err != nil {
				log.L().Panic("failed to decode key", zap.Error(err))
			}
			var exists bool
			for _, filter := range filters {
				if filter.Exist(bk) {
					exists = true
				}
			}
			if !exists {
				b.Delete(ArchiveTrieNamespace, bk, "failed to delete key")
			}
		}
		b.Delete(expiredNamespace, nil, "failed to delete archive")
	default:
		return err
	}
	b.Delete(bloomfilterNamespace, new(big.Int).SetUint64(store.expire).Bytes(), "failed to delete bloom filter")

	return store.flusher.KVStoreWithBuffer().WriteBatch(b)
}

func (store *factoryWorkingSetStore) Start(ctx context.Context) error {
	return store.tlt.Start(ctx)
}

func (store *factoryWorkingSetStore) Stop(ctx context.Context) error {
	return store.tlt.Stop(ctx)
}

func (store *factoryWorkingSetStore) ReadView(name string) (interface{}, error) {
	return store.view.Read(name)
}

func (store *factoryWorkingSetStore) WriteView(name string, value interface{}) error {
	return store.view.Write(name, value)
}

func (store *factoryWorkingSetStore) Get(ns string, key []byte) ([]byte, error) {
	return readStateFromTLT(store.tlt, ns, key)
}

func (store *factoryWorkingSetStore) Put(ns string, key []byte, value []byte) error {
	store.flusher.KVStoreWithBuffer().MustPut(ns, key, value)
	nsHash := hash.Hash160b([]byte(ns))

	return store.tlt.Upsert(nsHash[:], toLegacyKey(key), value)
}

func (store *factoryWorkingSetStore) Delete(ns string, key []byte) error {
	store.flusher.KVStoreWithBuffer().MustDelete(ns, key)
	nsHash := hash.Hash160b([]byte(ns))

	err := store.tlt.Delete(nsHash[:], toLegacyKey(key))
	if errors.Cause(err) == trie.ErrNotExist {
		return errors.Wrapf(state.ErrStateNotExist, "key %x doesn't exist in namespace %x", key, nsHash)
	}
	return err
}

func (store *factoryWorkingSetStore) States(ns string, keys [][]byte) ([][]byte, [][]byte, error) {
	return readStatesFromTLT(store.tlt, ns, keys)
}

func (store *factoryWorkingSetStore) Digest() hash.Hash256 {
	return hash.Hash256b(store.flusher.SerializeQueue())
}

func (store *factoryWorkingSetStore) Finalize(h uint64) error {
	rootHash, err := store.tlt.RootHash()
	if err != nil {
		return err
	}
	store.flusher.KVStoreWithBuffer().MustPut(AccountKVNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(h))
	store.flusher.KVStoreWithBuffer().MustPut(ArchiveTrieNamespace, []byte(ArchiveTrieRootKey), rootHash)
	// Persist the historical accountTrie's root hash
	store.flusher.KVStoreWithBuffer().MustPut(
		ArchiveTrieNamespace,
		[]byte(fmt.Sprintf("%s-%d", ArchiveTrieRootKey, h)),
		rootHash,
	)
	return nil
}

func (store *factoryWorkingSetStore) Commit() error {
	_dbBatchSizelMtc.WithLabelValues().Set(float64(store.flusher.KVStoreWithBuffer().Size()))
	if err := store.trimExpiredStates(); err != nil {
		return err
	}
	return store.flusher.Flush()
}

func (store *factoryWorkingSetStore) Snapshot() int {
	rh, err := store.tlt.RootHash()
	if err != nil {
		log.L().Panic("failed to do snapshot", zap.Error(err))
	}
	s := store.flusher.KVStoreWithBuffer().Snapshot()
	store.trieRoots[s] = rh
	return s
}

func (store *factoryWorkingSetStore) RevertSnapshot(snapshot int) error {
	if err := store.flusher.KVStoreWithBuffer().RevertSnapshot(snapshot); err != nil {
		return err
	}
	root, ok := store.trieRoots[snapshot]
	if !ok {
		// this should not happen, b/c we save the trie root on a successful return of Snapshot(), but check anyway
		return errors.Wrapf(trie.ErrInvalidTrie, "failed to get trie root for snapshot = %d", snapshot)
	}
	return store.tlt.SetRootHash(root[:])
}

func (store *factoryWorkingSetStore) ResetSnapshots() {
	store.flusher.KVStoreWithBuffer().ResetSnapshots()
	store.trieRoots = make(map[int][]byte)
}
