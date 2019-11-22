package protocol

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
)

// ChainManager defines the blockchain interface
type ChainManager interface {
	// ChainID returns the chain ID
	ChainID() uint32
	// CandidatesByHeight returns the candidate list by a given height
	CandidatesByHeight(height uint64) ([]*state.Candidate, error)
	// ProductivityByEpoch returns the number of produced blocks per delegate in an epoch
	ProductivityByEpoch(epochNum uint64) (uint64, map[string]uint64, error)
	// SimulateExecution simulates a running of smart contract operation
	SimulateExecution(caller address.Address, ex *action.Execution) ([]byte, *action.Receipt, error)
}

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

// DummyChainManager mocks ChainManager interface
type DummyChainManager struct {
}

// Nonce mocks base method
func (m *DummyChainManager) Nonce(addr string) (uint64, error) {
	if strings.EqualFold("io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6", addr) {
		return 0, errors.New("DummyChainManager nonce error")
	}
	return 2, nil
}

// ChainID return chain ID
func (m *DummyChainManager) ChainID() uint32 {
	return 0
}

// CandidatesByHeight returns the candidate list by a given height
func (m *DummyChainManager) CandidatesByHeight(height uint64) ([]*state.Candidate, error) {
	return nil, nil
}

// ProductivityByEpoch returns the number of produced blocks per delegate in an epoch
func (m *DummyChainManager) ProductivityByEpoch(epochNum uint64) (uint64, map[string]uint64, error) {
	return 0, nil, nil
}

// SimulateExecution simulates a running of smart contract operation
func (m *DummyChainManager) SimulateExecution(caller address.Address, ex *action.Execution) ([]byte, *action.Receipt, error) {
	return nil, nil, nil
}
