package erigonstore

import (
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
	"github.com/pkg/errors"
)

type KeySplitter func(key []byte) (part1 []byte, part2 []byte)

type keySplitContainer interface {
	Decode(suffix []byte, data systemcontracts.GenericValue) error
	Encode(suffix []byte) (systemcontracts.GenericValue, error)
}

type keySplitContractStorage struct {
	contract systemcontracts.StorageContract
	keySplit KeySplitter
	cos      *contractObjectStorage
}

func newKeySplitContractStorage(contract systemcontracts.StorageContract, split KeySplitter) *keySplitContractStorage {
	return &keySplitContractStorage{
		contract: contract,
		keySplit: split,
		cos:      newContractObjectStorage(contract),
	}
}

func (rhs *keySplitContractStorage) Store(key []byte, obj any) error {
	pf, sf := rhs.keySplit(key)
	if len(sf) == 0 {
		return rhs.cos.Store(key, obj)
	}
	gvc, ok := obj.(keySplitContainer)
	if !ok {
		return errors.New("object does not implement GenericValueContainer")
	}
	value, err := gvc.Encode(sf)
	if err != nil {
		return err
	}
	return rhs.contract.Put(pf, value)
}

func (rhs *keySplitContractStorage) Load(key []byte, obj any) error {
	pf, sf := rhs.keySplit(key)
	if len(sf) == 0 {
		return rhs.cos.Load(key, obj)
	}
	gvc, ok := obj.(keySplitContainer)
	if !ok {
		return errors.New("object does not implement GenericValueContainer")
	}
	value, err := rhs.contract.Get(pf)
	if err != nil {
		return err
	}
	if !value.KeyExists {
		return errors.Wrapf(state.ErrStateNotExist, "key: %x", key)
	}
	return gvc.Decode(sf, value.Value)
}

func (rhs *keySplitContractStorage) Delete(key []byte) error {
	pf, sf := rhs.keySplit(key)
	if len(sf) == 0 {
		return rhs.cos.Delete(key)
	}
	exist, err := rhs.contract.Remove(pf)
	if err != nil {
		return errors.Wrapf(err, "failed to remove data for key %x", key)
	}
	if !exist {
		return errors.Wrapf(state.ErrStateNotExist, "key: %x", key)
	}
	return nil
}

func (rhs *keySplitContractStorage) List() (state.Iterator, error) {
	return nil, errors.New("not implemented")
}

func (rhs *keySplitContractStorage) Batch(keys [][]byte) (state.Iterator, error) {
	return nil, errors.New("not implemented")
}
