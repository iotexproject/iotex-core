// Copyright (c) 2020 IoTeX Foundation
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
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/tracer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// stateDB implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
type stateDB struct {
	mutex                    sync.RWMutex
	currentChainHeight       uint64
	cfg                      Config
	registry                 *protocol.Registry
	dao                      db.KVStore // the underlying DB for account/contract storage
	timerFactory             *prometheustimer.TimerFactory
	workingsets              cache.LRUCache // lru cache for workingsets
	protocolView             protocol.View
	skipBlockValidationOnPut bool
	ps                       *patchStore
}

// StateDBOption sets stateDB construction parameter
type StateDBOption func(*stateDB, *Config) error

// DefaultPatchOption loads patchs
func DefaultPatchOption() StateDBOption {
	return func(sdb *stateDB, cfg *Config) (err error) {
		sdb.ps, err = newPatchStore(cfg.Chain.TrieDBPatchFile)
		return
	}
}

// RegistryStateDBOption sets the registry in state db
func RegistryStateDBOption(reg *protocol.Registry) StateDBOption {
	return func(sdb *stateDB, cfg *Config) error {
		sdb.registry = reg
		return nil
	}
}

// SkipBlockValidationStateDBOption skips block validation on PutBlock
func SkipBlockValidationStateDBOption() StateDBOption {
	return func(sdb *stateDB, cfg *Config) error {
		sdb.skipBlockValidationOnPut = true
		return nil
	}
}

// DisableWorkingSetCacheOption disable workingset cache
func DisableWorkingSetCacheOption() StateDBOption {
	return func(sdb *stateDB, cfg *Config) error {
		sdb.workingsets = cache.NewDummyLruCache()
		return nil
	}
}

// NewStateDB creates a new state db
func NewStateDB(cfg Config, dao db.KVStore, opts ...StateDBOption) (Factory, error) {
	sdb := stateDB{
		cfg:                cfg,
		currentChainHeight: 0,
		registry:           protocol.NewRegistry(),
		protocolView:       protocol.View{},
		workingsets:        cache.NewThreadSafeLruCache(int(cfg.Chain.WorkingSetCacheSize)),
		dao:                dao,
	}
	for _, opt := range opts {
		if err := opt(&sdb, &cfg); err != nil {
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
	return &sdb, nil
}

func (sdb *stateDB) Start(ctx context.Context) error {
	ctx = protocol.WithRegistry(ctx, sdb.registry)
	if err := sdb.dao.Start(ctx); err != nil {
		return err
	}
	// check factory height
	h, err := sdb.dao.Get(AccountKVNamespace, []byte(CurrentHeightKey))
	switch errors.Cause(err) {
	case nil:
		sdb.currentChainHeight = byteutil.BytesToUint64(h)
		// start all protocols
		if sdb.protocolView, err = sdb.registry.StartAll(ctx, sdb); err != nil {
			return err
		}
	case db.ErrNotExist:
		sdb.currentChainHeight = 0
		if err = sdb.dao.Put(AccountKVNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)); err != nil {
			return errors.Wrap(err, "failed to init statedb's height")
		}
		// start all protocols
		if sdb.protocolView, err = sdb.registry.StartAll(ctx, sdb); err != nil {
			return err
		}
		ctx = protocol.WithBlockCtx(
			ctx,
			protocol.BlockCtx{
				BlockHeight:    0,
				BlockTimeStamp: time.Unix(sdb.cfg.Genesis.Timestamp, 0),
				Producer:       sdb.cfg.Chain.ProducerAddress(),
				GasLimit:       sdb.cfg.Genesis.BlockGasLimit,
			})
		ctx = protocol.WithFeatureCtx(ctx)
		// init the state factory
		if err = sdb.createGenesisStates(ctx); err != nil {
			return errors.Wrap(err, "failed to create genesis states")
		}
	default:
		return err
	}

	return nil
}

func (sdb *stateDB) Stop(ctx context.Context) error {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	sdb.workingsets.Clear()
	return sdb.dao.Stop(ctx)
}

// Height returns factory's height
func (sdb *stateDB) Height() (uint64, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	height, err := sdb.dao.Get(AccountKVNamespace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

func (sdb *stateDB) newWorkingSet(ctx context.Context, height uint64) (*workingSet, error) {
	g := genesis.MustExtractGenesisContext(ctx)
	flusher, err := db.NewKVStoreFlusher(
		sdb.dao,
		batch.NewCachedBatch(),
		sdb.flusherOptions(!g.IsEaster(height))...,
	)
	if err != nil {
		return nil, err
	}
	for _, p := range sdb.ps.Get(height) {
		if p.Type == _Delete {
			flusher.KVStoreWithBuffer().MustDelete(p.Namespace, p.Key)
		} else {
			flusher.KVStoreWithBuffer().MustPut(p.Namespace, p.Key, p.Value)
		}
	}
	store := newStateDBWorkingSetStore(sdb.protocolView, flusher, g.IsNewfoundland(height))
	if err := store.Start(ctx); err != nil {
		return nil, err
	}

	return newWorkingSet(height, store), nil
}

func (sdb *stateDB) Register(p protocol.Protocol) error {
	return p.Register(sdb.registry)
}

func (sdb *stateDB) Validate(ctx context.Context, blk *block.Block) error {
	ctx = protocol.WithRegistry(ctx, sdb.registry)
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	ws, isExist, err := sdb.getFromWorkingSets(ctx, key)
	if err != nil {
		return err
	}
	if !isExist {
		if err = ws.ValidateBlock(ctx, blk); err != nil {
			return errors.Wrap(err, "failed to validate block with workingset in statedb")
		}
		sdb.workingsets.Add(key, ws)
	}
	receipts, err := ws.Receipts()
	if err != nil {
		return err
	}
	blk.Receipts = receipts
	return nil
}

// NewBlockBuilder returns block builder which hasn't been signed yet
func (sdb *stateDB) NewBlockBuilder(
	ctx context.Context,
	ap actpool.ActPool,
	sign func(action.Envelope) (action.SealedEnvelope, error),
) (*block.Builder, error) {
	ctx = protocol.WithRegistry(ctx, sdb.registry)
	sdb.mutex.RLock()
	currHeight := sdb.currentChainHeight
	sdb.mutex.RUnlock()
	ws, err := sdb.newWorkingSet(ctx, currHeight+1)
	if err != nil {
		return nil, err
	}
	postSystemActions := make([]action.SealedEnvelope, 0)
	for _, p := range sdb.registry.All() {
		if psac, ok := p.(protocol.PostSystemActionsCreator); ok {
			elps, err := psac.CreatePostSystemActions(ctx, ws)
			if err != nil {
				return nil, err
			}
			for _, elp := range elps {
				se, err := sign(elp)
				if err != nil {
					return nil, err
				}
				postSystemActions = append(postSystemActions, se)
			}
		}
	}
	blkBuilder, err := ws.CreateBuilder(ctx, ap, postSystemActions, sdb.cfg.Chain.AllowedBlockGasResidue)
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
	ctx, span := tracer.NewSpan(ctx, "stateDB.SimulateExecution")
	defer span.End()

	sdb.mutex.RLock()
	currHeight := sdb.currentChainHeight
	sdb.mutex.RUnlock()
	ws, err := sdb.newWorkingSet(ctx, currHeight+1)
	if err != nil {
		return nil, nil, err
	}

	return evm.SimulateExecution(ctx, ws, caller, ex, getBlockHash)
}

// ReadContractStorage reads contract's storage
func (sdb *stateDB) ReadContractStorage(ctx context.Context, contract address.Address, key []byte) ([]byte, error) {
	sdb.mutex.RLock()
	currHeight := sdb.currentChainHeight
	sdb.mutex.RUnlock()
	ws, err := sdb.newWorkingSet(ctx, currHeight+1)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate working set from state db")
	}
	return evm.ReadContractStorage(ctx, ws, contract, key)
}

// PutBlock persists all changes in RunActions() into the DB
func (sdb *stateDB) PutBlock(ctx context.Context, blk *block.Block) error {
	sdb.mutex.Lock()
	timer := sdb.timerFactory.NewTimer("Commit")
	sdb.mutex.Unlock()
	defer timer.End()
	producer := blk.PublicKey().Address()
	if producer == nil {
		return errors.New("failed to get address")
	}
	g := genesis.MustExtractGenesisContext(ctx)
	ctx = protocol.WithBlockCtx(
		protocol.WithRegistry(ctx, sdb.registry),
		protocol.BlockCtx{
			BlockHeight:    blk.Height(),
			BlockTimeStamp: blk.Timestamp(),
			GasLimit:       g.BlockGasLimit,
			Producer:       producer,
		},
	)
	ctx = protocol.WithFeatureCtx(ctx)
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	ws, isExist, err := sdb.getFromWorkingSets(ctx, key)
	if err != nil {
		return err
	}
	if !isExist {
		if !sdb.skipBlockValidationOnPut {
			err = ws.ValidateBlock(ctx, blk)
		} else {
			err = ws.Process(ctx, blk.RunnableActions().Actions())
		}
		if err != nil {
			log.L().Error("Failed to update state.", zap.Error(err))
			return err
		}
	}
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	receipts, err := ws.Receipts()
	if err != nil {
		return err
	}
	blk.Receipts = receipts
	h, _ := ws.Height()
	if sdb.currentChainHeight+1 != h {
		// another working set with correct version already committed, do nothing
		return fmt.Errorf(
			"current state height %d + 1 doesn't match working set height %d",
			sdb.currentChainHeight, h,
		)
	}

	if err := ws.Commit(ctx); err != nil {
		return err
	}
	sdb.currentChainHeight = h
	return nil
}

func (sdb *stateDB) DeleteTipBlock(_ context.Context, _ *block.Block) error {
	return errors.Wrap(ErrNotSupported, "cannot delete tip block from state db")
}

// State returns a confirmed state in the state factory
func (sdb *stateDB) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return 0, err
	}
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	if cfg.Keys != nil {
		return 0, errors.Wrap(ErrNotSupported, "Read state with keys option has not been implemented yet")
	}
	return sdb.currentChainHeight, sdb.state(cfg.Namespace, cfg.Key, s)
}

// State returns a set of states in the state factory
func (sdb *stateDB) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return 0, nil, err
	}
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	if cfg.Key != nil {
		return sdb.currentChainHeight, nil, errors.Wrap(ErrNotSupported, "Read states with key option has not been implemented yet")
	}
	values, err := readStates(sdb.dao, cfg.Namespace, cfg.Keys)
	if err != nil {
		return 0, nil, err
	}

	return sdb.currentChainHeight, state.NewIterator(values), nil
}

// StateAtHeight returns a confirmed state at height -- archive mode
func (sdb *stateDB) StateAtHeight(height uint64, s interface{}, opts ...protocol.StateOption) error {
	return ErrNotSupported
}

// StatesAtHeight returns a set states in the state factory at height -- archive mode
func (sdb *stateDB) StatesAtHeight(height uint64, opts ...protocol.StateOption) (state.Iterator, error) {
	return nil, errors.Wrap(ErrNotSupported, "state db does not support archive mode")
}

// ReadView reads the view
func (sdb *stateDB) ReadView(name string) (interface{}, error) {
	return sdb.protocolView.Read(name)
}

//======================================
// private trie constructor functions
//======================================

func (sdb *stateDB) flusherOptions(preEaster bool) []db.KVStoreFlusherOption {
	opts := []db.KVStoreFlusherOption{
		db.SerializeOption(func(wi *batch.WriteInfo) []byte {
			if preEaster {
				return wi.SerializeWithoutWriteType()
			}
			return wi.Serialize()
		}),
	}
	if !preEaster {
		return opts
	}
	return append(
		opts,
		db.SerializeFilterOption(func(wi *batch.WriteInfo) bool {
			return wi.Namespace() == evm.CodeKVNameSpace || wi.Namespace() == staking.CandsMapNS
		}),
	)
}

func (sdb *stateDB) state(ns string, addr []byte, s interface{}) error {
	data, err := sdb.dao.Get(ns, addr)
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

func (sdb *stateDB) createGenesisStates(ctx context.Context) error {
	ws, err := sdb.newWorkingSet(ctx, 0)
	if err != nil {
		return err
	}
	if err := ws.CreateGenesisStates(ctx); err != nil {
		return err
	}

	return ws.Commit(ctx)
}

// getFromWorkingSets returns (workingset, true) if it exists in a cache, otherwise generates new workingset and return (ws, false)
func (sdb *stateDB) getFromWorkingSets(ctx context.Context, key hash.Hash256) (*workingSet, bool, error) {
	if data, ok := sdb.workingsets.Get(key); ok {
		if ws, ok := data.(*workingSet); ok {
			// if it is already validated, return workingset
			return ws, true, nil
		}
		return nil, false, errors.New("type assertion failed to be WorkingSet")
	}
	sdb.mutex.RLock()
	currHeight := sdb.currentChainHeight
	sdb.mutex.RUnlock()
	tx, err := sdb.newWorkingSet(ctx, currHeight+1)
	return tx, false, err
}
