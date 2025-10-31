package erigonstore

import (
	"bytes"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type kvListContainer interface {
	Encode() ([][]byte, []systemcontracts.GenericValue, error)
	Decode(keys [][]byte, values []systemcontracts.GenericValue) error
}

type kvListStorage struct {
	contract systemcontracts.StorageContract
}

func newKVListStorage(contract systemcontracts.StorageContract) *kvListStorage {
	return &kvListStorage{
		contract: contract,
	}
}

func (cos *kvListStorage) Store(prefix []byte, obj any) error {
	ct, ok := obj.(kvListContainer)
	if !ok {
		return errors.Errorf("object of type %T does not supported", obj)
	}
	cnt, err := cos.contract.Count()
	if err != nil {
		return err
	}
	retval, err := cos.contract.ListKeys(0, cnt.Uint64())
	if err != nil {
		return err
	}
	keys, values, err := ct.Encode()
	if err != nil {
		return err
	}
	newKeys := make(map[string]struct{})
	for i, k := range keys {
		nk := append(prefix, k...)
		if err := cos.contract.Put(nk, values[i]); err != nil {
			return err
		}
		newKeys[string(nk)] = struct{}{}
	}
	// remove keys not in the new list
	for _, k := range retval.KeyList {
		if len(k) >= len(prefix) && bytes.Equal(k[:len(prefix)], prefix) {
			if _, exists := newKeys[string(k)]; !exists {
				if _, err := cos.contract.Remove(k); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (cos *kvListStorage) Load(prefix []byte, obj any) error {
	ct, ok := obj.(kvListContainer)
	if !ok {
		return errors.Errorf("object of type %T does not supported", obj)
	}
	cnt, err := cos.contract.Count()
	if err != nil {
		return err
	}
	retval, err := cos.contract.List(0, cnt.Uint64())
	if err != nil {
		return err
	}
	var (
		keys   [][]byte
		values []systemcontracts.GenericValue
	)
	prefixLen := len(prefix)
	for i, k := range retval.KeyList {
		if len(k) >= prefixLen && bytes.Equal(k[:prefixLen], prefix) {
			keys = append(keys, k[prefixLen:])
			values = append(values, retval.Values[i])
		}
	}
	if len(keys) == 0 {
		return errors.Wrapf(state.ErrStateNotExist, "prefix: %x", prefix)
	}
	return ct.Decode(keys, values)
}

func (cos *kvListStorage) Delete(prefix []byte) error {
	cnt, err := cos.contract.Count()
	if err != nil {
		return err
	}
	retval, err := cos.contract.ListKeys(0, cnt.Uint64())
	if err != nil {
		return err
	}
	prefixLen := len(prefix)
	for _, k := range retval.KeyList {
		if len(k) >= prefixLen && bytes.Equal(k[:prefixLen], prefix) {
			if _, err := cos.contract.Remove(k); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cos *kvListStorage) List() (state.Iterator, error) {
	return nil, errors.New("not implemented")
}

func (cos *kvListStorage) Batch(keys [][]byte) (state.Iterator, error) {
	return nil, errors.New("not implemented")
}
