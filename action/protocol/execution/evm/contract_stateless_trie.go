// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
)

// newStatelessTrie constructs a real MPT backed by the witness proof nodes in a
// MemKVStore. The returned trie supports Get/Upsert/Delete and — critically —
// RootHash() returns the correct post-state root after mutations, which allows
// contract.Commit() to persist the right Account.Root.
//
// For an empty-storage contract (StorageRoot == ZeroHash256 with no proof nodes),
// an empty MPT is created.
func newStatelessTrie(addr hash.Hash160, witness *ContractStorageWitness) (trie.Trie, error) {
	hashFunc := func(data []byte) []byte {
		h := hash.Hash256b(append(addr[:], data...))
		return h[:]
	}

	kvStore := trie.NewMemKVStore()
	for _, node := range witness.ProofNodes {
		h := hashFunc(node)
		if err := kvStore.Put(h, node); err != nil {
			return nil, errors.Wrap(err, "failed to populate proof node store")
		}
	}

	options := []mptrie.Option{
		mptrie.KVStoreOption(kvStore),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(hashFunc),
	}
	if witness.StorageRoot != hash.ZeroHash256 {
		options = append(options, mptrie.RootHashOption(witness.StorageRoot[:]))
	}

	tr, err := mptrie.New(options...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stateless MPT")
	}
	if err := tr.Start(context.Background()); err != nil {
		return nil, errors.Wrap(err, "failed to start stateless MPT")
	}
	return tr, nil
}
