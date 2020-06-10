// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"context"
	"encoding/hex"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/trie"
)

type (
	twoLayerTrie struct {
		layerOne trie.Trie
		layerTwo map[string]trie.Trie
		kvStore  trie.KVStore
		rootKey  string
	}
)

// NewTwoLayerTrie creates a two layer trie
func NewTwoLayerTrie(dbForTrie trie.KVStore, rootKey string) trie.TwoLayerTrie {
	return &twoLayerTrie{
		kvStore: dbForTrie,
		rootKey: rootKey,
	}
}

func (tlt *twoLayerTrie) layerTwoTrie(key []byte, layerTwoTrieKeyLen int) (trie.Trie, error) {
	hk := hex.EncodeToString(key)
	if lt, ok := tlt.layerTwo[hk]; ok {
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
	return lt, lt.Start(context.Background())
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
	tlt.layerTwo = make(map[string]trie.Trie)

	return tlt.layerOne.Start(ctx)
}

func (tlt *twoLayerTrie) Stop(ctx context.Context) error {
	for _, lt := range tlt.layerTwo {
		if err := lt.Stop(ctx); err != nil {
			return err
		}
	}
	return tlt.layerOne.Stop(ctx)
}

func (tlt *twoLayerTrie) IsEmpty() bool {
	return tlt.layerOne.IsEmpty()
}

func (tlt *twoLayerTrie) RootHash() []byte {
	return tlt.layerOne.RootHash()
}

func (tlt *twoLayerTrie) SetRootHash(rh []byte) error {
	for key, lt := range tlt.layerTwo {
		if err := lt.Stop(context.Background()); err != nil {
			return err
		}
		delete(tlt.layerTwo, key)
	}
	if err := tlt.layerOne.SetRootHash(rh); err != nil {
		return err
	}

	return nil
}

func (tlt *twoLayerTrie) Get(layerOneKey []byte, layerTwoKey []byte) ([]byte, error) {
	layerTwo, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return nil, err
	}

	return layerTwo.Get(layerTwoKey)
}

func (tlt *twoLayerTrie) Upsert(layerOneKey []byte, layerTwoKey []byte, value []byte) error {
	layerTwo, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return err
	}
	if err := layerTwo.Upsert(layerTwoKey, value); err != nil {
		return err
	}

	return tlt.layerOne.Upsert(layerOneKey, layerTwo.RootHash())
}

func (tlt *twoLayerTrie) Delete(layerOneKey []byte, layerTwoKey []byte) error {
	layerTwo, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return err
	}
	if err := layerTwo.Delete(layerTwoKey); err != nil {
		return err
	}

	if !layerTwo.IsEmpty() {
		return tlt.layerOne.Upsert(layerOneKey, layerTwo.RootHash())
	}

	return tlt.layerOne.Delete(layerOneKey)
}
