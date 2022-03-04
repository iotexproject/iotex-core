// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"bytes"
	"context"
	"encoding/hex"
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/trie"
)

type (
	layerTwo struct {
		tr         trie.Trie
		dirty      bool
		originHash []byte
	}
	twoLayerTrie struct {
		layerOne    trie.Trie
		layerTwoMap map[string]*layerTwo
		kvStore     trie.KVStore
		rootKey     string
	}
)

// NewTwoLayerTrie creates a two layer trie
func NewTwoLayerTrie(dbForTrie trie.KVStore, rootKey string) trie.TwoLayerTrie {
	return &twoLayerTrie{
		kvStore: dbForTrie,
		rootKey: rootKey,
	}
}

func (tlt *twoLayerTrie) layerTwoTrie(key []byte, layerTwoTrieKeyLen int) (*layerTwo, error) {
	hk := hex.EncodeToString(key)
	if lt, ok := tlt.layerTwoMap[hk]; ok {
		return lt, nil
	}
	opts := []Option{KVStoreOption(tlt.kvStore), KeyLengthOption(layerTwoTrieKeyLen)}
	value, err := tlt.layerOne.Get(key)
	switch errors.Cause(err) {
	case trie.ErrNotExist:
		// start an empty trie
	case nil:
		opts = append(opts, RootHashOption(value))
	default:
		return nil, err
	}

	lt, err := New(opts...)
	if err != nil {
		return nil, err
	}
	if err := lt.Start(context.Background()); err != nil {
		return nil, err
	}
	h, err := lt.RootHash()
	if err != nil {
		return nil, err
	}

	tlt.layerTwoMap[hk] = &layerTwo{
		tr:         lt,
		dirty:      false,
		originHash: h,
	}

	return tlt.layerTwoMap[hk], nil
}

func (tlt *twoLayerTrie) Start(ctx context.Context) error {
	rootHash, err := tlt.kvStore.Get([]byte(tlt.rootKey))
	if errors.Cause(err) == trie.ErrNotExist {
		rootHash = nil
	}
	layerOne, err := New(
		KVStoreOption(tlt.kvStore),
		RootHashOption(rootHash),
	)
	if err != nil {
		return errors.Wrapf(err, "failed to generate trie for %s", tlt.rootKey)
	}
	tlt.layerOne = layerOne
	tlt.layerTwoMap = make(map[string]*layerTwo)

	return tlt.layerOne.Start(ctx)
}

func (tlt *twoLayerTrie) Stop(ctx context.Context) error {
	if err := tlt.flush(ctx); err != nil {
		return err
	}

	return tlt.layerOne.Stop(ctx)
}

func (tlt *twoLayerTrie) stop(ctx context.Context, hkey string, lt *layerTwo) (err error) {
	key, err := hex.DecodeString(hkey)
	if err != nil {
		return err
	}
	if lt.dirty {
		rh, err := lt.tr.RootHash()
		if err != nil {
			return err
		}
		if !bytes.Equal(rh, lt.originHash) {
			if lt.tr.IsEmpty() {
				return tlt.layerOne.Delete(key)
			}

			return tlt.layerOne.Upsert(key, rh)
		}
	}

	return lt.tr.Stop(ctx)
}

func (tlt *twoLayerTrie) layerTwoKeys() []string {
	keys := make([]string, 0)
	for k := range tlt.layerTwoMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func (tlt *twoLayerTrie) flush(ctx context.Context) error {
	keys := tlt.layerTwoKeys()
	for _, hkey := range keys {
		if err := tlt.stop(ctx, hkey, tlt.layerTwoMap[hkey]); err != nil {
			return err
		}
	}
	tlt.layerTwoMap = make(map[string]*layerTwo)
	_, err := tlt.layerOne.RootHash()
	return err
}

func (tlt *twoLayerTrie) RootHash() ([]byte, error) {
	if err := tlt.flush(context.Background()); err != nil {
		return nil, err
	}
	return tlt.layerOne.RootHash()
}

func (tlt *twoLayerTrie) SetRootHash(rh []byte) error {
	if err := tlt.layerOne.SetRootHash(rh); err != nil {
		return err
	}
	keys := tlt.layerTwoKeys()
	for _, k := range keys {
		if err := tlt.layerTwoMap[k].tr.Stop(context.Background()); err != nil {
			return err
		}
	}
	tlt.layerTwoMap = make(map[string]*layerTwo)
	return nil
}

func (tlt *twoLayerTrie) Get(layerOneKey []byte, layerTwoKey []byte) ([]byte, error) {
	lt, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return nil, err
	}

	return lt.tr.Get(layerTwoKey)
}

func (tlt *twoLayerTrie) Upsert(layerOneKey []byte, layerTwoKey []byte, value []byte) error {
	lt, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return err
	}
	if err := lt.tr.Upsert(layerTwoKey, value); err != nil {
		return err
	}
	lt.dirty = true

	return nil
}

func (tlt *twoLayerTrie) Delete(layerOneKey []byte, layerTwoKey []byte) error {
	lt, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return err
	}
	if err := lt.tr.Delete(layerTwoKey); err != nil {
		return err
	}
	lt.dirty = true

	return nil
}
