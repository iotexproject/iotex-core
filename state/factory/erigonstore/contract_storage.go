package erigonstore

import (
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/state"
)

type ContractBackend = state.ContractBackend

// ContractStorage defines the interface for contract storage operations
type ContractStorage interface {
	StoreToContract(ns string, key []byte, backend ContractBackend) error
	LoadFromContract(ns string, key []byte, backend ContractBackend) error
	DeleteFromContract(ns string, key []byte, backend ContractBackend) error
	ListFromContract(ns string, backend ContractBackend) ([][]byte, []any, error)
	BatchFromContract(ns string, keys [][]byte, backend ContractBackend) ([]any, error)
}

// ContractStorageStandard defines the interface for standard contract storage operations
type ContractStorageStandard interface {
	ContractStorageAddress(ns string) (address.Address, error)
	New() ContractStorageStandard
	Serialize() ([]byte, error)
	Deserialize([]byte) error
}

// ContractStorageProxy defines the interface for contract storage proxy operations
type ContractStorageProxy interface {
	ContractStorageProxy() ContractStorage
}
