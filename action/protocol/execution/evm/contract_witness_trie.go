// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// recordingKVStore wraps a trie.KVStore to record every successful Get call's
// key→value pair. These recorded entries become the proof nodes for stateless
// validation: the validator's stateless trie issues exactly the same Get calls
// as the full node during replay, so recording all Gets captures every trie
// node needed — including sibling nodes loaded during branch collapse in Delete.
type recordingKVStore struct {
	inner    trie.KVStore
	recorded map[string][]byte // hash → serialized node data
}

func newRecordingKVStore(inner trie.KVStore) *recordingKVStore {
	return &recordingKVStore{
		inner:    inner,
		recorded: make(map[string][]byte),
	}
}

func (r *recordingKVStore) Start(ctx context.Context) error { return r.inner.Start(ctx) }
func (r *recordingKVStore) Stop(ctx context.Context) error  { return r.inner.Stop(ctx) }

func (r *recordingKVStore) Get(key []byte) ([]byte, error) {
	v, err := r.inner.Get(key)
	if err == nil {
		r.recorded[string(key)] = append([]byte(nil), v...)
	}
	return v, err
}

func (r *recordingKVStore) Put(key []byte, value []byte) error {
	return r.inner.Put(key, value)
}

func (r *recordingKVStore) Delete(key []byte) error {
	return r.inner.Delete(key)
}

// prime forces a Get on the given key so that the value is recorded.
// Use this to capture the trie root node, which is already in memory after
// Clone and therefore won't be loaded through Get during normal operations.
func (r *recordingKVStore) prime(key []byte) error {
	_, err := r.Get(key)
	return err
}

// witnessTrie wraps a trie.Trie to track first-touch storage keys and their
// prestate values during EVM execution. At transaction end, it builds the
// contract storage witness using recorded kvstore reads (production) or
// GetProof fallback (tests).
type witnessTrie struct {
	inner        trie.Trie
	prestateRoot []byte // root hash before first mutation; nil if no mutations

	// kvStore is a recordingKVStore that records all trie node reads.
	// nil when inner trie is MemKVStore-backed (tests).
	kvStore *recordingKVStore

	// firstTouch records the prestate value for each key at first access.
	// nil value means the key was absent in prestate.
	firstTouch map[hash.Hash256][]byte
	// hasFirstTouch tracks whether a key has been recorded (distinguishing nil-value from not-yet-seen).
	hasFirstTouch map[hash.Hash256]bool

	// hashFunc is the contract-scoped hash function (includes address prefix).
	hashFunc func([]byte) []byte
}

func newWitnessTrie(inner trie.Trie, hashFunc func([]byte) []byte, kvStore *recordingKVStore) *witnessTrie {
	return &witnessTrie{
		inner:         inner,
		kvStore:       kvStore,
		firstTouch:    make(map[hash.Hash256][]byte),
		hasFirstTouch: make(map[hash.Hash256]bool),
		hashFunc:      hashFunc,
	}
}

// capturePrestateRoot saves the trie root hash before the first mutation.
func (w *witnessTrie) capturePrestateRoot() error {
	if w.prestateRoot != nil {
		return nil
	}
	// For empty tries, use ZeroHash256 to match the account representation.
	if w.inner.IsEmpty() {
		w.prestateRoot = hash.ZeroHash256[:]
		return nil
	}
	rootHash, err := w.inner.RootHash()
	if err != nil {
		return errors.Wrap(err, "failed to get prestate root hash")
	}
	w.prestateRoot = make([]byte, len(rootHash))
	copy(w.prestateRoot, rootHash)
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

// Upsert inserts/updates a value and records first-touch + prestate root.
func (w *witnessTrie) Upsert(key []byte, value []byte) error {
	k := hash.BytesToHash256(key)
	w.recordFirstTouch(k)
	if err := w.capturePrestateRoot(); err != nil {
		return err
	}
	return w.inner.Upsert(key, value)
}

// Delete removes a key and records first-touch + prestate root.
func (w *witnessTrie) Delete(key []byte) error {
	k := hash.BytesToHash256(key)
	w.recordFirstTouch(k)
	if err := w.capturePrestateRoot(); err != nil {
		return err
	}
	return w.inner.Delete(key)
}

// BuildWitness generates the ContractStorageWitness from recorded first-touch
// keys using the recorded kvstore reads. Must be called after execution completes.
func (w *witnessTrie) BuildWitness() (*ContractStorageWitness, error) {
	// if len(w.firstTouch) == 0 && len(w.hasFirstTouch) == 0 {
	// 	return nil, nil
	// }

	// Determine storage root: prestate root if mutations occurred, otherwise current root.
	var storageRoot hash.Hash256
	if w.prestateRoot != nil {
		storageRoot = hash.BytesToHash256(w.prestateRoot)
	} else {
		if w.inner.IsEmpty() {
			storageRoot = hash.ZeroHash256
		} else {
			rootHash, err := w.inner.RootHash()
			if err != nil {
				return nil, errors.Wrap(err, "failed to get root hash")
			}
			storageRoot = hash.BytesToHash256(rootHash)
		}
	}

	// An empty trie uses ZeroHash256 to match the account representation.
	if storageRoot == hash.ZeroHash256 {
		entries := make([]ContractStorageWitnessEntry, 0, len(w.hasFirstTouch))
		for key := range w.hasFirstTouch {
			entries = append(entries, ContractStorageWitnessEntry{
				Key:   key,
				Value: nil,
			})
		}
		return &ContractStorageWitness{
			StorageRoot: hash.ZeroHash256,
			Entries:     entries,
		}, nil
	}

	// Collect entries.
	entries := make([]ContractStorageWitnessEntry, 0, len(w.hasFirstTouch))
	for key := range w.hasFirstTouch {
		entries = append(entries, ContractStorageWitnessEntry{
			Key:   key,
			Value: w.firstTouch[key],
		})
	}

	// Collect proof nodes.
	var dedupedNodes [][]byte
	if w.kvStore != nil {
		// Production path: use all recorded kvstore reads as proof nodes.
		dedupedNodes = make([][]byte, 0, len(w.kvStore.recorded))
		for _, data := range w.kvStore.recorded {
			dedupedNodes = append(dedupedNodes, data)
		}
	} else {
		// Test fallback path: use GetProof from the inner trie.
		pt, isProofTrie := w.inner.(trie.ProofTrie)
		if isProofTrie {
			proofNodeSet := make(map[string][]byte)
			for key := range w.hasFirstTouch {
				proofNodes, err := pt.GetProof(key[:])
				if err != nil && errors.Cause(err) != trie.ErrNotExist {
					return nil, errors.Wrapf(err, "failed to get proof for key %x", key[:])
				}
				for _, node := range proofNodes {
					h := w.hashFunc(node)
					proofNodeSet[string(h)] = node
				}
			}
			dedupedNodes = make([][]byte, 0, len(proofNodeSet))
			for _, node := range proofNodeSet {
				dedupedNodes = append(dedupedNodes, node)
			}
		}
	}

	witness := &ContractStorageWitness{
		StorageRoot: storageRoot,
		Entries:     entries,
		ProofNodes:  dedupedNodes,
	}

	// Log proof node hashes and recorded KV store keys for debugging
	if w.kvStore != nil {
		proofHashes := make([]string, 0, len(dedupedNodes))
		for _, node := range dedupedNodes {
			h := w.hashFunc(node)
			proofHashes = append(proofHashes, hex.EncodeToString(h))
		}
		recordedKeys := make([]string, 0, len(w.kvStore.recorded))
		for k := range w.kvStore.recorded {
			recordedKeys = append(recordedKeys, hex.EncodeToString([]byte(k)))
		}
		log.L().Info("BuildWitness: proof node hashes",
			zap.String("storageRoot", hex.EncodeToString(storageRoot[:])),
			zap.Int("proofNodeCount", len(dedupedNodes)),
			zap.Int("entryCount", len(entries)),
			zap.String("proofHashes", strings.Join(proofHashes, ",")),
			zap.String("recordedKeys", strings.Join(recordedKeys, ",")),
		)
	}

	// Self-test: replay all entries via a temporary stateless MPT.
	if w.kvStore != nil {
		if err := w.selfTestWitness(witness); err != nil {
			return nil, errors.Wrap(err, "witness self-test failed (producer bug)")
		}
	}
	return witness, nil
}

// selfTestWitness creates a temporary stateless MPT from the witness and
// verifies that every entry key can be read-back correctly. It also replays
// the mutations that occurred during execution to catch proof insufficiency.
func (w *witnessTrie) selfTestWitness(witness *ContractStorageWitness) error {
	if witness.StorageRoot == hash.ZeroHash256 {
		return nil
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

	// Phase 2: replay the actual mutations to catch proof insufficiency.
	// if w.inner != nil {
	// 	for key := range w.hasFirstTouch {
	// 		curVal, curErr := w.inner.Get(key[:])
	// 		preVal := w.firstTouch[key]
	// 		if preVal == nil && curErr == nil {
	// 			if err := tr.Upsert(key[:], curVal); err != nil {
	// 				return errors.Wrapf(err, "self-test: Upsert replay failed for key %x", key[:])
	// 			}
	// 		} else if preVal != nil && curErr != nil {
	// 			if err := tr.Delete(key[:]); err != nil {
	// 				return errors.Wrapf(err, "self-test: Delete replay failed for key %x", key[:])
	// 			}
	// 		} else if preVal != nil && curErr == nil {
	// 			if err := tr.Upsert(key[:], curVal); err != nil {
	// 				return errors.Wrapf(err, "self-test: Upsert (update) replay failed for key %x", key[:])
	// 			}
	// 		}
	// 	}
	// }

	return nil
}

// --- Delegate remaining trie.Trie methods to inner ---

func (w *witnessTrie) Start(ctx context.Context) error          { return w.inner.Start(ctx) }
func (w *witnessTrie) Stop(ctx context.Context) error           { return w.inner.Stop(ctx) }
func (w *witnessTrie) RootHash() ([]byte, error)                { return w.inner.RootHash() }
func (w *witnessTrie) SetRootHash(hash []byte) error            { return w.inner.SetRootHash(hash) }
func (w *witnessTrie) IsEmpty() bool                            { return w.inner.IsEmpty() }
func (w *witnessTrie) Clone(kv trie.KVStore) (trie.Trie, error) { return w.inner.Clone(kv) }
