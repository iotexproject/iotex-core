package systemcontracts

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

// NamespaceStorageContractWrapper wraps the NamespaceStorageContract
type NamespaceStorageContractWrapper struct {
	contract NamespaceStorageContract
	ns       string
}

func NewNamespaceStorageContractWrapper(contractAddress common.Address, backend ContractBackend, owner common.Address, ns string) (*NamespaceStorageContractWrapper, error) {
	if ns == "" {
		return nil, errors.New("namespace cannot be empty")
	}

	contract, err := NewNamespaceStorageContract(contractAddress, backend, owner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create NamespaceStorage contract")
	}

	return &NamespaceStorageContractWrapper{
		contract: *contract,
		ns:       ns,
	}, nil
}

// Address returns the contract address
func (ns *NamespaceStorageContractWrapper) Address() common.Address {
	return ns.contract.Address()
}

// Put stores data with the given namespace and key
func (ns *NamespaceStorageContractWrapper) Put(key []byte, value NamespaceGenericValue) error {
	return ns.contract.Put(ns.ns, key, value)
}

// Get retrieves data by namespace and key
func (ns *NamespaceStorageContractWrapper) Get(key []byte) (*NamespaceGetResult, error) {
	return ns.contract.Get(ns.ns, key)
}

// Remove deletes data by namespace and key
func (ns *NamespaceStorageContractWrapper) Remove(key []byte) error {
	return ns.contract.Remove(ns.ns, key)
}

// Exists checks if a key exists in a namespace
func (ns *NamespaceStorageContractWrapper) Exists(key []byte) (bool, error) {
	return ns.contract.Exists(ns.ns, key)
}

// BatchGet retrieves multiple values by their keys within a namespace
func (ns *NamespaceStorageContractWrapper) BatchGet(keys [][]byte) (*NamespaceBatchGetResult, error) {
	return ns.contract.BatchGet(ns.ns, keys)
}

// BatchPut stores multiple key-value pairs in the same namespace
func (ns *NamespaceStorageContractWrapper) BatchPut(keys [][]byte, values []NamespaceGenericValue) error {
	return ns.contract.BatchPut(ns.ns, keys, values)
}

// List retrieves all stored data in a namespace with pagination
func (ns *NamespaceStorageContractWrapper) List(offset, limit uint64) (*NamespaceListResult, error) {
	return ns.contract.List(ns.ns, big.NewInt(int64(offset)), big.NewInt(int64(limit)))
}

// ListKeys retrieves all keys in a namespace with pagination
func (ns *NamespaceStorageContractWrapper) ListKeys(offset, limit uint64) (*NamespaceListKeysResult, error) {
	return ns.contract.ListKeys(ns.ns, big.NewInt(int64(offset)), big.NewInt(int64(limit)))
}

// Count returns the number of items in the namespace
func (ns *NamespaceStorageContractWrapper) Count() (*big.Int, error) {
	return ns.contract.CountInNamespace(ns.ns)
}

// Clear removes all data in the namespace
func (ns *NamespaceStorageContractWrapper) Clear() error {
	return ns.contract.ClearNamespace(ns.ns)
}
