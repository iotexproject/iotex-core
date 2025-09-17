package erigonstore

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type (
	// ObjectStorage defines an interface of object storage
	ObjectStorage interface {
		Store(key []byte, obj any) error
		Load(key []byte, obj any) error
		Delete(key []byte) error
		List() (state.Iterator, error)
		Batch(keys [][]byte) (state.Iterator, error)
	}

	contractObjectStorage struct {
		contract systemcontracts.StorageContract
	}
)

func newContractObjectStorage(contract systemcontracts.StorageContract) *contractObjectStorage {
	return &contractObjectStorage{
		contract: contract,
	}
}

func (cos *contractObjectStorage) Store(key []byte, obj any) error {
	gvc, ok := obj.(systemcontracts.GenericValueContainer)
	if !ok {
		return errors.New("object does not implement GenericValueContainer")
	}
	value, err := gvc.Encode()
	if err != nil {
		return err
	}
	return cos.contract.Put(key, value)
}

func (cos *contractObjectStorage) Load(key []byte, obj any) error {
	gvc, ok := obj.(systemcontracts.GenericValueContainer)
	if !ok {
		return errors.New("object does not implement GenericValueContainer")
	}
	value, err := cos.contract.Get(key)
	if err != nil {
		return err
	}
	// TODO: handle value.KeyExists
	return gvc.Decode(value.Value)
}

func (cos *contractObjectStorage) Delete(key []byte) error {
	return cos.contract.Remove(key)
}

func (cos *contractObjectStorage) List() (state.Iterator, error) {
	count, err := cos.contract.Count()
	if err != nil {
		return nil, err
	}
	retval, err := cos.contract.List(0, count.Uint64())
	if err != nil {
		return nil, err
	}

	return systemcontracts.NewGenericValueObjectIterator(retval.KeyList, retval.Values, nil)
}

func (cos *contractObjectStorage) Batch(keys [][]byte) (state.Iterator, error) {
	retval, err := cos.contract.BatchGet(keys)
	if err != nil {
		return nil, err
	}

	return systemcontracts.NewGenericValueObjectIterator(keys, retval.Values, retval.ExistsFlags)
}

func (cos *contractObjectStorage) Count() (*big.Int, error) {
	return cos.contract.Count()
}
