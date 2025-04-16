// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

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
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/v2/state"
)

type (
	// daoRetrofitter represents the DAO-related methods to accommodate archive-mode
	daoRetrofitter interface {
		lifecycle.StartStopper
		atHeight(uint64) db.KVStore
		getHeight() (uint64, error)
		putHeight(uint64) error
	}
	// stateDB implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	stateDB struct {
		mutex                    sync.RWMutex
		currentChainHeight       uint64
		cfg                      Config
		registry                 *protocol.Registry
		dao                      daoRetrofitter
		timerFactory             *prometheustimer.TimerFactory
		workingsets              cache.LRUCache // lru cache for workingsets
		protocolView             *protocol.Views
		skipBlockValidationOnPut bool
		ps                       *patchStore
	}
)

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
		protocolView:       &protocol.Views{},
		workingsets:        cache.NewThreadSafeLruCache(int(cfg.Chain.WorkingSetCacheSize)),
	}
	for _, opt := range opts {
		if err := opt(&sdb, &cfg); err != nil {
			log.S().Errorf("Failed to execute state factory creation option %p: %v", opt, err)
			return nil, err
		}
	}
	sdb.dao = newDaoRetrofitter(dao)
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
	h, err := sdb.dao.getHeight()
	switch errors.Cause(err) {
	case nil:
		sdb.currentChainHeight = h
		// start all protocols
		if sdb.protocolView, err = sdb.registry.StartAll(ctx, sdb); err != nil {
			return err
		}
	case db.ErrNotExist:
		sdb.currentChainHeight = 0
		if err = sdb.dao.putHeight(0); err != nil {
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
				GasLimit:       sdb.cfg.Genesis.BlockGasLimitByHeight(0),
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
	return sdb.dao.getHeight()
}

func (sdb *stateDB) newReadOnlyWorkingSet(ctx context.Context, height uint64) (*workingSet, error) {
	return sdb.newWorkingSetWithKVStore(ctx, height, &readOnlyKV{sdb.dao.atHeight(height)})
}

func (sdb *stateDB) newWorkingSet(ctx context.Context, height uint64) (*workingSet, error) {
	return sdb.newWorkingSetWithKVStore(ctx, height, sdb.dao.atHeight(height))
}

func (sdb *stateDB) newWorkingSetWithKVStore(ctx context.Context, height uint64, kvstore db.KVStore) (*workingSet, error) {
	store, err := sdb.createWorkingSetStore(ctx, height, kvstore)
	if err != nil {
		return nil, err
	}
	if err := store.Start(ctx); err != nil {
		return nil, err
	}
	views := sdb.protocolView.Clone()
	if err := views.Commit(ctx, sdb); err != nil {
		return nil, err
	}
	return newWorkingSet(height, views, store, sdb), nil
}

func (sdb *stateDB) CreateWorkingSetStore(ctx context.Context, height uint64, kvstore db.KVStore) (workingSetStore, error) {
	return sdb.createWorkingSetStore(ctx, height, kvstore)
}

func (sdb *stateDB) createWorkingSetStore(ctx context.Context, height uint64, kvstore db.KVStore) (workingSetStore, error) {
	g := genesis.MustExtractGenesisContext(ctx)
	flusher, err := db.NewKVStoreFlusher(
		kvstore,
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
	return newStateDBWorkingSetStore(flusher, g.IsNewfoundland(height)), nil
}

func (sdb *stateDB) Register(p protocol.Protocol) error {
	return p.Register(sdb.registry)
}

func (sdb *stateDB) Validate(ctx context.Context, blk *block.Block) error {
	ctx = protocol.WithRegistry(ctx, sdb.registry)
	blkHash := blk.HashBlock()
	ws, isExist, err := sdb.getFromWorkingSets(ctx, blkHash)
	if err != nil {
		return err
	}
	if !isExist {
		if err = ws.ValidateBlock(ctx, blk); err != nil {
			return errors.Wrap(err, "failed to validate block with workingset in statedb")
		}
		sdb.workingsets.Add(blkHash, ws)
	}
	receipts, err := ws.Receipts()
	if err != nil {
		return err
	}
	blk.Receipts = receipts
	return nil
}

// Mint mints a block
func (sdb *stateDB) Mint(
	ctx context.Context,
	ap actpool.ActPool,
	pk crypto.PrivateKey,
) (*block.Block, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	expectedBlockHeight := bcCtx.Tip.Height + 1
	ctx = protocol.WithRegistry(ctx, sdb.registry)
	var (
		ws  *workingSet
		err error
	)
	sdb.mutex.RLock()
	currHeight := sdb.currentChainHeight
	sdb.mutex.RUnlock()
	switch {
	case currHeight+1 < expectedBlockHeight:
		parent, ok := sdb.workingsets.Get(bcCtx.Tip.Hash)
		if !ok {
			return nil, errors.Wrapf(ErrNotSupported, "failed to create block at height %d, current height is %d", expectedBlockHeight, sdb.currentChainHeight)
		}
		ws, err = parent.(*workingSet).NewWorkingSet(ctx)
	case currHeight+1 > expectedBlockHeight:
		return nil, errors.Wrapf(ErrNotSupported, "cannot create block at height %d, current height is %d", expectedBlockHeight, sdb.currentChainHeight)
	default:
		ws, err = sdb.newWorkingSet(ctx, currHeight+1)
	}
	if err != nil {
		return nil, err
	}
	sign := func(elp action.Envelope) (*action.SealedEnvelope, error) {
		return action.Sign(elp, pk)
	}
	blkBuilder, err := ws.CreateBuilder(ctx, ap, sign, sdb.cfg.Chain.AllowedBlockGasResidue)
	if err != nil {
		return nil, err
	}

	blk, err := blkBuilder.SignAndBuild(pk)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block builder at new block height %d", expectedBlockHeight)
	}
	sdb.workingsets.Add(blk.HashBlock(), ws)
	return &blk, nil
}

func (sdb *stateDB) WorkingSet(ctx context.Context) (protocol.StateManager, error) {
	sdb.mutex.RLock()
	height := sdb.currentChainHeight
	sdb.mutex.RUnlock()
	return sdb.newReadOnlyWorkingSet(ctx, height+1)
}

func (sdb *stateDB) WorkingSetAtHeight(ctx context.Context, height uint64, preacts ...*action.SealedEnvelope) (protocol.StateManager, error) {
	ws, err := sdb.newReadOnlyWorkingSet(ctx, height)
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain working set from state db")
	}
	if len(preacts) == 0 {
		return ws, nil
	}
	// prepare workingset at height, and run acts
	ws.height++
	if err := ws.Process(ctx, preacts); err != nil {
		return nil, err
	}
	return ws, nil
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
	ctx = protocol.WithRegistry(ctx, sdb.registry)
	ws, isExist, err := sdb.getFromWorkingSets(ctx, blk.HashBlock())
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
	sdb.protocolView = ws.views
	sdb.currentChainHeight = h
	return nil
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
	return sdb.currentChainHeight, sdb.state(sdb.currentChainHeight, cfg.Namespace, cfg.Key, s)
}

// States returns a set of states in the state factory
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
	keys, values, err := readStates(sdb.dao.atHeight(sdb.currentChainHeight), cfg.Namespace, cfg.Keys)
	if err != nil {
		return 0, nil, err
	}
	iter, err := state.NewIterator(keys, values)
	if err != nil {
		return 0, nil, err
	}

	return sdb.currentChainHeight, iter, nil
}

// ReadView reads the view
func (sdb *stateDB) ReadView(name string) (protocol.View, error) {
	return sdb.protocolView.Read(name)
}

// StateReaderAt returns a state reader at a specific height
func (sdb *stateDB) StateReaderAt(blkHeight uint64, blkHash hash.Hash256) (protocol.StateReader, error) {
	sdb.mutex.RLock()
	curHeight := sdb.currentChainHeight
	sdb.mutex.RUnlock()
	if blkHeight == curHeight {
		return sdb, nil
	} else if blkHeight < curHeight {
		return nil, errors.Errorf("cannot read state at height %d, current height is %d", blkHeight, curHeight)
	}
	if data, ok := sdb.workingsets.Get(blkHash); ok {
		if ws, ok := data.(*workingSet); ok {
			return ws, nil
		}
		return nil, errors.New("type assertion failed to be WorkingSet")
	}
	return nil, errors.Errorf("failed to get workingset at %x", blkHash)
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

func (sdb *stateDB) state(h uint64, ns string, addr []byte, s interface{}) error {
	data, err := sdb.dao.atHeight(h).Get(ns, addr)
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

	if err := ws.Commit(ctx); err != nil {
		return err
	}
	sdb.protocolView = ws.views
	return nil
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
