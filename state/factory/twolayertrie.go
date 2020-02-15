// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/trie"
)

// TwoLayerTrie is a trie data structure with two layers
type TwoLayerTrie struct {
	layerOne trie.Trie
}

// Start starts the layer one trie
func (tlt *TwoLayerTrie) Start(ctx context.Context) error {
	return tlt.layerOne.Start(ctx)
}

// Stop stops the layer one trie
func (tlt *TwoLayerTrie) Stop(ctx context.Context) error {
	return tlt.layerOne.Stop(ctx)
}

// RootHash returns the layer one trie root
func (tlt *TwoLayerTrie) RootHash() []byte {
	return tlt.layerOne.RootHash()
}

func (tlt *TwoLayerTrie) layerTwoTrie(key []byte, layerTwoTrieKeyLen int) (trie.Trie, error) {
	value, err := tlt.layerOne.Get(key)
	switch errors.Cause(err) {
	case trie.ErrNotExist:
		return trie.NewTrie(trie.KVStoreOption(tlt.layerOne.DB()), trie.KeyLengthOption(layerTwoTrieKeyLen))
	case nil:
		return trie.NewTrie(trie.KVStoreOption(tlt.layerOne.DB()), trie.RootHashOption(value), trie.KeyLengthOption(layerTwoTrieKeyLen))
	default:
		return nil, err
	}
}

// Get returns the layer two value
func (tlt *TwoLayerTrie) Get(layerOneKey []byte, layerTwoKey []byte) ([]byte, error) {
	layerTwo, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return nil, err
	}
	if err := layerTwo.Start(context.Background()); err != nil {
		return nil, err
	}
	defer layerTwo.Stop(context.Background())

	return layerTwo.Get(layerTwoKey)
}

// Upsert upserts an item
func (tlt *TwoLayerTrie) Upsert(layerOneKey []byte, layerTwoKey []byte, value []byte) error {
	layerTwo, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return err
	}
	if err := layerTwo.Start(context.Background()); err != nil {
		return err
	}
	defer layerTwo.Stop(context.Background())

	if err := layerTwo.Upsert(layerTwoKey, value); err != nil {
		return err
	}

	return tlt.layerOne.Upsert(layerOneKey, layerTwo.RootHash())
}

// Delete deletes an item
func (tlt *TwoLayerTrie) Delete(layerOneKey []byte, layerTwoKey []byte) error {
	layerTwo, err := tlt.layerTwoTrie(layerOneKey, len(layerTwoKey))
	if err != nil {
		return err
	}
	if err := layerTwo.Start(context.Background()); err != nil {
		return err
	}
	defer layerTwo.Stop(context.Background())

	if err := layerTwo.Delete(layerTwoKey); err != nil {
		return err
	}

	if layerTwo.IsEmpty() {
		return tlt.layerOne.Delete(layerOneKey)
	}

	return tlt.layerOne.Upsert(layerOneKey, layerTwo.RootHash())
}
