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
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
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
		RootHash() hash.Hash256
		RootHashByHeight(uint64) (hash.Hash256, error)
		Height() (uint64, error)
		NewWorkingSet() (WorkingSet, error)
		Commit(WorkingSet) error
		// Candidate pool
		CandidatesByHeight(uint64) ([]*state.Candidate, error)

		State(hash.Hash160, interface{}) error
		AddActionHandlers(...protocol.ActionHandler)
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle          lifecycle.Lifecycle
		mutex              sync.RWMutex
		currentChainHeight uint64
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
	}

	for _, opt := range opts {
		if err := opt(sf, cfg); err != nil {
			log.S().Errorf("Failed to execute state factory creation option %p: %v", opt, err)
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
		log.L().Error("Failed to generate prometheus timer factory.", zap.Error(err))
	}
	sf.timerFactory = timerFactory

	return sf, nil
}

func (sf *factory) Start(ctx context.Context) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	if err := sf.dao.Start(ctx); err != nil {
		return err
	}
	return sf.lifecycle.OnStart(ctx)
}

func (sf *factory) Stop(ctx context.Context) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	if err := sf.dao.Stop(ctx); err != nil {
		return err
	}
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
func (sf *factory) RootHash() hash.Hash256 {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	return sf.rootHash()
}

// RootHashByHeight returns the hash of the root node of the state trie at a given height
func (sf *factory) RootHashByHeight(blockHeight uint64) (hash.Hash256, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()

	data, err := sf.dao.Get(AccountKVNameSpace, []byte(fmt.Sprintf("%s-%d", AccountTrieRootKey, blockHeight)))
	if err != nil {
		return hash.ZeroHash256, err
	}
	var rootHash hash.Hash256
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
	timer := sf.timerFactory.NewTimer("Commit")
	defer timer.End()
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
	// Load Candidates on the given height from underlying db
	candidatesKey := candidatesutil.ConstructKey(height)
	err := sf.state(candidatesKey, &candidates)
	log.L().Debug(
		"CandidatesByHeight",
		zap.Uint64("height", height),
		zap.Any("candidates", candidates),
		zap.Error(err),
	)
	if errors.Cause(err) == nil {
		if len(candidates) > 0 {
			return candidates, nil
		}
		err = state.ErrStateNotExist
	}
	return nil, errors.Wrapf(
		err,
		"failed to get state of candidateList for height %d",
		height,
	)
}

// State returns a confirmed state in the state factory
func (sf *factory) State(addr hash.Hash160, state interface{}) error {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()

	return sf.state(addr, state)
}

//======================================
// private trie constructor functions
//======================================

func (sf *factory) rootHash() hash.Hash256 {
	return hash.BytesToHash256(sf.accountTrie.RootHash())
}

func (sf *factory) state(addr hash.Hash160, s interface{}) error {
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

func (sf *factory) accountState(encodedAddr string) (*state.Account, error) {
	// TODO: state db shouldn't serve this function
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		return nil, errors.Wrap(err, "error when getting the pubkey hash")
	}
	pkHash := hash.BytesToHash160(addr.Bytes())
	var account state.Account
	if err := sf.state(pkHash, &account); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			account = state.EmptyAccount()
			return &account, nil
		}
		return nil, errors.Wrapf(err, "error when loading state of %x", pkHash)
	}
	return &account, nil
}
