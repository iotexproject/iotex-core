package db

import "github.com/pkg/errors"

var (
	// ErrEmptyDBPath is the error when db path is empty
	ErrEmptyDBPath = errors.New("empty db path")
)

// CreateKVStore creates db from config and db path
func CreateKVStore(cfg Config, dbPath string) (KVStore, error) {
	if len(dbPath) == 0 {
		return nil, ErrEmptyDBPath
	}
	cfg.DbPath = dbPath

	return NewBoltDB(cfg), nil
}

// CreateKVStoreWithCache creates db with cache from config and db path, cacheSize
func CreateKVStoreWithCache(cfg Config, dbPath string, cacheSize int) (KVStore, error) {
	dao, err := CreateKVStore(cfg, dbPath)
	if err != nil {
		return nil, err
	}

	return NewKvStoreWithCache(dao, cacheSize), nil
}
