package db

import "github.com/pkg/errors"

var (
	// ErrDBEmptyTrieDBPath is the error when trie db path is empty
	ErrDBEmptyTrieDBPath = errors.New("Invalid empty trie db path")
)

// CreateDAOForStateDB creates state db from config
func CreateDAOForStateDB(cfg Config, dbPath string) (KVStore, error) {
	if len(dbPath) == 0 {
		return nil, ErrDBEmptyTrieDBPath
	}
	cfg.DbPath = dbPath

	return NewBoltDB(cfg), nil

}

// CreateDAOForStateDBWithCache creates state db with cache from config
func CreateDAOForStateDBWithCache(cfg Config, dbPath string, stateDBCacheSize int) (KVStore, error) {
	dao, err := CreateDAOForStateDB(cfg, dbPath)
	if err != nil {
		return nil, err
	}

	return NewKvStoreWithCache(dao, stateDBCacheSize), nil
}
