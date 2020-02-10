// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// CurrentHeightKey indicates the key of current factory height in underlying DB
	CurrentHeightKey = "currentHeight"
	// AccountTrieRootKey indicates the key of accountTrie root hash in underlying DB
	AccountTrieRootKey = "accountTrieRoot"
)

var (
	// ErrNotSupported is the error that the statedb is not for archive mode
	ErrNotSupported = errors.New("not supported")
	// ErrNoArchiveData is the error that the node have no archive data
	ErrNoArchiveData = errors.New("no archive data")
)

type (
	// Factory defines an interface for managing states
	Factory interface {
		lifecycle.StartStopper
		protocol.StateReader
		// TODO : erase this interface
		NewWorkingSet() (WorkingSet, error)
		Validate(context.Context, *block.Block) error
		SimulateExecution(context.Context, address.Address, *action.Execution, evm.GetBlockHash) ([]byte, *action.Receipt, error)
		Commit(context.Context, *block.Block) error
		DeleteWorkingSet(*block.Block) error
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle          lifecycle.Lifecycle
		mutex              sync.RWMutex
		cfg                config.Config
		currentChainHeight uint64
		saveHistory        bool
		accountTrie        trie.Trie  // global state trie
		dao                db.KVStore // the underlying DB for account/contract storage
		timerFactory       *prometheustimer.TimerFactory
		workingsets        *lru.Cache // lru cache for workingsets
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
		sf.dao = db.NewBoltDB(cfg.DB)
		sf.saveHistory = cfg.Chain.EnableHistoryStateDB
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
		cfg:                cfg,
		currentChainHeight: 0,
	}

	for _, opt := range opts {
		if err := opt(sf, cfg); err != nil {
			log.S().Errorf("Failed to execute state factory creation option %p: %v", opt, err)
			return nil, err
		}
	}
	dbForTrie, err := db.NewKVStoreForTrie(protocol.AccountNameSpace, evm.PruneKVNameSpace, sf.dao)
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
	if sf.workingsets, err = lru.New(int(cfg.Chain.WorkingSetCacheSize)); err != nil {
		return nil, errors.Wrap(err, "failed to generate lru cache for workingsets")
	}

	return sf, nil
}

func (sf *factory) Start(ctx context.Context) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	if err := sf.dao.Start(ctx); err != nil {
		return err
	}
	// check factory height
	_, err := sf.dao.Get(protocol.AccountNameSpace, []byte(CurrentHeightKey))
	switch errors.Cause(err) {
	case nil:
		break
	case db.ErrNotExist:
		// init the state factory
		if err := sf.createGenesisStates(ctx); err != nil {
			return errors.Wrap(err, "failed to create genesis states")
		}
	default:
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
	sf.workingsets.Purge()
	return sf.lifecycle.OnStop(ctx)
}

// Height returns factory's height
func (sf *factory) Height() (uint64, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	height, err := sf.dao.Get(protocol.AccountNameSpace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

// NewWorkingSet returns new working set
func (sf *factory) NewWorkingSet() (WorkingSet, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()

	return newWorkingSet(sf.currentChainHeight+1, sf.dao, sf.rootHash(), sf.saveHistory)
}

func (sf *factory) Validate(ctx context.Context, blk *block.Block) error {
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	ws, isExist, err := sf.getFromWorkingSets(key)
	if err != nil {
		return err
	}
	if isExist {
		return nil
	}
	if err := validateWithWorkingset(ctx, ws, blk); err != nil {
		return errors.Wrap(err, "failed to validate block with workingset in factory")
	}
	sf.putIntoWorkingSets(key, ws)
	return nil
}

// NewBlockBuilder returns block builder which hasn't been signed yet
func (sf *factory) NewBlockBuilder(
	ctx context.Context,
	actionMap map[string][]action.SealedEnvelope,
	postSystemActions []action.SealedEnvelope,
) (*block.Builder, error) {
	sf.mutex.Lock()
	ws, err := newWorkingSet(sf.currentChainHeight+1, sf.dao, sf.rootHash(), sf.saveHistory)
	sf.mutex.Unlock()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to obtain working set from state factory")
	}
	blkBuilder, err := createBuilderWithWorkingset(ctx, ws, actionMap, postSystemActions, sf.cfg.Chain.AllowedBlockGasResidue)
	if err != nil {
		return nil, err
	}

	blkCtx := protocol.MustGetBlockCtx(ctx)
	key := generateWorkingSetCacheKey(blkBuilder.GetCurrentBlockHeader(), blkCtx.Producer.String())
	sf.putIntoWorkingSets(key, ws)
	return blkBuilder, nil
}

// SimulateExecution simulates a running of smart contract operation, this is done off the network since it does not
// cause any state change
func (sf *factory) SimulateExecution(
	ctx context.Context,
	caller address.Address,
	ex *action.Execution,
	getBlockHash evm.GetBlockHash,
) ([]byte, *action.Receipt, error) {
	sf.mutex.Lock()
	ws, err := newWorkingSet(sf.currentChainHeight+1, sf.dao, sf.rootHash(), sf.saveHistory)
	sf.mutex.Unlock()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to obtain working set from state factory")
	}

	return simulateExecution(ctx, ws, caller, ex, getBlockHash)
}

// Commit persists all changes in RunActions() into the DB
func (sf *factory) Commit(ctx context.Context, blk *block.Block) error {
	sf.mutex.Lock()
	timer := sf.timerFactory.NewTimer("Commit")
	sf.mutex.Unlock()
	defer timer.End()
	producer, err := address.FromBytes(blk.PublicKey().Hash())
	if err != nil {
		return err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	ctx = protocol.WithBlockCtx(ctx,
		protocol.BlockCtx{
			BlockHeight:    blk.Height(),
			BlockTimeStamp: blk.Timestamp(),
			GasLimit:       bcCtx.Genesis.BlockGasLimit,
			Producer:       producer,
		},
	)
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	ws, isExist, err := sf.getFromWorkingSets(key)
	if err != nil {
		return err
	}
	if !isExist {
		// regenerate workingset
		_, ws, err = runActions(ctx, ws, blk.RunnableActions().Actions())
		if err != nil {
			log.L().Panic("Failed to update state.", zap.Error(err))
			return err
		}
	}
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	if sf.currentChainHeight+1 != ws.Version() {
		// another working set with correct version already committed, do nothing
		return fmt.Errorf(
			"current state height %d + 1 doesn't match working set version %d",
			sf.currentChainHeight,
			ws.Version(),
		)
	}

	return sf.commit(ws)
}

// State returns a confirmed state in the state factory
func (sf *factory) State(addr hash.Hash160, state interface{}, opts ...protocol.StateOption) error {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return err
	}
	if cfg.AtHeight {
		return sf.stateAtHeight(cfg.Height, addr, state)
	}
	return sf.state(addr, state)
}

// DeleteWorkingSet returns true if it remove ws from workingsets cache successfully
func (sf *factory) DeleteWorkingSet(blk *block.Block) error {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()

	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	sf.workingsets.Remove(key)
	return nil
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

func (sf *factory) stateAtHeight(height uint64, addr hash.Hash160, s interface{}) error {
	if !sf.saveHistory {
		return ErrNoArchiveData
	}
	// get root through height
	rootHash, err := sf.dao.Get(protocol.AccountNameSpace, []byte(fmt.Sprintf("%s-%d", AccountTrieRootKey, height)))
	if err != nil {
		return errors.Wrap(err, "failed to get root hash through height")
	}
	dbForTrie, err := db.NewKVStoreForTrie(protocol.AccountNameSpace, evm.PruneKVNameSpace, sf.dao, db.CachedBatchOption(batch.NewCachedBatch()))
	if err != nil {
		return errors.Wrap(err, "failed to generate state tire db")
	}
	tr, err := trie.NewTrie(trie.KVStoreOption(dbForTrie), trie.RootHashOption(rootHash))
	if err != nil {
		return errors.Wrap(err, "failed to generate state trie from config")
	}
	err = tr.Start(context.Background())
	if err != nil {
		return err
	}
	defer tr.Stop(context.Background())
	mstate, err := tr.Get(addr[:])
	if errors.Cause(err) == trie.ErrNotExist {
		return errors.Wrapf(state.ErrStateNotExist, "addrHash = %x", addr[:])
	}
	if err != nil {
		return errors.Wrapf(err, "failed to get account of %x", addr)
	}
	return state.Deserialize(s, mstate)
}

func (sf *factory) commit(ws WorkingSet) error {
	if err := ws.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	// Update chain height and root
	height, err := ws.Height()
	if err != nil {
		return errors.Wrap(err, "failed to get working set height")
	}
	sf.currentChainHeight = height
	h, err := ws.RootHash()
	if err != nil {
		return errors.Wrap(err, "failed to get root hash of working set")
	}
	if err := sf.accountTrie.SetRootHash(h[:]); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	return nil
}

func (sf *factory) createGenesisStates(ctx context.Context) error {
	ws, err := newWorkingSet(0, sf.dao, sf.rootHash(), sf.saveHistory)
	if err != nil {
		return errors.Wrap(err, "failed to obtain working set from state factory")
	}
	if err := createGenesisStates(ctx, ws); err != nil {
		return err
	}
	// add Genesis states
	if err := sf.commit(ws); err != nil {
		return errors.Wrap(err, "failed to commit Genesis states")
	}
	return nil
}

// getFromWorkingSets returns (workingset, true) if it exists in a cache, otherwise generates new workingset and return (ws, false)
func (sf *factory) getFromWorkingSets(key hash.Hash256) (WorkingSet, bool, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	if data, ok := sf.workingsets.Get(key); ok {
		if ws, ok := data.(WorkingSet); ok {
			// if it is already validated, return workingset
			return ws, true, nil
		}
		return nil, false, errors.New("type assertion failed to be WorkingSet")
	}
	ws, err := newWorkingSet(sf.currentChainHeight+1, sf.dao, sf.rootHash(), sf.saveHistory)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to obtain working set from state factory")
	}
	return ws, false, nil
}

func (sf *factory) putIntoWorkingSets(key hash.Hash256, ws WorkingSet) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	sf.workingsets.Add(key, ws)
	return
}
