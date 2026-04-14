// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
)

// witnessStateDB wraps a stateDB to intercept GetState, GetCommittedState,
// and SetState calls, capturing first-touch prestate values for witness
// generation. It sits outside the snapshot/revert mechanism, so all recorded
// data naturally survives reverts without any special handling.
type witnessStateDB struct {
	stateDB // inner stateDB (the full stateDB interface including IoTeX extensions)

	// entries records the first-touch prestate value per (addr, key).
	// The outer map is keyed by contract address, the inner by storage key.
	// A nil value in the inner map means the key was absent in prestate.
	entries map[common.Address]map[common.Hash][]byte

	// touched tracks whether a (addr, key) has been recorded to distinguish
	// nil-value (absent) from not-yet-seen.
	touched map[common.Address]map[common.Hash]bool

	// prestateRoots records the storage root of each contract at first access.
	// This is the prestate root needed to reconstruct the MPT from proof nodes.
	prestateRoots map[common.Address]hash.Hash256

	// getRootFunc fetches a contract's current storage root hash from the
	// underlying StateDBAdapter. This avoids coupling to internal types.
	getRootFunc func(common.Address) hash.Hash256
}

// newWitnessStateDB creates a witnessStateDB wrapping the given inner stateDB.
// getRootFunc provides a way to get a contract's prestate storage root.
func newWitnessStateDB(inner stateDB, getRootFunc func(common.Address) hash.Hash256) *witnessStateDB {
	return &witnessStateDB{
		stateDB:       inner,
		entries:       make(map[common.Address]map[common.Hash][]byte),
		touched:       make(map[common.Address]map[common.Hash]bool),
		prestateRoots: make(map[common.Address]hash.Hash256),
		getRootFunc:   getRootFunc,
	}
}

// recordPrestateRoot captures the storage root for a contract on first access.
func (w *witnessStateDB) recordPrestateRoot(addr common.Address) {
	if _, ok := w.prestateRoots[addr]; ok {
		return
	}
	if w.getRootFunc != nil {
		w.prestateRoots[addr] = w.getRootFunc(addr)
	}
}

// recordFirstTouch records the prestate value for (addr, key) on first access.
func (w *witnessStateDB) recordFirstTouch(addr common.Address, key common.Hash, value common.Hash) {
	m, ok := w.touched[addr]
	if !ok {
		m = make(map[common.Hash]bool)
		w.touched[addr] = m
	}
	if m[key] {
		return
	}
	m[key] = true
	em, ok := w.entries[addr]
	if !ok {
		em = make(map[common.Hash][]byte)
		w.entries[addr] = em
	}
	if value == (common.Hash{}) {
		// Zero hash means absent in EVM semantics — record nil prestate value
		em[key] = nil
	} else {
		v := make([]byte, len(value))
		copy(v, value[:])
		em[key] = v
	}
}

// GetState intercepts storage reads for witness entry capture.
func (w *witnessStateDB) GetState(addr common.Address, key common.Hash) common.Hash {
	w.recordPrestateRoot(addr)
	value := w.stateDB.GetState(addr, key)
	w.recordFirstTouch(addr, key, value)
	return value
}

// GetCommittedState intercepts committed storage reads for witness entry capture.
func (w *witnessStateDB) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	w.recordPrestateRoot(addr)
	value := w.stateDB.GetCommittedState(addr, key)
	w.recordFirstTouch(addr, key, value)
	return value
}

// SetState intercepts storage writes. EVM always calls GetCommittedState before
// SetState, so the key should already be recorded. Defensive: if not, capture
// the committed value as prestate.
func (w *witnessStateDB) SetState(addr common.Address, key common.Hash, value common.Hash) {
	w.recordPrestateRoot(addr)
	if m, ok := w.touched[addr]; !ok || !m[key] {
		committed := w.stateDB.GetCommittedState(addr, key)
		w.recordFirstTouch(addr, key, committed)
	}
	w.stateDB.SetState(addr, key, value)
}

// WitnessEntries returns the collected first-touch entries per contract.
func (w *witnessStateDB) WitnessEntries() map[common.Address][]ContractStorageWitnessEntry {
	result := make(map[common.Address][]ContractStorageWitnessEntry, len(w.entries))
	for addr, em := range w.entries {
		entries := make([]ContractStorageWitnessEntry, 0, len(em))
		for key, val := range em {
			entries = append(entries, ContractStorageWitnessEntry{
				Key:   hash.BytesToHash256(key[:]),
				Value: val,
			})
		}
		result[addr] = entries
	}
	return result
}

// PrestateRoots returns the prestate storage roots per contract.
func (w *witnessStateDB) PrestateRoots() map[common.Address]hash.Hash256 {
	return w.prestateRoots
}

// assembleWitnesses combines witness entries from the witnessStateDB with
// proof nodes from the sharedRecordingKVStore into per-contract witnesses.
func assembleWitnesses(wdb *witnessStateDB, recorder *sharedRecordingKVStore) map[common.Address]*ContractStorageWitness {
	entries := wdb.WitnessEntries()
	roots := wdb.PrestateRoots()
	witnesses := make(map[common.Address]*ContractStorageWitness)
	// Collect all contract addresses from both sources
	allAddrs := make(map[common.Address]struct{})
	for addr := range entries {
		allAddrs[addr] = struct{}{}
	}
	for _, addr := range recorder.AllContracts() {
		allAddrs[addr] = struct{}{}
	}
	for addr := range allAddrs {
		w := &ContractStorageWitness{
			StorageRoot: roots[addr],
			Entries:     entries[addr],
			ProofNodes:  recorder.ProofNodesForContract(addr),
		}
		if len(w.Entries) > 0 || len(w.ProofNodes) > 0 {
			witnesses[addr] = w
		}
	}
	if len(witnesses) == 0 {
		return nil
	}
	return witnesses
}
