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
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// stateDB implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
type stateDB struct {
	mutex              sync.RWMutex
	currentChainHeight uint64
	saveHistory        bool
	cfg                config.Config
	dao                db.KVStore // the underlying DB for account/contract storage
	timerFactory       *prometheustimer.TimerFactory
	workingsets        *lru.Cache // lru cache for workingsets
}

// StateDBOption sets stateDB construction parameter
type StateDBOption func(*stateDB, config.Config) error

// PrecreatedStateDBOption uses pre-created state db
func PrecreatedStateDBOption(kv db.KVStore) StateDBOption {
	return func(sdb *stateDB, cfg config.Config) error {
		if kv == nil {
			return errors.New("Invalid state db")
		}
		sdb.dao = kv
		return nil
	}
}

// DefaultStateDBOption creates default state db from config
func DefaultStateDBOption() StateDBOption {
	return func(sdb *stateDB, cfg config.Config) error {
		dbPath := cfg.Chain.TrieDBPath
		if len(dbPath) == 0 {
			return errors.New("Invalid empty trie db path")
		}
		cfg.DB.DbPath = dbPath // TODO: remove this after moving TrieDBPath from cfg.Chain to cfg.DB
		sdb.dao = db.NewBoltDB(cfg.DB)
		sdb.saveHistory = cfg.Chain.EnableHistoryStateDB
		return nil
	}
}

// InMemStateDBOption creates in memory state db
func InMemStateDBOption() StateDBOption {
	return func(sdb *stateDB, cfg config.Config) error {
		sdb.dao = db.NewMemKVStore()
		return nil
	}
}

// NewStateDB creates a new state db
func NewStateDB(cfg config.Config, opts ...StateDBOption) (Factory, error) {
	sdb := stateDB{
		cfg:                cfg,
		currentChainHeight: 0,
	}
	for _, opt := range opts {
		if err := opt(&sdb, cfg); err != nil {
			log.S().Errorf("Failed to execute state factory creation option %p: %v", opt, err)
			return nil, err
		}
	}
	timerFactory, err := prometheustimer.New(
		"iotex_statefactory_perf",
		"Performance of state factory module",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(cfg.Chain.ID), 10)},
	)
	if err != nil {
		log.L().Error("Failed to generate prometheus timer factory.", zap.Error(err))
	}
	sdb.timerFactory = timerFactory
	if sdb.workingsets, err = lru.New(int(cfg.Chain.WorkingSetCacheSize)); err != nil {
		return nil, errors.Wrap(err, "failed to generate lru cache for workingsets")
	}
	return &sdb, nil
}

func (sdb *stateDB) Start(ctx context.Context) error {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	if err := sdb.dao.Start(ctx); err != nil {
		return err
	}
	// check factory height
	h, err := sdb.dao.Get(protocol.AccountNameSpace, []byte(CurrentHeightKey))
	switch errors.Cause(err) {
	case nil:
		sdb.currentChainHeight = byteutil.BytesToUint64(h)
		break
	case db.ErrNotExist:
		// init the state factory
		if err = sdb.createGenesisStates(ctx); err != nil {
			return errors.Wrap(err, "failed to create genesis states")
		}
		if err = sdb.dao.Put(protocol.AccountNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)); err != nil {
			return errors.Wrap(err, "failed to init statedb's height")
		}
	default:
		return err
	}

	return nil
}

func (sdb *stateDB) Stop(ctx context.Context) error {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	sdb.workingsets.Purge()
	return sdb.dao.Stop(ctx)
}

// Height returns factory's height
func (sdb *stateDB) Height() (uint64, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	height, err := sdb.dao.Get(protocol.AccountNameSpace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

func (sdb *stateDB) NewWorkingSet() (WorkingSet, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()

	return newStateTX(sdb.currentChainHeight+1, sdb.dao, sdb.saveHistory), nil
}

func (sdb *stateDB) Validate(ctx context.Context, blk *block.Block) error {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	if data, ok := sdb.workingsets.Get(key); ok {
		if _, ok := data.(WorkingSet); !ok {
			return errors.New("type assertion failed to be WorkingSet")
		}
		// if already validated, return nil
		return nil
	}
	ws := newStateTX(sdb.currentChainHeight+1, sdb.dao, sdb.saveHistory)

	if err := validateWithWorkingset(ctx, ws, blk); err != nil {
		return errors.Wrap(err, "failed to validate block with workingset in statedb")
	}

	sdb.workingsets.Add(key, ws)
	return nil
}

// NewBlockBuilder returns block builder which hasn't been signed yet
func (sdb *stateDB) NewBlockBuilder(
	ctx context.Context,
	actionMap map[string][]action.SealedEnvelope,
	postSystemActions []action.SealedEnvelope,
) (*block.Builder, error) {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	ws := newStateTX(sdb.currentChainHeight+1, sdb.dao, sdb.saveHistory)
	blkBuilder, err := createBuilderWithWorkingset(ctx, ws, actionMap, postSystemActions, sdb.cfg.Chain.AllowedBlockGasResidue)
	if err != nil {
		return nil, err
	}

	blkCtx := protocol.MustGetBlockCtx(ctx)
	key := generateWorkingSetCacheKey(blkBuilder.GetCurrentBlockHeader(), blkCtx.Producer.String())
	sdb.workingsets.Add(key, ws)
	return blkBuilder, nil
}

// SimulateExecution simulates a running of smart contract operation, this is done off the network since it does not
// cause any state change
func (sdb *stateDB) SimulateExecution(
	ctx context.Context,
	caller address.Address,
	ex *action.Execution,
	getBlockHash evm.GetBlockHash,
) ([]byte, *action.Receipt, error) {
	sdb.mutex.Lock()
	ws := newStateTX(sdb.currentChainHeight+1, sdb.dao, sdb.saveHistory)
	sdb.mutex.Unlock()

	return simulateExecution(ctx, ws, caller, ex, getBlockHash)
}

// Commit persists all changes in RunActions() into the DB
func (sdb *stateDB) Commit(ctx context.Context, blk *block.Block) error {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	timer := sdb.timerFactory.NewTimer("Commit")
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
	data, ok := sdb.workingsets.Get(key)
	var ws WorkingSet
	if ok {
		if ws, ok = data.(WorkingSet); !ok {
			return errors.New("type assertion failed to be WorkingSet")
		}
	} else {
		// regenerate the workingset
		ws = newStateTX(sdb.currentChainHeight+1, sdb.dao, sdb.saveHistory)
		_, ws, err = runActions(ctx, ws, blk.RunnableActions().Actions())
		if err != nil {
			log.L().Panic("Failed to update state.", zap.Error(err))
			return err
		}
	}
	if sdb.currentChainHeight+1 != ws.Version() {
		// another working set with correct version already committed, do nothing
		return fmt.Errorf(
			"current state height %d + 1 doesn't match working set version %d",
			sdb.currentChainHeight,
			ws.Version(),
		)
	}

	return sdb.commit(ws)
}

// State returns a confirmed state in the state factory
func (sdb *stateDB) State(addr hash.Hash160, state interface{}, opts ...protocol.StateOption) error {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()

	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return err
	}

	ns := protocol.AccountNameSpace
	if cfg.Namespace != "" {
		ns = cfg.Namespace
	}

	return sdb.state(ns, addr, state)
}

// DeleteWorkingSet returns true if it remove ws from workingsets cache successfully
func (sdb *stateDB) DeleteWorkingSet(blk *block.Block) error {
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	sdb.workingsets.Remove(key)
	return nil
}

// StateAtHeight returns a confirmed state in the state factory
func (sdb *stateDB) StateAtHeight(height uint64, addr hash.Hash160, state interface{}) error {
	return ErrNotSupported
}

//======================================
// private trie constructor functions
//======================================

func (sdb *stateDB) state(ns string, addr hash.Hash160, s interface{}) error {
	data, err := sdb.dao.Get(ns, addr[:])
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return errors.Wrapf(state.ErrStateNotExist, "state of %x doesn't exist", addr)
		}
		return errors.Wrapf(err, "error when getting the state of %x", addr)
	}
	if err := state.Deserialize(s, data); err != nil {
		return errors.Wrapf(err, "error when deserializing state data into %T", s)
	}
	return nil
}

func (sdb *stateDB) commit(ws WorkingSet) error {
	if err := ws.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	// Update chain height
	height, err := ws.Height()
	if err != nil {
		return errors.Wrap(err, "failed to get working set height")
	}
	sdb.currentChainHeight = height

	return nil
}

// Initialize initializes the state db
func (sdb *stateDB) createGenesisStates(ctx context.Context) error {
	ws := newStateTX(0, sdb.dao, sdb.saveHistory)
	if err := createGenesisStates(ctx, ws); err != nil {
		return err
	}

	return sdb.commit(ws)
}
