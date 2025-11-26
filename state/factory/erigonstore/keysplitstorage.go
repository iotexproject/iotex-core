package erigonstore

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type KeySplitter func(key []byte) (part1 []byte, part2 []byte)

type keySplitContainer interface {
	Decode(suffix []byte, data systemcontracts.GenericValue) error
	Encode(suffix []byte) (systemcontracts.GenericValue, error)
}

type keySplitContractStorage struct {
	contract         systemcontracts.StorageContract
	keySplit         KeySplitter
	keyPrefixStorage map[string]ObjectStorage
	fallback         ObjectStorage
}

func newKeySplitContractStorageWithfallback(contract systemcontracts.StorageContract, split KeySplitter, fallback ObjectStorage, keyPrefixStorage map[string]ObjectStorage) *keySplitContractStorage {
	return &keySplitContractStorage{
		contract:         contract,
		keySplit:         split,
		fallback:         fallback,
		keyPrefixStorage: keyPrefixStorage,
	}
}

func (rhs *keySplitContractStorage) Store(key []byte, obj any) error {
	pf, sf := rhs.keySplit(key)
	fallback := rhs.matchStorage(key)
	if len(sf) == 0 {
		return fallback.Store(key, obj)
	}
	gvc, ok := obj.(keySplitContainer)
	if !ok {
		return fallback.Store(pf, obj)
	}
	value, err := gvc.Encode(sf)
	if err != nil {
		return err
	}
	return rhs.contract.Put(pf, value)
}

func (rhs *keySplitContractStorage) Load(key []byte, obj any) error {
	pf, sf := rhs.keySplit(key)
	fallback := rhs.matchStorage(key)
	if len(sf) == 0 {
		return fallback.Load(key, obj)
	}
	gvc, ok := obj.(keySplitContainer)
	if !ok {
		return fallback.Load(pf, obj)
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
	fallback := rhs.matchStorage(key)
	if len(sf) == 0 {
		return fallback.Delete(key)
	}
	return fallback.Delete(pf)
}

func (rhs *keySplitContractStorage) List() (state.Iterator, error) {
	return nil, errors.New("not implemented")
}

func (rhs *keySplitContractStorage) Batch(keys [][]byte) (state.Iterator, error) {
	return nil, errors.New("not implemented")
}

func (rhs *keySplitContractStorage) matchStorage(key []byte) ObjectStorage {
	for prefix, os := range rhs.keyPrefixStorage {
		sk := string(key)
		if len(sk) >= len(prefix) && sk[:len(prefix)] == prefix {
			return os
		}
	}
	return rhs.fallback
}
