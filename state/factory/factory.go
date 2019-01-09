// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// AccountKVNameSpace is the bucket name for account trie
	AccountKVNameSpace = "Account"

	// CandidateKVNameSpace is the bucket name for candidate data storage
	CandidateKVNameSpace = "Candidate"

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
		AccountState(string) (*state.Account, error)
		RootHash() hash.Hash32B
		RootHashByHeight(uint64) (hash.Hash32B, error)
		Height() (uint64, error)
		NewWorkingSet() (WorkingSet, error)
		Commit(WorkingSet) error
		// Candidate pool
		CandidatesByHeight(uint64) ([]*state.Candidate, error)

		State(hash.PKHash, interface{}) error
		AddActionHandlers(...protocol.ActionHandler)
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle          lifecycle.Lifecycle
		mutex              sync.RWMutex
		currentChainHeight uint64
		numCandidates      uint
		accountTrie        trie.Trie                // global state trie
		dao                db.KVStore               // the underlying DB for account/contract storage
		actionHandlers     []protocol.ActionHandler // the handlers to handle actions
		timerFactory       *prometheustimer.TimerFactory
	}
)

// Option sets Factory construction parameter
type Option func(*factory, config.Config) error

// PrecreatedTrieDBOption uses pre-created trie DB for state factory
func PrecreatedTrieDBOption(kv db.KVStore) Option {
	return func(sf *factory, cfg config.Config) (err error) {
		if kv == nil {
			return errors.New("Invalid empty trie db")
		}
		sf.dao = kv
		return nil
	}
}

// DefaultTrieOption creates trie from config for state factory
func DefaultTrieOption() Option {
	return func(sf *factory, cfg config.Config) (err error) {
		dbPath := cfg.Chain.TrieDBPath
		if len(dbPath) == 0 {
			return errors.New("Invalid empty trie db path")
		}
		cfg.DB.DbPath = dbPath // TODO: remove this after moving TrieDBPath from cfg.Chain to cfg.DB
		sf.dao = db.NewOnDiskDB(cfg.DB)
		return nil
	}
}

// InMemTrieOption creates in memory trie for state factory
func InMemTrieOption() Option {
	return func(sf *factory, cfg config.Config) (err error) {
		sf.dao = db.NewMemKVStore()
		return nil
	}
}

// NewFactory creates a new state factory
func NewFactory(cfg config.Config, opts ...Option) (Factory, error) {
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
	dbForTrie, err := db.NewKVStoreForTrie(AccountKVNameSpace, sf.dao)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create db for trie")
	}
	if sf.accountTrie, err = trie.NewTrie(
		trie.KVStoreOption(dbForTrie),
		trie.RootKeyOption(AccountTrieRootKey),
	); err != nil {
		return nil, errors.Wrap(err, "failed to generate accountTrie from config")
	}
	sf.lifecycle.Add(sf.accountTrie)
	timerFactory, err := prometheustimer.New(
		"iotex_statefactory_perf",
		"Performance of state factory module",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(cfg.Chain.ID), 10)},
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate prometheus timer factory")
	}
	sf.timerFactory = timerFactory

	return sf, nil
}

func (sf *factory) Start(ctx context.Context) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()

	return sf.lifecycle.OnStart(ctx)
}

func (sf *factory) Stop(ctx context.Context) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()

	return sf.lifecycle.OnStop(ctx)
}

// AddActionHandlers adds action handlers to the state factory
func (sf *factory) AddActionHandlers(actionHandlers ...protocol.ActionHandler) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()

	sf.actionHandlers = append(sf.actionHandlers, actionHandlers...)
}

//======================================
// account functions
//======================================
// Balance returns balance
func (sf *factory) Balance(addr string) (*big.Int, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	account, err := sf.accountState(addr)
	if err != nil {
		return nil, err
	}
	return account.Balance, nil
}

// Nonce returns the Nonce if the account exists
func (sf *factory) Nonce(addr string) (uint64, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	account, err := sf.accountState(addr)
	if err != nil {
		return 0, err
	}
	return account.Nonce, nil
}

// account returns the confirmed account state on the chain
func (sf *factory) AccountState(addr string) (*state.Account, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()

	return sf.accountState(addr)
}

// RootHash returns the hash of the root node of the state trie
func (sf *factory) RootHash() hash.Hash32B {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.rootHash()
}

// RootHashByHeight returns the hash of the root node of the state trie at a given height
func (sf *factory) RootHashByHeight(blockHeight uint64) (hash.Hash32B, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()

	data, err := sf.dao.Get(AccountKVNameSpace, []byte(fmt.Sprintf("%s-%d", AccountTrieRootKey, blockHeight)))
	if err != nil {
		return hash.ZeroHash32B, err
	}
	var rootHash hash.Hash32B
	copy(rootHash[:], data)
	return rootHash, nil
}

// Height returns factory's height
func (sf *factory) Height() (uint64, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	height, err := sf.dao.Get(AccountKVNameSpace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

func (sf *factory) NewWorkingSet() (WorkingSet, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return NewWorkingSet(sf.currentChainHeight, sf.dao, sf.rootHash(), sf.actionHandlers)
}

// Commit persists all changes in RunActions() into the DB
func (sf *factory) Commit(ws WorkingSet) error {
	if ws == nil {
		return errors.New("working set doesn't exist")
	}
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	defer sf.timerFactory.NewTimer("Commit").End()
	if sf.currentChainHeight != ws.Version() {
		// another working set with correct version already committed, do nothing
		return fmt.Errorf(
			"current state height %d doesn't match working set version %d",
			sf.currentChainHeight,
			ws.Version(),
		)
	}
	if err := ws.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	// Update chain height and root
	sf.currentChainHeight = ws.Height()
	h := ws.RootHash()
	if err := sf.accountTrie.SetRootHash(h[:]); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	return nil
}

//======================================
// Candidate functions
//======================================
// CandidatesByHeight returns array of Candidates in candidate pool of a given height
func (sf *factory) CandidatesByHeight(height uint64) ([]*state.Candidate, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	var candidates state.CandidateList
	for h := int(height); h >= 0; h-- {
		// Load Candidates on the given height from underlying db
		candidatesKey := candidatesutil.ConstructKey(uint64(h))
		var err error
		if err = sf.State(candidatesKey, &candidates); err == nil {
			break
		}
		if errors.Cause(err) != state.ErrStateNotExist {
			return nil, errors.Wrap(err, "failed to get most recent state of candidateList")
		}
	}
	if len(candidates) == 0 {
		return nil, errors.Wrap(state.ErrStateNotExist, "failed to get most recent state of candidateList")
	}

	if len(candidates) > int(sf.numCandidates) {
		candidates = candidates[:sf.numCandidates]
	}
	return candidates, nil
}

// State returns a confirmed state in the state factory
func (sf *factory) State(addr hash.PKHash, state interface{}) error {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()

	return sf.state(addr, state)
}

//======================================
// private trie constructor functions
//======================================

func (sf *factory) rootHash() hash.Hash32B {
	return byteutil.BytesTo32B(sf.accountTrie.RootHash())
}

func (sf *factory) state(addr hash.PKHash, s interface{}) error {
	data, err := sf.accountTrie.Get(addr[:])
	if err != nil {
		if errors.Cause(err) == trie.ErrNotExist {
			return errors.Wrapf(state.ErrStateNotExist, "state of %x doesn't exist", addr)
		}
		return errors.Wrapf(err, "error when getting the state of %x", addr)
	}
	if err := state.Deserialize(s, data); err != nil {
		return errors.Wrapf(err, "error when deserializing state data into %T", s)
	}
	return nil
}

func (sf *factory) accountState(addr string) (*state.Account, error) {
	pkHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	var account state.Account
	if err := sf.state(pkHash, &account); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return state.EmptyAccount, nil
		}
		return nil, errors.Wrapf(err, "error when loading state of %x", pkHash)
	}
	return &account, nil
}
