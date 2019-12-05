package protocol

import (
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
)

// StateReader defines an interface to read state db
type StateReader interface {
	Height() (uint64, error)
	State(hash.Hash160, interface{}) error
}

// StateManager defines the state DB interface atop IoTeX blockchain
type StateManager interface {
	StateReader
	// Accounts
	Snapshot() int
	Revert(int) error
	// General state
	PutState(hash.Hash160, interface{}) error
	DelState(pkHash hash.Hash160) error
	GetDB() db.KVStore
	GetCachedBatch() batch.CachedBatch
}
