// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"
	"math/big"
	"sort"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/trie"
)

var (
	// ErrNotEnoughBalance is the error that the balance is not enough
	ErrNotEnoughBalance = errors.New("not enough balance")

	// ErrAccountNotExist is the error that the account does not exist
	ErrAccountNotExist = errors.New("account does not exist")

	// ErrAccountCollision is the error that the account already exists
	ErrAccountCollision = errors.New("account already exists")

	// ErrFailedToMarshalState is the error that the state marshaling is failed
	ErrFailedToMarshalState = errors.New("failed to marshal state")

	// ErrFailedToUnmarshalState is the error that the state un-marshaling is failed
	ErrFailedToUnmarshalState = errors.New("failed to unmarshal state")
)

const (
	// CurrentHeightKey indicates the key of current factory height in underlying DB
	CurrentHeightKey = "currentHeight"
	// AccountTrieRootKey indicates the key of accountTrie root hash in underlying DB
	AccountTrieRootKey = "accountTrieRoot"
)

type (
	// Factory defines an interface for managing states
	Factory interface {
		lifecycle.StartStopper
		// Accounts
		LoadOrCreateState(string, uint64) (*State, error)
		Balance(string) (*big.Int, error)
		Nonce(string) (uint64, error) // Note that Nonce starts with 1.
		State(string) (*State, error)
		CachedState(string) (*State, error)
		RootHash() hash.Hash32B
		Height() (uint64, error)
		NewWorkingSet() (WorkingSet, error)
		RunActions(uint64, []*action.Transfer, []*action.Vote, []*action.Execution, []action.Action) (hash.Hash32B, error)
		Commit(WorkingSet) error
		// Contracts
		GetCodeHash(hash.PKHash) (hash.Hash32B, error)
		GetCode(hash.PKHash) ([]byte, error)
		SetCode(hash.PKHash, []byte) error
		GetContractState(hash.PKHash, hash.Hash32B) (hash.Hash32B, error)
		SetContractState(hash.PKHash, hash.Hash32B, hash.Hash32B) error
		// Candidate pool
		candidates() (uint64, []*Candidate)
		CandidatesByHeight(uint64) ([]*Candidate, error)
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle          lifecycle.Lifecycle
		mutex              sync.RWMutex
		currentChainHeight uint64
		numCandidates      uint
		activeWs           WorkingSet      // active working set
		rootHash           hash.Hash32B    // new root hash after running executions in this block
		dao                db.KVStore      // the underlying DB for account/contract storage
		actionHandlers     []ActionHandler // the handlers to handle actions
	}

	// ActionHandler is the interface for the action handlers. For each incoming action, the assembled actions will be
	// called one by one to process it. ActionHandler implementation is supposed to parse the sub-type of the action to
	// decide if it wants to handle this action or not.
	ActionHandler interface {
		handle(action.Action) error
	}
)

// FactoryOption sets Factory construction parameter
type FactoryOption func(*factory, *config.Config) error

// PrecreatedTrieDBOption uses pre-created trie DB for state factory
func PrecreatedTrieDBOption(kv db.KVStore) FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		if kv == nil {
			return errors.New("Invalid empty trie db")
		}
		if err := kv.Start(context.Background()); err != nil {
			return errors.Wrap(err, "failed to start trie db")
		}
		sf.dao = kv
		// get state trie root
		root, err := sf.getRoot(trie.AccountKVNameSpace, AccountTrieRootKey)
		if err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying DB")
		}
		sf.rootHash = root
		return nil
	}
}

// DefaultTrieOption creates trie from config for state factory
func DefaultTrieOption() FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		dbPath := cfg.Chain.TrieDBPath
		if len(dbPath) == 0 {
			return errors.New("Invalid empty trie db path")
		}
		trieDB := db.NewBoltDB(dbPath, &cfg.DB)
		if err := trieDB.Start(context.Background()); err != nil {
			return errors.Wrap(err, "failed to start trie db")
		}
		sf.dao = trieDB
		// get state trie root
		root, err := sf.getRoot(trie.AccountKVNameSpace, AccountTrieRootKey)
		if err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying DB")
		}
		sf.rootHash = root
		return nil
	}
}

// InMemTrieOption creates in memory trie for state factory
func InMemTrieOption() FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		trieDB := db.NewMemKVStore()
		if err := trieDB.Start(context.Background()); err != nil {
			return errors.Wrap(err, "failed to start trie db")
		}
		sf.dao = trieDB
		// get state trie root
		root, err := sf.getRoot(trie.AccountKVNameSpace, AccountTrieRootKey)
		if err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying DB")
		}
		sf.rootHash = root
		return nil
	}
}

// ActionHandlerOption sets the action handlers for state factory
func ActionHandlerOption(actionHandlers ...ActionHandler) FactoryOption {
	return func(sf *factory, cfg *config.Config) error {
		sf.actionHandlers = actionHandlers
		return nil
	}
}

// NewFactory creates a new state factory
func NewFactory(cfg *config.Config, opts ...FactoryOption) (Factory, error) {
	sf := &factory{
		currentChainHeight: 0,
		numCandidates:      cfg.Chain.NumCandidates,
	}

	for _, opt := range opts {
		if err := opt(sf, cfg); err != nil {
			logger.Error().Err(err).Msgf("Failed to execute state factory creation option %p", opt)
			return nil, err
		}
	}
	// create default working set
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return nil, err
	}
	sf.activeWs = ws
	if sf.dao != nil {
		sf.lifecycle.Add(sf.dao)
	}
	return sf, nil
}

func (sf *factory) Start(ctx context.Context) error { return sf.lifecycle.OnStart(ctx) }

func (sf *factory) Stop(ctx context.Context) error { return sf.lifecycle.OnStop(ctx) }

//======================================
// State/Account functions
//======================================
// LoadOrCreateState loads existing or adds a new State with initial balance to the factory
// addr should be a bech32 properly-encoded string
func (sf *factory) LoadOrCreateState(addr string, init uint64) (*State, error) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return sf.activeWs.LoadOrCreateState(addr, init)
}

// Balance returns balance
func (sf *factory) Balance(addr string) (*big.Int, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.activeWs.balance(addr)
}

// Nonce returns the Nonce if the account exists
func (sf *factory) Nonce(addr string) (uint64, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.activeWs.Nonce(addr)
}

// State returns the confirmed state on the chain
func (sf *factory) State(addr string) (*State, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.activeWs.state(addr)
}

// CachedState returns the cached state if the address exists in local cache
func (sf *factory) CachedState(addr string) (*State, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.activeWs.CachedState(addr)
}

// RootHash returns the hash of the root node of the state trie
func (sf *factory) RootHash() hash.Hash32B {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.rootHash
}

// Height returns factory's height
func (sf *factory) Height() (uint64, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	height, err := sf.dao.Get(trie.AccountKVNameSpace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

func (sf *factory) NewWorkingSet() (WorkingSet, error) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return NewWorkingSet(sf.currentChainHeight, sf.dao, sf.rootHash, sf.actionHandlers)
}

// RunActions will be called 2 times in
// 1. In MintNewBlock(), the block producer runs all executions in new block and get the new trie root hash (which
// is written in block header), but all changes are not committed to blockchain yet
// 2. In CommitBlock(), all nodes except block producer will run all execution and verify the trie root hash match
// what's written in the block header
func (sf *factory) RunActions(
	blockHeight uint64,
	tsf []*action.Transfer,
	vote []*action.Vote,
	executions []*action.Execution,
	actions []action.Action) (hash.Hash32B, error) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	// use the default working set to run the actions
	return sf.activeWs.RunActions(blockHeight, tsf, vote, executions, actions)
}

// Commit persists all changes in RunActions() into the DB
func (sf *factory) Commit(ws WorkingSet) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	if ws != nil {
		if sf.currentChainHeight != ws.version() {
			// another working set with correct version already committed, do nothing
			return nil
		}
		sf.activeWs = nil
		sf.activeWs = ws
	}
	if err := sf.activeWs.commit(); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	// Update chain height and root
	sf.currentChainHeight = sf.activeWs.height()
	sf.rootHash = sf.activeWs.rootHash()
	return nil
}

//======================================
// Contract functions
//======================================
// GetCodeHash returns contract's code hash
func (sf *factory) GetCodeHash(addr hash.PKHash) (hash.Hash32B, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.activeWs.GetCodeHash(addr)
}

// GetCode returns contract's code
func (sf *factory) GetCode(addr hash.PKHash) ([]byte, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.activeWs.GetCode(addr)
}

// SetCode sets contract's code
func (sf *factory) SetCode(addr hash.PKHash, code []byte) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return sf.activeWs.SetCode(addr, code)
}

// GetContractState returns contract's storage value
func (sf *factory) GetContractState(addr hash.PKHash, key hash.Hash32B) (hash.Hash32B, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.activeWs.GetContractState(addr, key)
}

// SetContractState writes contract's storage value
func (sf *factory) SetContractState(addr hash.PKHash, key, value hash.Hash32B) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return sf.activeWs.SetContractState(addr, key, value)
}

//======================================
// Candidate functions
//======================================
// Candidates returns array of candidates in candidate pool
func (sf *factory) candidates() (uint64, []*Candidate) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	candidates, err := MapToCandidates(sf.activeWs.workingCandidates())
	if err != nil {
		return sf.currentChainHeight, nil
	}
	if len(candidates) <= int(sf.numCandidates) {
		return sf.currentChainHeight, candidates
	}
	sort.Sort(candidates)
	return sf.currentChainHeight, candidates[:sf.numCandidates]
}

// CandidatesByHeight returns array of candidates in candidate pool of a given height
func (sf *factory) CandidatesByHeight(height uint64) ([]*Candidate, error) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	// Load candidates on the given height from underlying db
	candidates, err := sf.activeWs.getCandidates(height)
	if err != nil {
		return []*Candidate{}, errors.Wrapf(err, "failed to get candidates on height %d", height)
	}
	if len(candidates) > int(sf.numCandidates) {
		candidates = candidates[:sf.numCandidates]
	}
	return candidates, nil
}

//======================================
// private trie constructor functions
//======================================
func (sf *factory) getRoot(nameSpace string, key string) (hash.Hash32B, error) {
	var trieRoot hash.Hash32B
	switch root, err := sf.dao.Get(nameSpace, []byte(key)); errors.Cause(err) {
	case nil:
		trieRoot = byteutil.BytesTo32B(root)
	case bolt.ErrBucketNotFound:
		trieRoot = trie.EmptyRoot
	default:
		return hash.ZeroHash32B, err
	}
	return trieRoot, nil
}
