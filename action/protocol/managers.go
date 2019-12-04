package protocol

import (
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/db"
)

// StateManager defines the state DB interface atop IoTeX blockchain
type StateManager interface {
	// Accounts
	Height() uint64
	Snapshot() int
	Revert(int) error
	// General state
	State(hash.Hash160, interface{}) error
	PutState(hash.Hash160, interface{}) error
	DelState(pkHash hash.Hash160) error
	GetDB() db.KVStore
	GetCachedBatch() db.CachedBatch
}
