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
