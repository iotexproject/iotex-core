// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/v2/state"
)

var errNoStorageWitness = errors.New("contract storage accessed without witness data")

// statelessTrieKVStore is a composite KV store for the stateless trie.
// Reads come from the in-memory store (proof nodes from witness + newly written nodes).
// Writes go to BOTH the in-memory store (so the trie can read them back
// immediately) AND the state manager batch (for digest computation + DB flush).
type statelessTrieKVStore struct {
	mem            trie.KVStore          // in-memory store for proof nodes + new nodes
	sm             protocol.StateManager // state manager for digest computation
	addr           hash.Hash160          // contract address for error reporting
	proofNodeCount int                   // initial count of proof nodes loaded
	putCount       int                   // number of Put calls (new nodes from mutations)
}

func (s *statelessTrieKVStore) Start(ctx context.Context) error { return nil }
func (s *statelessTrieKVStore) Stop(ctx context.Context) error  { return nil }

func (s *statelessTrieKVStore) Get(key []byte) ([]byte, error) {
	v, err := s.mem.Get(key)
	if err != nil {
		panic(fmt.Sprintf("%v: contract %x, missing proof node %s (initial proof nodes: %d, puts: %d)",
			errNoStorageWitness, s.addr, hex.EncodeToString(key),
			s.proofNodeCount, s.putCount))
	}
	return v, nil
}

func (s *statelessTrieKVStore) Put(key []byte, value []byte) error {
	s.putCount++
	if err := s.mem.Put(key, value); err != nil {
		return err
	}
	// mirror the write to the state manager batch for digest computation
	var sb protocol.SerializableBytes = make([]byte, len(value))
	copy(sb, value)
	_, err := s.sm.PutState(sb, protocol.KeyOption(key), protocol.NamespaceOption(ContractKVNameSpace))
	return err
}

func (s *statelessTrieKVStore) Delete(key []byte) error {
	// Do NOT delete from mem: the MPT calls deleteNode() when replacing trie
	// nodes during Upsert/Delete operations. On the full node this is fine
	// because sm.Revert() (called during RevertToSnapshot) rolls back those
	// deletions in the state manager's KV store. But the MemKVStore is not
	// part of the state manager's snapshot/revert mechanism, so deleting here
	// would permanently lose proof nodes and flushed intermediate nodes.
	// Keeping them in mem is harmless (orphaned hashes are never looked up
	// during forward execution) and essential for SetRootHash during reverts.

	// Still mirror the delete to the state manager batch for digest computation
	_, err := s.sm.DelState(protocol.KeyOption(key), protocol.NamespaceOption(ContractKVNameSpace))
	if errors.Cause(err) == state.ErrStateNotExist {
		return nil
	}
	return err
}

// newStatelessTrie constructs a real MPT backed by the witness proof nodes in a
// MemKVStore. The returned trie supports Get/Upsert/Delete and — critically —
// RootHash() returns the correct post-state root after mutations, which allows
// contract.Commit() to persist the right Account.Root.
//
// When sm is non-nil, trie writes are also mirrored to the state manager batch
// under ContractKVNameSpace so that the delta-state digest matches the producer's.
//
// For an empty-storage contract (StorageRoot == ZeroHash256 with no proof nodes),
// an empty MPT is created.
func newStatelessTrie(addr hash.Hash160, witness *ContractStorageWitness, sm protocol.StateManager, enableAsync bool) (trie.Trie, error) {
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

	// Use a composite KV store that mirrors writes to the state manager batch
	// for delta-state digest computation.
	var trieKV trie.KVStore = kvStore
	if sm != nil {
		trieKV = &statelessTrieKVStore{mem: kvStore, sm: sm, addr: addr, proofNodeCount: len(witness.ProofNodes)}
	}

	options := []mptrie.Option{
		mptrie.KVStoreOption(trieKV),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(hashFunc),
	}
	if witness.StorageRoot != hash.ZeroHash256 {
		options = append(options, mptrie.RootHashOption(witness.StorageRoot[:]))
	}
	if enableAsync {
		options = append(options, mptrie.AsyncOption())
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
