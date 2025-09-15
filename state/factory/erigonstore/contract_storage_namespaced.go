package erigonstore

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type contractStorageNamespacedWrapper struct {
	standard ContractStorageStandard
}

// NewContractStorageNamespacedWrapper creates a new ContractStorage wrapper for namespaced storage
func NewContractStorageNamespacedWrapper(standard ContractStorageStandard) ContractStorage {
	return &contractStorageNamespacedWrapper{standard: standard}
}

func (cs *contractStorageNamespacedWrapper) StoreToContract(ns string, key []byte, backend ContractBackend) error {
	contract, err := cs.storageContract(ns, backend)
	if err != nil {
		return err
	}
	data, err := cs.standard.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize storage standard")
	}
	if err := contract.Put(ns, key, systemcontracts.NamespaceGenericValue{PrimaryData: data}); err != nil {
		return errors.Wrapf(err, "failed to store storage standard to contract %s", contract.Address().Hex())
	}
	log.S().Debugf("Stored storage standard to contract %s with key %x, value: %+v", contract.Address().Hex(), key, cs.standard)
	return nil
}

func (cs *contractStorageNamespacedWrapper) LoadFromContract(ns string, key []byte, backend ContractBackend) error {
	contract, err := cs.storageContract(ns, backend)
	if err != nil {
		return err
	}
	value, err := contract.Get(ns, key)
	if err != nil {
		return errors.Wrapf(err, "failed to get storage standard from contract %s with key %x", contract.Address().Hex(), key)
	}
	if !value.KeyExists {
		return errors.Wrapf(state.ErrStateNotExist, "storage standard does not exist in contract %s with key %x", contract.Address().Hex(), key)
	}
	if err := cs.standard.Deserialize(value.Value.PrimaryData); err != nil {
		return errors.Wrap(err, "failed to deserialize storage standard")
	}
	log.S().Debugf("Loaded storage standard from contract %s with key %x, value: %+v", contract.Address().Hex(), key, cs.standard)
	return nil
}

func (cs *contractStorageNamespacedWrapper) DeleteFromContract(ns string, key []byte, backend ContractBackend) error {
	contract, err := cs.storageContract(ns, backend)
	if err != nil {
		return err
	}
	if err := contract.Remove(ns, key); err != nil {
		return errors.Wrapf(err, "failed to delete storage standard from contract %s with key %x", contract.Address().Hex(), key)
	}
	log.S().Debugf("Deleted storage standard from contract %s with key %x", contract.Address().Hex(), key)
	return nil
}

func (cs *contractStorageNamespacedWrapper) ListFromContract(ns string, backend ContractBackend) ([][]byte, []any, error) {
	contract, err := cs.storageContract(ns, backend)
	if err != nil {
		return nil, nil, err
	}
	count, err := contract.CountInNamespace(ns)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to count storage standards in contract %s", contract.Address().Hex())
	}
	if count.Sign() == 0 {
		return nil, nil, nil
	}
	listResult, err := contract.List(ns, big.NewInt(0), count)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to list storage standards from contract %s", contract.Address().Hex())
	}
	log.S().Debugf("Listed storage standards from contract %s with keys %v", contract.Address().Hex(), listResult.KeyList)
	var indices []any
	for _, value := range listResult.Values {
		bi := cs.standard.New()
		if err := bi.Deserialize(value.PrimaryData); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to deserialize storage standard from contract %s", contract.Address().Hex())
		}
		indices = append(indices, bi)
	}
	log.S().Debugf("Listed %d storage standards from contract %s", len(indices), contract.Address().Hex())
	return listResult.KeyList, indices, nil
}

func (cs *contractStorageNamespacedWrapper) BatchFromContract(ns string, keys [][]byte, backend ContractBackend) ([]any, error) {
	contract, err := cs.storageContract(ns, backend)
	if err != nil {
		return nil, err
	}
	storeResult, err := contract.BatchGet(ns, keys)
	if err != nil {
		return nil, errors.Wrap(err, "failed to batch get storage standards from contract")
	}
	results := make([]any, 0, len(storeResult.Values))
	for i, value := range storeResult.Values {
		if !storeResult.ExistsFlags[i] {
			results = append(results, nil)
			continue
		}
		res := cs.standard.New()
		if err := res.Deserialize(value.PrimaryData); err != nil {
			return nil, errors.Wrapf(err, "failed to deserialize storage standard %x", keys[i])
		}
		results = append(results, res)
	}
	log.S().Debugf("Batch loaded %d storage standard from contract %s with keys %d", len(results), contract.Address().Hex(), len(keys))
	return results, nil
}

func (cs *contractStorageNamespacedWrapper) storageContract(ns string, backend ContractBackend) (*systemcontracts.NamespaceStorageContract, error) {
	addr, err := cs.standard.ContractStorageAddress(ns)
	if err != nil {
		return nil, err
	}
	contract, err := systemcontracts.NewNamespaceStorageContract(common.BytesToAddress(addr.Bytes()), backend)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block meta storage contract")
	}
	return contract, nil
}
