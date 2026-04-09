package evm

import "github.com/iotexproject/go-pkgs/hash"

// ContractStorageWitnessEntry captures the proven pre-state value for one
// touched storage slot. A nil Value means the slot was absent in pre-state.
type ContractStorageWitnessEntry struct {
	Key   hash.Hash256
	Value []byte
}

// ContractStorageWitness is the minimal per-contract witness payload for the
// contract-storage-only validation path.
type ContractStorageWitness struct {
	StorageRoot hash.Hash256
	Entries     []ContractStorageWitnessEntry
	ProofNodes  [][]byte
}

// MergeFrom merges entries and proof nodes from another witness into this one.
// This is needed when the same contract is accessed in both a reverted sub-call
// and a subsequent non-reverted call — each access creates its own witnessTrie
// so the proof nodes must be unioned.
func (w *ContractStorageWitness) MergeFrom(other *ContractStorageWitness) {
	// Merge entries (deduplicate by key)
	existingKeys := make(map[hash.Hash256]bool, len(w.Entries))
	for _, e := range w.Entries {
		existingKeys[e.Key] = true
	}
	for _, e := range other.Entries {
		if !existingKeys[e.Key] {
			w.Entries = append(w.Entries, e)
		}
	}
	// Merge proof nodes (deduplicate by content)
	existingNodes := make(map[string]bool, len(w.ProofNodes))
	for _, n := range w.ProofNodes {
		existingNodes[string(n)] = true
	}
	for _, n := range other.ProofNodes {
		if !existingNodes[string(n)] {
			w.ProofNodes = append(w.ProofNodes, n)
		}
	}
}
