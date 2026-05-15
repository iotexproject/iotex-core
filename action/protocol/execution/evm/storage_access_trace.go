package evm

import (
	"bytes"
	"sort"

	"github.com/ethereum/go-ethereum/common"
)

// ContractStorageAccess summarizes the contract storage keys touched during one
// EVM execution. Reads and writes are deduplicated and returned in a stable
// order for downstream sizing and witness experiments.
type ContractStorageAccess struct {
	Address common.Address
	Reads   []common.Hash
	Writes  []common.Hash
}

type storageAccessTrace struct {
	reads  map[common.Hash]struct{}
	writes map[common.Hash]struct{}
}

type storageAccessTracer struct {
	contracts map[common.Address]*storageAccessTrace
}

func newStorageAccessTracer() *storageAccessTracer {
	return &storageAccessTracer{
		contracts: make(map[common.Address]*storageAccessTrace),
	}
}

func (t *storageAccessTracer) recordRead(addr common.Address, key common.Hash) {
	trace := t.ensureContract(addr)
	trace.reads[key] = struct{}{}
}

func (t *storageAccessTracer) recordWrite(addr common.Address, key common.Hash) {
	trace := t.ensureContract(addr)
	trace.writes[key] = struct{}{}
}

func (t *storageAccessTracer) ensureContract(addr common.Address) *storageAccessTrace {
	trace, ok := t.contracts[addr]
	if ok {
		return trace
	}
	trace = &storageAccessTrace{
		reads:  make(map[common.Hash]struct{}),
		writes: make(map[common.Hash]struct{}),
	}
	t.contracts[addr] = trace
	return trace
}

func (t *storageAccessTracer) list() []ContractStorageAccess {
	if len(t.contracts) == 0 {
		return nil
	}
	addrs := make([]common.Address, 0, len(t.contracts))
	for addr := range t.contracts {
		addrs = append(addrs, addr)
	}
	sort.Slice(addrs, func(i, j int) bool {
		return bytes.Compare(addrs[i][:], addrs[j][:]) < 0
	})

	accesses := make([]ContractStorageAccess, 0, len(addrs))
	for _, addr := range addrs {
		trace := t.contracts[addr]
		accesses = append(accesses, ContractStorageAccess{
			Address: addr,
			Reads:   sortedHashes(trace.reads),
			Writes:  sortedHashes(trace.writes),
		})
	}
	return accesses
}

func sortedHashes(m map[common.Hash]struct{}) []common.Hash {
	if len(m) == 0 {
		return nil
	}
	hashes := make([]common.Hash, 0, len(m))
	for h := range m {
		hashes = append(hashes, h)
	}
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i][:], hashes[j][:]) < 0
	})
	return hashes
}
