package protocol

import (
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"
)

// NamespaceOption creates an option for given namesapce
func NamespaceOption(ns string) StateOption {
	return func(sc *StateConfig) error {
		sc.Namespace = ns
		return nil
	}
}

// KeyOption sets the key for call
func KeyOption(key []byte) StateOption {
	return func(cfg *StateConfig) error {
		cfg.Key = make([]byte, len(key))
		copy(cfg.Key, key)
		return nil
	}
}

// LegacyKeyOption sets the key for call with legacy key
func LegacyKeyOption(key hash.Hash160) StateOption {
	return func(cfg *StateConfig) error {
		cfg.Key = make([]byte, len(key[:]))
		copy(cfg.Key, key[:])
		return nil
	}
}

// FilterOption sets the filter
func FilterOption(cond db.Condition, minKey, maxKey []byte) StateOption {
	return func(cfg *StateConfig) error {
		cfg.Cond = cond
		cfg.MinKey = make([]byte, len(minKey))
		copy(cfg.MinKey, minKey)
		cfg.MaxKey = make([]byte, len(maxKey))
		copy(cfg.MaxKey, maxKey)
		return nil
	}
}

// CreateStateConfig creates a config for accessing stateDB
func CreateStateConfig(opts ...StateOption) (*StateConfig, error) {
	cfg := StateConfig{}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, errors.Wrap(err, "failed to execute state option")
		}
	}
	return &cfg, nil
}

type (
	// StateConfig is the config for accessing stateDB
	StateConfig struct {
		Namespace string // namespace used by state's storage
		Key       []byte
		MinKey    []byte
		MaxKey    []byte
		Cond      db.Condition
	}

	// StateOption sets parameter for access state
	StateOption func(*StateConfig) error

	// StateReader defines an interface to read stateDB
	StateReader interface {
		Height() (uint64, error)
		State(interface{}, ...StateOption) (uint64, error)
		States(...StateOption) (uint64, state.Iterator, error)
		ReadView(string) (interface{}, error)
	}

	// StateManager defines the stateDB interface atop IoTeX blockchain
	StateManager interface {
		StateReader
		// Accounts
		Snapshot() int
		Revert(int) error
		// General state
		PutState(interface{}, ...StateOption) (uint64, error)
		DelState(...StateOption) (uint64, error)
		WriteView(string, interface{}) error
		Dock
	}

	// Dock defines an interface for protocol to read/write their private data in StateReader/Manager
	// data are stored as interface{}, user needs to type-assert on their own upon Unload()
	Dock interface {
		ProtocolDirty(string) bool
		Load(string, string, interface{}) error
		Unload(string, string, interface{}) error
		Reset()
	}
)
