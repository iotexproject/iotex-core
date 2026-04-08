// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db/trie"
)

var errNoStorageWitness = errors.New("contract storage accessed without witness data")

// noStorageTrie is a trie implementation that rejects all storage operations.
// It is used for contracts that have no witness data in stateless validation:
// the full node never accessed this contract's storage, so any storage access
// on the stateless node indicates a divergence that should fail immediately.
// RootHash/SetRootHash/IsEmpty are allowed because they are used by the
// contract infrastructure (Snapshot, LoadRoot) for non-storage bookkeeping.
type noStorageTrie struct {
	root []byte
}

func (t *noStorageTrie) Start(context.Context) error { return nil }
func (t *noStorageTrie) Stop(context.Context) error  { return nil }
func (t *noStorageTrie) Upsert([]byte, []byte) error { return errNoStorageWitness }
func (t *noStorageTrie) Get([]byte) ([]byte, error)  { return nil, errNoStorageWitness }
func (t *noStorageTrie) Delete([]byte) error          { return errNoStorageWitness }
func (t *noStorageTrie) RootHash() ([]byte, error)    { return t.root, nil }
func (t *noStorageTrie) SetRootHash(root []byte) error {
	t.root = make([]byte, len(root))
	copy(t.root, root)
	return nil
}
func (t *noStorageTrie) IsEmpty() bool { return true }
func (t *noStorageTrie) Clone(trie.KVStore) (trie.Trie, error) {
	return nil, errNoStorageWitness
}
