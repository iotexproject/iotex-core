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
	switch cfg.DBType {
	case DBPebble:
		return NewPebbleDB(cfg), nil
	case DBBolt:
		return NewBoltDB(cfg), nil
	default:
		return nil, errors.Errorf("unsupported db type %s", cfg.DBType)
	}
}

// CreateKVStoreWithCache creates db with cache from config and db path, cacheSize
func CreateKVStoreWithCache(cfg Config, dbPath string, cacheSize int) (KVStore, error) {
	dao, err := CreateKVStore(cfg, dbPath)
	if err != nil {
		return nil, err
	}

	return NewKvStoreWithCache(dao, cacheSize), nil
}

// CreateKVStoreVersioned creates versioned db from config and db path
func CreateKVStoreVersioned(cfg Config, dbPath string, vns []string) (KVStore, error) {
	if len(dbPath) == 0 {
		return nil, ErrEmptyDBPath
	}
	cfg.DbPath = dbPath
	return NewKVStoreWithVersion(cfg, VersionedNamespaceOption(vns...)), nil
}
