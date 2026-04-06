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
)

// witnessTrie wraps a trie.Trie to track first-touch storage keys and their
// prestate values during EVM execution. At transaction end, it builds the
// contract storage witness by generating proofs from a prestate trie snapshot.
type witnessTrie struct {
	inner    trie.Trie
	prestate trie.Trie // cloned at first mutation, used for proof generation

	// firstTouch records the prestate value for each key at first access.
	// nil value means the key was absent in prestate.
	firstTouch map[hash.Hash256][]byte
	// hasFirstTouch tracks whether a key has been recorded (distinguishing nil-value from not-yet-seen).
	hasFirstTouch map[hash.Hash256]bool

	// hashFunc is the contract-scoped hash function (includes address prefix).
	hashFunc func([]byte) []byte
}

func newWitnessTrie(inner trie.Trie, hashFunc func([]byte) []byte) *witnessTrie {
	return &witnessTrie{
		inner:         inner,
		firstTouch:    make(map[hash.Hash256][]byte),
		hasFirstTouch: make(map[hash.Hash256]bool),
		hashFunc:      hashFunc,
	}
}

// capturePrestate lazily clones the inner trie before the first mutation.
func (w *witnessTrie) capturePrestate() error {
	if w.prestate != nil {
		return nil
	}
	clone, err := w.inner.Clone(trie.NewMemKVStore())
	if err != nil {
		return errors.Wrap(err, "failed to clone prestate trie")
	}
	w.prestate = clone
	return nil
}

// recordFirstTouch records the prestate value on first access of a key.
func (w *witnessTrie) recordFirstTouch(key hash.Hash256) {
	if w.hasFirstTouch[key] {
		return
	}
	w.hasFirstTouch[key] = true
	v, err := w.inner.Get(key[:])
	if err != nil {
		// key doesn't exist in prestate — record nil
		w.firstTouch[key] = nil
	} else {
		cp := make([]byte, len(v))
		copy(cp, v)
		w.firstTouch[key] = cp
	}
}

// Get retrieves a value from the trie and records first-touch.
func (w *witnessTrie) Get(key []byte) ([]byte, error) {
	k := hash.BytesToHash256(key)
	w.recordFirstTouch(k)
	return w.inner.Get(key)
}

// Upsert inserts/updates a value and records first-touch + prestate snapshot.
func (w *witnessTrie) Upsert(key []byte, value []byte) error {
	k := hash.BytesToHash256(key)
	w.recordFirstTouch(k)
	if err := w.capturePrestate(); err != nil {
		return err
	}
	return w.inner.Upsert(key, value)
}

// Delete removes a key and records first-touch + prestate snapshot.
func (w *witnessTrie) Delete(key []byte) error {
	k := hash.BytesToHash256(key)
	w.recordFirstTouch(k)
	if err := w.capturePrestate(); err != nil {
		return err
	}
	return w.inner.Delete(key)
}

// BuildWitness generates the ContractStorageWitness from recorded first-touch
// keys using proofs from the prestate trie. Must be called after execution completes.
func (w *witnessTrie) BuildWitness() (*ContractStorageWitness, error) {
	if len(w.firstTouch) == 0 && len(w.hasFirstTouch) == 0 {
		return nil, nil
	}
	// Determine the proof source: prestate clone if mutations happened, otherwise the current trie.
	proofSource := w.prestate
	if proofSource == nil {
		// No mutations occurred — trie is still in prestate, use it directly.
		proofSource = w.inner
	}

	// An empty trie's RootHash() returns emptyRootHash (hash of empty branch
	// protobuf), but Account.Root uses ZeroHash256 to mean "no storage".
	// Normalize to ZeroHash256 so the witness matches the account representation
	// and the validator's fast path in VerifyContractStorageWitness.
	if proofSource.IsEmpty() {
		entries := make([]ContractStorageWitnessEntry, 0, len(w.hasFirstTouch))
		for key := range w.hasFirstTouch {
			entries = append(entries, ContractStorageWitnessEntry{
				Key:   key,
				Value: nil, // all absent in empty trie
			})
		}
		return &ContractStorageWitness{
			StorageRoot: hash.ZeroHash256,
			Entries:     entries,
		}, nil
	}

	rootHash, err := proofSource.RootHash()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get prestate root hash")
	}
	storageRoot := hash.BytesToHash256(rootHash)

	// Collect entries and deduplicated proof nodes.
	entries := make([]ContractStorageWitnessEntry, 0, len(w.hasFirstTouch))
	proofNodeSet := make(map[string][]byte)

	pt, isProofTrie := proofSource.(trie.ProofTrie)
	for key := range w.hasFirstTouch {
		entries = append(entries, ContractStorageWitnessEntry{
			Key:   key,
			Value: w.firstTouch[key], // nil for absent keys
		})
		if !isProofTrie {
			continue
		}
		proofNodes, err := pt.GetProof(key[:])
		if err != nil && errors.Cause(err) != trie.ErrNotExist {
			return nil, errors.Wrapf(err, "failed to get proof for key %x", key[:])
		}
		for _, node := range proofNodes {
			h := w.hashFunc(node)
			proofNodeSet[string(h)] = node
		}
	}

	// Flatten deduplicated proof nodes.
	dedupedNodes := make([][]byte, 0, len(proofNodeSet))
	for _, node := range proofNodeSet {
		dedupedNodes = append(dedupedNodes, node)
	}

	return &ContractStorageWitness{
		StorageRoot: storageRoot,
		Entries:     entries,
		ProofNodes:  dedupedNodes,
	}, nil
}

// --- Delegate remaining trie.Trie methods to inner ---

func (w *witnessTrie) Start(ctx context.Context) error     { return w.inner.Start(ctx) }
func (w *witnessTrie) Stop(ctx context.Context) error      { return w.inner.Stop(ctx) }
func (w *witnessTrie) RootHash() ([]byte, error)           { return w.inner.RootHash() }
func (w *witnessTrie) SetRootHash(hash []byte) error       { return w.inner.SetRootHash(hash) }
func (w *witnessTrie) IsEmpty() bool                       { return w.inner.IsEmpty() }
func (w *witnessTrie) Clone(kv trie.KVStore) (trie.Trie, error) { return w.inner.Clone(kv) }
