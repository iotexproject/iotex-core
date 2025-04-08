package protocol

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/state"
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

// KeysOption sets the key for call
func KeysOption(f func() ([][]byte, error)) StateOption {
	return func(cfg *StateConfig) (err error) {
		cfg.Keys, err = f()
		return err
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
		Keys      [][]byte
	}

	// StateOption sets parameter for access state
	StateOption func(*StateConfig) error

	// StateReader defines an interface to read stateDB
	StateReader interface {
		Height() (uint64, error)
		State(interface{}, ...StateOption) (uint64, error)
		States(...StateOption) (uint64, state.Iterator, error)
		ReadView(string) (View, error)
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
		WriteView(string, View) error
	}
)

type (
	SimulateOption       func(*SimulateOptionConfig)
	SimulateOptionConfig struct {
		PreOpt     func(StateManager) error
		Nonce, Gas uint64
		GasPrice   *big.Int
	}
)

func WithSimulatePreOpt(fn func(StateManager) error) SimulateOption {
	return func(so *SimulateOptionConfig) {
		so.PreOpt = fn
	}
}
