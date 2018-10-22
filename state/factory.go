// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"
	"math/big"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
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
		Balance(string) (*big.Int, error)
		Nonce(string) (uint64, error) // Note that Nonce starts with 1.
		AccountState(string) (*Account, error)
		RootHash() hash.Hash32B
		Height() (uint64, error)
		NewWorkingSet() (WorkingSet, error)
		Commit(WorkingSet) error
		// Candidate pool
		CandidatesByHeight(uint64) ([]*Candidate, error)

		State(hash.PKHash, State) (State, error)
		AddActionHandlers(...ActionHandler)
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle          lifecycle.Lifecycle
		mutex              sync.RWMutex
		currentChainHeight uint64
		numCandidates      uint
		rootHash           hash.Hash32B    // new root hash after running executions in this block
		accountTrie        trie.Trie       // global state trie
		dao                db.KVStore      // the underlying DB for account/contract storage
		actionHandlers     []ActionHandler // the handlers to handle actions
	}

	// ActionHandler is the interface for the action handlers. For each incoming action, the assembled actions will be
	// called one by one to process it. ActionHandler implementation is supposed to parse the sub-type of the action to
	// decide if it wants to handle this action or not.
	ActionHandler interface {
		Handle(action.Action, WorkingSet) error
	}
)

// FactoryOption sets Factory construction parameter
type FactoryOption func(*factory, *config.Config) error

// PrecreatedTrieDBOption uses pre-created trie DB for state factory
func PrecreatedTrieDBOption(kv db.KVStore) FactoryOption {
	return func(sf *factory, cfg *config.Config) (err error) {
		if kv == nil {
			return errors.New("Invalid empty trie db")
		}
		if err = kv.Start(context.Background()); err != nil {
			return errors.Wrap(err, "failed to start trie db")
		}
		sf.dao = kv
		// get state trie root
		if sf.rootHash, err = sf.getRoot(trie.AccountKVNameSpace, AccountTrieRootKey); err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying DB")
		}
		if sf.accountTrie, err = trie.NewTrie(sf.dao, trie.AccountKVNameSpace, sf.rootHash); err != nil {
			return errors.Wrap(err, "failed to generate accountTrie from config")
		}
		return nil
	}
}

// DefaultTrieOption creates trie from config for state factory
func DefaultTrieOption() FactoryOption {
	return func(sf *factory, cfg *config.Config) (err error) {
		dbPath := cfg.Chain.TrieDBPath
		if len(dbPath) == 0 {
			return errors.New("Invalid empty trie db path")
		}
		trieDB := db.NewBoltDB(dbPath, &cfg.DB)
		if err = trieDB.Start(context.Background()); err != nil {
			return errors.Wrap(err, "failed to start trie db")
		}
		sf.dao = trieDB
		// get state trie root
		if sf.rootHash, err = sf.getRoot(trie.AccountKVNameSpace, AccountTrieRootKey); err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying DB")
		}
		if sf.accountTrie, err = trie.NewTrie(sf.dao, trie.AccountKVNameSpace, sf.rootHash); err != nil {
			return errors.Wrap(err, "failed to generate accountTrie from config")
		}
		return nil
	}
}

// InMemTrieOption creates in memory trie for state factory
func InMemTrieOption() FactoryOption {
	return func(sf *factory, cfg *config.Config) (err error) {
		trieDB := db.NewMemKVStore()
		if err = trieDB.Start(context.Background()); err != nil {
			return errors.Wrap(err, "failed to start trie db")
		}
		sf.dao = trieDB
		// get state trie root
		if sf.rootHash, err = sf.getRoot(trie.AccountKVNameSpace, AccountTrieRootKey); err != nil {
			return errors.Wrap(err, "failed to get accountTrie's root hash from underlying DB")
		}
		if sf.accountTrie, err = trie.NewTrie(sf.dao, trie.AccountKVNameSpace, sf.rootHash); err != nil {
			return errors.Wrap(err, "failed to generate accountTrie from config")
		}
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
	if sf.accountTrie != nil {
		sf.lifecycle.Add(sf.accountTrie)
	}
	return sf, nil
}

func (sf *factory) Start(ctx context.Context) error { return sf.lifecycle.OnStart(ctx) }

func (sf *factory) Stop(ctx context.Context) error { return sf.lifecycle.OnStop(ctx) }

// AddActionHandlers adds action handlers to the state factory
func (sf *factory) AddActionHandlers(actionHandlers ...ActionHandler) {
	sf.actionHandlers = append(sf.actionHandlers, actionHandlers...)
}

//======================================
// account functions
//======================================
// Balance returns balance
func (sf *factory) Balance(addr string) (*big.Int, error) {
	account, err := sf.AccountState(addr)
	if err != nil {
		return nil, err
	}
	return account.Balance, nil
}

// Nonce returns the Nonce if the account exists
func (sf *factory) Nonce(addr string) (uint64, error) {
	account, err := sf.AccountState(addr)
	if err != nil {
		return 0, err
	}
	return account.Nonce, nil
}

// account returns the confirmed account state on the chain
func (sf *factory) AccountState(addr string) (*Account, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()

	pkHash, err := addressToPKHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	var account Account
	state, err := sf.State(pkHash, &account)
	if err != nil {
		return nil, errors.Wrapf(err, "error when loading state of %x", pkHash)
	}
	accountPtr, ok := state.(*Account)
	if !ok {
		return nil, errors.New("error when casting state into account")
	}
	return accountPtr, nil
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

// Commit persists all changes in RunActions() into the DB
func (sf *factory) Commit(ws WorkingSet) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	if ws != nil {
		if sf.currentChainHeight != ws.Version() {
			// another working set with correct version already committed, do nothing
			return nil
		}
	}
	if err := ws.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	// Update chain height and root
	sf.currentChainHeight = ws.Height()
	sf.rootHash = ws.RootHash()
	if err := sf.accountTrie.SetRoot(sf.rootHash); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	return nil
}

//======================================
// Candidate functions
//======================================
// CandidatesByHeight returns array of Candidates in candidate pool of a given height
func (sf *factory) CandidatesByHeight(height uint64) ([]*Candidate, error) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	// Load Candidates on the given height from underlying db
	candidatesBytes, err := sf.dao.Get(trie.CandidateKVNameSpace, byteutil.Uint64ToBytes(height))
	if err != nil {
		return []*Candidate{}, errors.Wrapf(err, "failed to get candidates on Height %d", height)
	}
	var candidates CandidateList
	if err := candidates.Deserialize(candidatesBytes); err != nil {
		return []*Candidate{}, errors.Wrapf(err, "failed to get candidates on height %d", height)
	}
	if len(candidates) > int(sf.numCandidates) {
		candidates = candidates[:sf.numCandidates]
	}
	return candidates, nil
}

// State returns a confirmed state in the state factory
func (sf *factory) State(addr hash.PKHash, state State) (State, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	data, err := sf.accountTrie.Get(addr[:])
	if err != nil {
		if errors.Cause(err) == trie.ErrNotExist {
			return nil, errors.Wrapf(ErrAccountNotExist, "state of %x doesn't exist", addr)
		}
		return nil, errors.Wrapf(err, "error when getting the state of %x", addr)
	}
	if err := state.Deserialize(data); err != nil {
		return nil, errors.Wrapf(err, "error when deserializing state data into %T", state)
	}
	return state, nil
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

func addressToPKHash(addr string) (hash.PKHash, error) {
	var pkHash hash.PKHash
	senderPKHashBytes, err := iotxaddress.GetPubkeyHash(addr)
	if err != nil {
		return pkHash, errors.Wrap(err, "cannot get the hash of the address")
	}
	return byteutil.BytesTo20B(senderPKHashBytes), nil
}
