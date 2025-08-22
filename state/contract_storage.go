package state

import (
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type ContractStorage interface {
	StoreToContract(ns string, key []byte, backend systemcontracts.ContractBackend) error
	LoadFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error
	DeleteFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error
	ListFromContract(ns string, backend systemcontracts.ContractBackend) ([][]byte, []any, error)
	BatchFromContract(ns string, keys [][]byte, backend systemcontracts.ContractBackend) ([]any, error)
}

type ContractStorageStandard interface {
	ContractStorageAddress(ns string, key []byte) (address.Address, error)
	New() ContractStorageStandard
	Serialize() ([]byte, error)
	Deserialize([]byte) error
}

type ContractStorageProxy interface {
	ContractStorageProxy() ContractStorage
}
