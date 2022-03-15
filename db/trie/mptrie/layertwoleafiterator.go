// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/trie"
)

// NewLayerTwoLeafIterator returns a new leaf iterator
func NewLayerTwoLeafIterator(tr trie.TwoLayerTrie, layerOneKey []byte, l int) (trie.Iterator, error) {
	tlt, ok := tr.(*twoLayerTrie)
	if !ok {
		return nil, errors.New("trie is not supported type")
	}
	lt, err := tlt.layerTwoTrie(layerOneKey, l)
	if err != nil {
		return nil, err
	}
	return NewLeafIterator(lt.tr)
}
