package erigonstore

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/pkg/enc"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type rewardHistoryStorage struct {
	contract systemcontracts.StorageContract
	height   uint64
	keySplit KeySplitter
}

func newRewardHistoryStorage(contract systemcontracts.StorageContract, height uint64, keySplit KeySplitter) *rewardHistoryStorage {
	return &rewardHistoryStorage{
		contract: contract,
		height:   height,
		keySplit: keySplit,
	}
}

func (ds *rewardHistoryStorage) Store(key []byte, obj any) error {
	_, err := ds.contract.Remove(key)
	return err
}

func (ds *rewardHistoryStorage) Delete(key []byte) error {
	return ds.contract.Put(key, systemcontracts.GenericValue{})
}

func (ds *rewardHistoryStorage) Load(key []byte, obj any) error {
	_, heightKey := ds.keySplit(key)
	height := enc.MachineEndian.Uint64(heightKey)
	if height > ds.height {
		return errors.Wrapf(state.ErrStateNotExist, "key: %x at height %d", key, height)
	}
	result, err := ds.contract.Get(key)
	if err != nil {
		return errors.Wrapf(err, "failed to get data for key %x", key)
	}
	if result.KeyExists {
		return errors.Wrapf(state.ErrStateNotExist, "key: %x", key)
	}
	return nil
}

func (ds *rewardHistoryStorage) Batch(keys [][]byte) (state.Iterator, error) {
	return nil, errors.New("not implemented")
}

func (ds *rewardHistoryStorage) List() (state.Iterator, error) {
	return nil, errors.New("not implemented")
}
