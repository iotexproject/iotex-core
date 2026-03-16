package evm

import "github.com/ethereum/go-ethereum/common"

// ContractStorageAccess summarizes the contract storage keys touched during one
// EVM execution. Reads and writes are deduplicated and returned in a stable order.
type ContractStorageAccess struct {
	Address common.Address
	Reads   []common.Hash
	Writes  []common.Hash
}
