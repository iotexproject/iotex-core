// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"
	"encoding/hex"

	"github.com/pkg/errors"
)

// TwoLayerTrie is a trie data structure with two layers
type TwoLayerTrie struct {
	layerOne Trie
	layerTwo map[string]Trie
	kvStore  KVStore
	rootKey  string
}

// NewTwoLayerTrie creates a two layer trie
func NewTwoLayerTrie(dbForTrie KVStore, rootKey string) *TwoLayerTrie {
	return &TwoLayerTrie{
		kvStore: dbForTrie,
		rootKey: rootKey,
	}
}

func (tlt *TwoLayerTrie) layerTwoTrie(key []byte, layerTwoTrieKeyLen int) (Trie, error) {
	hk := hex.EncodeToString(key)
	if lt, ok := tlt.layerTwo[hk]; ok {
		return lt, nil
	}
	opts := []Option{KVStoreOption(tlt.kvStore), KeyLengthOption(layerTwoTrieKeyLen)}
	value, err := tlt.layerOne.Get(key)
	switch errors.Cause(err) {
	case ErrNotExist:
		// start an empty trie
	case nil:
		opts = append(opts, RootHashOption(value))
	default:
		return nil, err
	}

	lt, err := NewTrie(opts...)
	if err != nil {
		return nil, err
	}
	return lt, lt.Start(context.Background())
}

// Start starts the layer one trie
func (tlt *TwoLayerTrie) Start(ctx context.Context) error {
	layerOne, err := NewTrie(
		KVStoreOption(tlt.kvStore),
		RootKeyOption(tlt.rootKey),
	)
	if err != nil {
		return errors.Wrapf(err, "failed to generate trie for %s", tlt.rootKey)
	}
	tlt.layerOne = layerOne
	tlt.layerTwo = make(map[string]Trie)

	return tlt.layerOne.Start(ctx)
}

// Stop stops the layer one trie
func (tlt *TwoLayerTrie) Stop(ctx context.Context) error {
	for _, lt := range tlt.layerTwo {
		if err := lt.Stop(ctx); err != nil {
			return err
		}
	}
	return tlt.layerOne.Stop(ctx)
}

// RootHash returns the layer one trie root
func (tlt *TwoLayerTrie) RootHash() []byte {
	return tlt.layerOne.RootHash()
}

// SetRootHash sets root hash for layer one trie
func (tlt *TwoLayerTrie) SetRootHash(rh []byte) error {
	return tlt.layerOne.SetRootHash(rh)
}

// Get returns the value in layer two
func (tlt *TwoLayerTrie) Get(layerOneKey []byte, layerTwoKey []byte) ([]byte, error) {
	layerTwo, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return nil, err
	}

	return layerTwo.Get(layerTwoKey)
}

// Upsert upserts an item in layer two
func (tlt *TwoLayerTrie) Upsert(layerOneKey []byte, layerTwoKey []byte, value []byte) error {
	layerTwo, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return err
	}
	if err := layerTwo.Upsert(layerTwoKey, value); err != nil {
		return err
	}

	return tlt.layerOne.Upsert(layerOneKey, layerTwo.RootHash())
}

// Delete deletes an item in layer two
func (tlt *TwoLayerTrie) Delete(layerOneKey []byte, layerTwoKey []byte) error {
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
