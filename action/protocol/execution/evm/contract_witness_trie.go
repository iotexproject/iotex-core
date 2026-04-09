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

// saveOnDeleteKVStore wraps a trie.KVStore to intercept Delete calls, saving
// old node values into a secondary MemKVStore before they are tombstoned in
// the primary store. This allows a prestate trie clone to resolve deleted
// hashNodes without needing a full CollectNodes snapshot.
//
// It also records every node loaded during execution (via Get) so that
// BuildWitness can include nodes that GetProof alone would miss — notably
// sibling nodes loaded during branch collapse in Delete operations.
type saveOnDeleteKVStore struct {
	inner  trie.KVStore        // SM-backed production store
	saved  trie.KVStore        // MemKVStore holding deleted nodes
	loaded map[string][]byte   // hash→serialized data of all nodes loaded from inner/saved
}

func newSaveOnDeleteKVStore(inner trie.KVStore) *saveOnDeleteKVStore {
	return &saveOnDeleteKVStore{
		inner:  inner,
		saved:  trie.NewMemKVStore(),
		loaded: make(map[string][]byte),
	}
}

func (s *saveOnDeleteKVStore) Start(ctx context.Context) error { return s.inner.Start(ctx) }
func (s *saveOnDeleteKVStore) Stop(ctx context.Context) error  { return s.inner.Stop(ctx) }

func (s *saveOnDeleteKVStore) Get(key []byte) ([]byte, error) {
	v, err := s.inner.Get(key)
	if err == nil {
		s.loaded[string(key)] = append([]byte(nil), v...)
		return v, nil
	}
	// Fallback to saved (deleted) nodes.
	v, err = s.saved.Get(key)
	if err == nil {
		s.loaded[string(key)] = append([]byte(nil), v...)
	}
	return v, err
}

func (s *saveOnDeleteKVStore) Put(key []byte, value []byte) error {
	return s.inner.Put(key, value)
}

func (s *saveOnDeleteKVStore) Delete(key []byte) error {
	// Save the old value before it is tombstoned.
	if v, err := s.inner.Get(key); err == nil {
		_ = s.saved.Put(key, v)
	}
	return s.inner.Delete(key)
}

// witnessTrie wraps a trie.Trie to track first-touch storage keys and their
// prestate values during EVM execution. At transaction end, it builds the
// contract storage witness by generating proofs from a prestate trie snapshot.
type witnessTrie struct {
	inner    trie.Trie
	prestate trie.Trie // cloned at first mutation, used for proof generation

	// kvStore is a saveOnDeleteKVStore that preserves deleted trie nodes.
	// nil when inner trie is MemKVStore-backed (tests).
	kvStore *saveOnDeleteKVStore

	// firstTouch records the prestate value for each key at first access.
	// nil value means the key was absent in prestate.
	firstTouch map[hash.Hash256][]byte
	// hasFirstTouch tracks whether a key has been recorded (distinguishing nil-value from not-yet-seen).
	hasFirstTouch map[hash.Hash256]bool

	// hashFunc is the contract-scoped hash function (includes address prefix).
	hashFunc func([]byte) []byte
}

func newWitnessTrie(inner trie.Trie, hashFunc func([]byte) []byte, kvStore *saveOnDeleteKVStore) *witnessTrie {
	return &witnessTrie{
		inner:         inner,
		kvStore:       kvStore,
		firstTouch:    make(map[hash.Hash256][]byte),
		hasFirstTouch: make(map[hash.Hash256]bool),
		hashFunc:      hashFunc,
	}
}

// capturePrestate lazily clones the inner trie before the first mutation.
// When kvStore is set (production), the clone uses the saveOnDeleteKVStore
// which preserves deleted nodes so that GetProof can still traverse them.
// When kvStore is nil (tests with MemKVStore), it falls back to CollectNodes.
func (w *witnessTrie) capturePrestate() error {
	if w.prestate != nil {
		return nil
	}
	var cloneKV trie.KVStore
	if w.kvStore != nil {
		// Production path: clone with saveOnDeleteKVStore.
		// Deleted nodes are saved on the fly, so no upfront collection needed.
		cloneKV = w.kvStore
	} else {
		// Test path: inner trie is MemKVStore-backed, collect all nodes.
		cloneKV = trie.NewMemKVStore()
		if nc, ok := w.inner.(trie.NodeCollector); ok {
			if err := nc.CollectNodes(cloneKV); err != nil {
				return errors.Wrap(err, "failed to collect prestate trie nodes")
			}
		}
	}
	clone, err := w.inner.Clone(cloneKV)
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

	// Determine which keys were deleted (present in prestate, absent in poststate).
	deleted := make(map[hash.Hash256]bool)
	for key := range w.hasFirstTouch {
		if w.firstTouch[key] != nil {
			// Key existed in prestate; check if it was deleted during execution.
			if _, err := w.inner.Get(key[:]); err != nil {
				deleted[key] = true
			}
		}
	}

	pt, isProofTrie := proofSource.(trie.ProofTrie)
	// siblingProver collects proof + sibling serializations for deleted keys.
	type siblingProver interface {
		GetProofWithSiblings([]byte) ([][]byte, error)
	}
	sp, hasSiblingProver := proofSource.(siblingProver)

	for key := range w.hasFirstTouch {
		entries = append(entries, ContractStorageWitnessEntry{
			Key:   key,
			Value: w.firstTouch[key], // nil for absent keys
		})
		if !isProofTrie {
			continue
		}

		// For deleted keys, use GetProofWithSiblings to include sibling nodes
		// needed during branch collapse on the validator side.
		if deleted[key] && hasSiblingProver {
			proofNodes, err := sp.GetProofWithSiblings(key[:])
			if err != nil && errors.Cause(err) != trie.ErrNotExist {
				return nil, errors.Wrapf(err, "failed to get proof with siblings for key %x", key[:])
			}
			for _, node := range proofNodes {
				h := w.hashFunc(node)
				proofNodeSet[string(h)] = node
			}
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

	// Also include nodes loaded at runtime from the saveOnDeleteKVStore.
	// This catches any additional nodes loaded during inner trie execution
	// (e.g. Upsert that splits an extension node, loading a hashNode sibling).
	if w.kvStore != nil {
		for h, data := range w.kvStore.loaded {
			if _, ok := proofNodeSet[h]; !ok {
				proofNodeSet[h] = data
			}
		}
	}

	// Flatten deduplicated proof nodes.
	dedupedNodes := make([][]byte, 0, len(proofNodeSet))
	for _, node := range proofNodeSet {
		dedupedNodes = append(dedupedNodes, node)
	}

	witness := &ContractStorageWitness{
		StorageRoot: storageRoot,
		Entries:     entries,
		ProofNodes:  dedupedNodes,
	}
	// Self-test: replay all entries via a temporary stateless MPT to ensure
	// the generated proof nodes are sufficient for the validator.
	// Only run in production mode (kvStore != nil) where hash functions are
	// guaranteed to be consistent between the trie and the witness trie.
	if w.kvStore != nil {
		if err := w.selfTestWitness(witness); err != nil {
			return nil, errors.Wrap(err, "witness self-test failed (producer bug)")
		}
	}
	return witness, nil
}

// selfTestWitness creates a temporary stateless MPT from the witness and
// verifies that every entry key can be read-back correctly. It also replays
// the mutations (Upsert/Delete) that occurred during execution to catch proof
// insufficiency for operations like Delete branch collapse that need sibling
// nodes beyond the read path.
func (w *witnessTrie) selfTestWitness(witness *ContractStorageWitness) error {
	if witness.StorageRoot == hash.ZeroHash256 {
		return nil // empty trie, nothing to test
	}

	kvStore := trie.NewMemKVStore()
	for _, node := range witness.ProofNodes {
		h := w.hashFunc(node)
		if err := kvStore.Put(h, node); err != nil {
			return err
		}
	}

	options := []mptrie.Option{
		mptrie.KVStoreOption(kvStore),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(w.hashFunc),
		mptrie.RootHashOption(witness.StorageRoot[:]),
	}
	tr, err := mptrie.New(options...)
	if err != nil {
		return errors.Wrapf(err, "self-test: failed to create MPT from proof (root %x)", witness.StorageRoot[:])
	}
	if err := tr.Start(context.Background()); err != nil {
		return errors.Wrapf(err, "self-test: failed to start MPT (root %x)", witness.StorageRoot[:])
	}

	// Phase 1: verify all entries can be read from the prestate proof.
	for _, entry := range witness.Entries {
		v, err := tr.Get(entry.Key[:])
		if entry.Value == nil {
			// key should not exist
			if err == nil {
				return errors.Errorf("self-test: key %x should be absent but got value %x (root %x)",
					entry.Key[:], v, witness.StorageRoot[:])
			}
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "self-test: Get failed for key %x (root %x, %d proof nodes)",
				entry.Key[:], witness.StorageRoot[:], len(witness.ProofNodes))
		}
	}

	// Phase 2: replay the actual mutations to catch proof insufficiency
	// during Upsert/Delete (e.g. sibling nodes needed for branch collapse).
	if w.inner != nil {
		for key := range w.hasFirstTouch {
			// Check the current value in the inner (post-execution) trie.
			curVal, curErr := w.inner.Get(key[:])
			preVal := w.firstTouch[key]
			if preVal == nil && curErr == nil {
				// Key was absent in prestate, now present → Upsert
				if err := tr.Upsert(key[:], curVal); err != nil {
					return errors.Wrapf(err, "self-test: Upsert replay failed for key %x", key[:])
				}
			} else if preVal != nil && curErr != nil {
				// Key was present in prestate, now absent → Delete
				if err := tr.Delete(key[:]); err != nil {
					return errors.Wrapf(err, "self-test: Delete replay failed for key %x", key[:])
				}
			} else if preVal != nil && curErr == nil {
				// Key existed and still exists → update value
				if err := tr.Upsert(key[:], curVal); err != nil {
					return errors.Wrapf(err, "self-test: Upsert (update) replay failed for key %x", key[:])
				}
			}
			// preVal == nil && curErr != nil: was absent, still absent → no-op
		}
	}

	return nil
}

// --- Delegate remaining trie.Trie methods to inner ---

func (w *witnessTrie) Start(ctx context.Context) error          { return w.inner.Start(ctx) }
func (w *witnessTrie) Stop(ctx context.Context) error           { return w.inner.Stop(ctx) }
func (w *witnessTrie) RootHash() ([]byte, error)                { return w.inner.RootHash() }
func (w *witnessTrie) SetRootHash(hash []byte) error            { return w.inner.SetRootHash(hash) }
func (w *witnessTrie) IsEmpty() bool                            { return w.inner.IsEmpty() }
func (w *witnessTrie) Clone(kv trie.KVStore) (trie.Trie, error) { return w.inner.Clone(kv) }
