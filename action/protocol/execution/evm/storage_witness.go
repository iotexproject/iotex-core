package evm

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/iotexproject/go-pkgs/hash"
)

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

func touchedStorageKeys(access ContractStorageAccess) []common.Hash {
	touched := make(map[common.Hash]struct{}, len(access.Reads)+len(access.Writes))
	for _, key := range access.Reads {
		touched[key] = struct{}{}
	}
	for _, key := range access.Writes {
		touched[key] = struct{}{}
	}
	return sortedHashes(touched)
}

func cloneBytes(v []byte) []byte {
	if v == nil {
		return nil
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out
}
