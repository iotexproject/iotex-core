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
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

const (
	// AccountKVNamespace is the bucket name for account
	AccountKVNamespace = "Account"
	// ArchiveNamespacePrefix is the prefix of the buckets storing history data
	ArchiveNamespacePrefix = "Archive"
	// CurrentHeightKey indicates the key of current factory height in underlying DB
	CurrentHeightKey = "currentHeight"
	// ArchiveTrieNamespace is the bucket for the latest state view
	ArchiveTrieNamespace = "AccountTrie"
	// ArchiveTrieRootKey indicates the key of accountTrie root hash in underlying DB
	ArchiveTrieRootKey = "archiveTrieRoot"
)

var (
	// ErrNotSupported is the error that the statedb is not for archive mode
	ErrNotSupported = errors.New("not supported")
	// ErrNoArchiveData is the error that the node have no archive data
	ErrNoArchiveData = errors.New("no archive data")

	_dbBatchSizelMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_db_batch_size",
			Help: "DB batch size",
		},
		[]string{},
	)

	//DefaultConfig is the default config for state factory
	DefaultConfig = Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.Default,
	}
)

func init() {
	prometheus.MustRegister(_dbBatchSizelMtc)
}

type (
	// Factory defines an interface for managing states
	Factory interface {
		lifecycle.StartStopper
		protocol.StateReader
		Register(protocol.Protocol) error
		Validate(context.Context, *block.Block) error
		// NewBlockBuilder creates block builder
		NewBlockBuilder(context.Context, actpool.ActPool, func(action.Envelope) (*action.SealedEnvelope, error)) (*block.Builder, error)
		PutBlock(context.Context, *block.Block) error
		WorkingSet(context.Context) (protocol.StateManager, error)
		WorkingSetAtHeight(context.Context, uint64, ...*action.SealedEnvelope) (protocol.StateManager, error)
		OngoingBlockHeight() uint64
		PendingBlockHeader(uint64) (*block.Header, error)
		PutBlockHeader(*block.Header)
		CancelBlock(uint64)
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle                lifecycle.Lifecycle
		mutex                    sync.RWMutex
		cfg                      Config
		registry                 *protocol.Registry
		currentChainHeight       uint64
		saveHistory              bool
		twoLayerTrie             trie.TwoLayerTrie // global state trie, this is a read only trie
		dao                      db.KVStore        // the underlying DB for account/contract storage
		timerFactory             *prometheustimer.TimerFactory
		chamber                  WorkingSetChamber
		protocolView             protocol.View
		skipBlockValidationOnPut bool
		ps                       *patchStore
	}

	// Config contains the config for factory
	Config struct {
		Chain   blockchain.Config
		Genesis genesis.Genesis
	}
)

// GenerateConfig generates the factory config
func GenerateConfig(chain blockchain.Config, g genesis.Genesis) Config {
	return Config{
		Chain:   chain,
		Genesis: g,
	}
}

// Option sets Factory construction parameter
type Option func(*factory, *Config) error

// RegistryOption sets the registry in state db
func RegistryOption(reg *protocol.Registry) Option {
	return func(sf *factory, cfg *Config) error {
		sf.registry = reg
		return nil
	}
}

// SkipBlockValidationOption skips block validation on PutBlock
func SkipBlockValidationOption() Option {
	return func(sf *factory, cfg *Config) error {
		sf.skipBlockValidationOnPut = true
		return nil
	}
}

// DefaultTriePatchOption loads patchs
func DefaultTriePatchOption() Option {
	return func(sf *factory, cfg *Config) (err error) {
		sf.ps, err = newPatchStore(cfg.Chain.TrieDBPatchFile)
		return
	}
}

// NewFactory creates a new state factory
func NewFactory(cfg Config, dao db.KVStore, opts ...Option) (Factory, error) {
	sf := &factory{
		cfg:                cfg,
		currentChainHeight: 0,
		registry:           protocol.NewRegistry(),
		saveHistory:        cfg.Chain.EnableArchiveMode,
		protocolView:       protocol.View{},
		chamber:            newWorkingsetChamber(int(cfg.Chain.WorkingSetCacheSize)),
		dao:                dao,
	}

	for _, opt := range opts {
		if err := opt(sf, &cfg); err != nil {
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
	sf.timerFactory = timerFactory

	return sf, nil
}

func (sf *factory) Start(ctx context.Context) error {
	ctx = protocol.WithRegistry(ctx, sf.registry)
	err := sf.dao.Start(ctx)
	if err != nil {
		return err
	}
	if sf.twoLayerTrie, err = newTwoLayerTrie(ArchiveTrieNamespace, sf.dao, ArchiveTrieRootKey, true); err != nil {
		return errors.Wrap(err, "failed to generate accountTrie from config")
	}
	if err := sf.twoLayerTrie.Start(ctx); err != nil {
		return err
	}
	// check factory height
	// TODO: move current height from account kv namespace to some other namespace
	h, err := sf.dao.Get(AccountKVNamespace, []byte(CurrentHeightKey))
	switch errors.Cause(err) {
	case nil:
		sf.currentChainHeight = byteutil.BytesToUint64(h)
		// start all protocols
		if sf.protocolView, err = sf.registry.StartAll(ctx, sf); err != nil {
			return err
		}
	case db.ErrNotExist:
		if err = sf.dao.Put(AccountKVNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)); err != nil {
			return errors.Wrap(err, "failed to init factory's height")
		}
		// start all protocols
		if sf.protocolView, err = sf.registry.StartAll(ctx, sf); err != nil {
			return err
		}
		ctx = protocol.WithBlockCtx(
			ctx,
			protocol.BlockCtx{
				BlockHeight:    0,
				BlockTimeStamp: time.Unix(sf.cfg.Genesis.Timestamp, 0),
				Producer:       sf.cfg.Chain.ProducerAddress(),
				GasLimit:       sf.cfg.Genesis.BlockGasLimitByHeight(0),
			})
		ctx = protocol.WithFeatureCtx(ctx)
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
	sf.chamber.Clear()
	return sf.lifecycle.OnStop(ctx)
}

// Height returns factory's height
func (sf *factory) Height() (uint64, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	height, err := sf.dao.Get(AccountKVNamespace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

func (sf *factory) OngoingBlockHeight() uint64 {
	return sf.chamber.OngoingBlockHeight()
}

func (sf *factory) PendingBlockHeader(height uint64) (*block.Header, error) {
	if h := sf.chamber.GetBlockHeader(height); h != nil {
		return h, nil
	}
	return nil, errors.Errorf("pending block %d not exist", height)
}

func (sf *factory) PutBlockHeader(header *block.Header) {
	sf.chamber.PutBlockHeader(header)
}

func (sf *factory) CancelBlock(height uint64) {
	sf.chamber.AbandonWorkingSets(height)
}

func (sf *factory) newWorkingSet(ctx context.Context, height uint64) (*workingSet, error) {
	span := tracer.SpanFromContext(ctx)
	span.AddEvent("factory.newWorkingSet")
	defer span.End()

	g := genesis.MustExtractGenesisContext(ctx)
	flusher, err := db.NewKVStoreFlusher(
		sf.dao,
		batch.NewCachedBatch(),
		sf.flusherOptions(!g.IsEaster(height))...,
	)
	if err != nil {
		return nil, err
	}
	store, err := newFactoryWorkingSetStore(sf.protocolView, flusher)
	if err != nil {
		return nil, err
	}
	var parent *workingSet
	if height > 0 {
		parent = sf.chamber.GetWorkingSet(height - 1)
	}
	return sf.createSfWorkingSet(ctx, height, store, parent)
}

func (sf *factory) newWorkingSetAtHeight(ctx context.Context, height uint64) (*workingSet, error) {
	span := tracer.SpanFromContext(ctx)
	span.AddEvent("factory.newWorkingSet")
	defer span.End()

	g := genesis.MustExtractGenesisContext(ctx)
	flusher, err := db.NewKVStoreFlusher(
		sf.dao,
		batch.NewCachedBatch(),
		sf.flusherOptions(!g.IsEaster(height))...,
	)
	if err != nil {
		return nil, err
	}
	store, err := newFactoryWorkingSetStoreAtHeight(sf.protocolView, flusher, height)
	if err != nil {
		return nil, err
	}
	return sf.createSfWorkingSet(ctx, height, store, nil)
}

func (sf *factory) createSfWorkingSet(ctx context.Context, height uint64, store workingSetStore, parent *workingSet) (*workingSet, error) {
	if err := store.Start(ctx); err != nil {
		return nil, err
	}
	for _, p := range sf.ps.Get(height) {
		if p.Type == _Delete {
			if err := store.Delete(p.Namespace, p.Key); err != nil {
				return nil, err
			}
		} else {
			if err := store.Put(p.Namespace, p.Key, p.Value); err != nil {
				return nil, err
			}
		}
	}
	return newWorkingSet(height, store, parent), nil
}

func (sf *factory) flusherOptions(preEaster bool) []db.KVStoreFlusherOption {
	opts := []db.KVStoreFlusherOption{
		db.SerializeFilterOption(func(wi *batch.WriteInfo) bool {
			if wi.Namespace() == ArchiveTrieNamespace {
				return true
			}
			if wi.Namespace() != evm.CodeKVNameSpace && wi.Namespace() != staking.CandsMapNS {
				return false
			}
			return preEaster
		}),
		db.SerializeOption(func(wi *batch.WriteInfo) []byte {
			if preEaster {
				return wi.SerializeWithoutWriteType()
			}
			return wi.Serialize()
		}),
	}
	if sf.saveHistory {
		opts = append(opts, db.FlushTranslateOption(func(wi *batch.WriteInfo) *batch.WriteInfo {
			if wi.WriteType() == batch.Delete && wi.Namespace() == ArchiveTrieNamespace {
				return nil
			}
			return wi
		}))
	}

	return opts
}

func (sf *factory) Register(p protocol.Protocol) error {
	return p.Register(sf.registry)
}

func (sf *factory) Validate(ctx context.Context, blk *block.Block) error {
	ctx = protocol.WithRegistry(ctx, sf.registry)
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	ws, isExist, err := sf.getFromWorkingSets(ctx, key)
	if err != nil {
		return err
	}
	if !isExist {
		if err := ws.ValidateBlock(ctx, blk); err != nil {
			return errors.Wrap(err, "failed to validate block with workingset in factory")
		}
		sf.chamber.PutWorkingSet(key, ws)
	}
	receipts, err := ws.Receipts()
	if err != nil {
		return err
	}
	blk.Receipts = receipts
	return nil
}

// NewBlockBuilder returns block builder which hasn't been signed yet
func (sf *factory) NewBlockBuilder(
	ctx context.Context,
	ap actpool.ActPool,
	sign func(action.Envelope) (*action.SealedEnvelope, error),
) (*block.Builder, error) {
	sf.mutex.Lock()
	ctx = protocol.WithRegistry(ctx, sf.registry)
	ws, err := sf.newWorkingSet(ctx, sf.chamber.OngoingBlockHeight()+1)
	sf.mutex.Unlock()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to obtain working set from state factory")
	}
	postSystemActions := make([]*action.SealedEnvelope, 0)
	unsignedSystemActions, err := ws.generateSystemActions(ctx)
	if err != nil {
		return nil, err
	}
	for _, elp := range unsignedSystemActions {
		se, err := sign(elp)
		if err != nil {
			return nil, err
		}
		postSystemActions = append(postSystemActions, se)
	}
	blkBuilder, err := ws.CreateBuilder(ctx, ap, postSystemActions, sf.cfg.Chain.AllowedBlockGasResidue)
	if err != nil {
		return nil, err
	}

	blkCtx := protocol.MustGetBlockCtx(ctx)
	key := generateWorkingSetCacheKey(blkBuilder.GetCurrentBlockHeader(), blkCtx.Producer.String())
	sf.chamber.PutWorkingSet(key, ws)
	return blkBuilder, nil
}

func (sf *factory) WorkingSet(ctx context.Context) (protocol.StateManager, error) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return sf.newWorkingSet(ctx, sf.currentChainHeight+1)
}

func (sf *factory) WorkingSetAtHeight(ctx context.Context, height uint64, preacts ...*action.SealedEnvelope) (protocol.StateManager, error) {
	if !sf.saveHistory {
		return nil, ErrNoArchiveData
	}
	sf.mutex.Lock()
	if height > sf.currentChainHeight {
		sf.mutex.Unlock()
		return nil, errors.Errorf("query height %d is higher than tip height %d", height, sf.currentChainHeight)
	}
	ws, err := sf.newWorkingSetAtHeight(ctx, height)
	sf.mutex.Unlock()
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain working set from state factory")
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
func (sf *factory) PutBlock(ctx context.Context, blk *block.Block) (err error) {
	timer := sf.timerFactory.NewTimer("Commit")
	var (
		ws      *workingSet
		isExist bool
	)
	defer func() {
		timer.End()
		if err != nil {
			// abandon current workingset, and all pending workingsets beyond current height
			ws.abandon()
			sf.chamber.AbandonWorkingSets(ws.height)
		}
	}()
	producer := blk.PublicKey().Address()
	if producer == nil {
		return errors.New("failed to get address")
	}
	ctx = protocol.WithRegistry(ctx, sf.registry)
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	ws, isExist, err = sf.getFromWorkingSets(ctx, key)
	if err != nil {
		return
	}
	if err = ws.verifyParent(); err != nil {
		return
	}
	ws.detachParent()
	if !isExist {
		// regenerate workingset
		if !sf.skipBlockValidationOnPut {
			err = ws.ValidateBlock(ctx, blk)
		} else {
			err = ws.Process(ctx, blk.RunnableActions().Actions())
		}
		if err != nil {
			log.L().Error("Failed to update state.", zap.Error(err))
			return
		}
	}
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	receipts, err := ws.Receipts()
	if err != nil {
		return
	}
	blk.Receipts = receipts
	h, _ := ws.Height()
	if sf.currentChainHeight+1 != h {
		// another working set with correct version already committed, do nothing
		return fmt.Errorf(
			"current state height %d + 1 doesn't match working set height %d",
			sf.currentChainHeight, h,
		)
	}

	if err = ws.Commit(ctx); err != nil {
		return
	}
	rh, err := sf.dao.Get(ArchiveTrieNamespace, []byte(ArchiveTrieRootKey))
	if err != nil {
		return
	}
	if err = sf.twoLayerTrie.SetRootHash(rh); err != nil {
		return
	}
	sf.currentChainHeight = h
	return nil
}

// State returns a confirmed state in the state factory
func (sf *factory) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	cfg, err := processOptions(opts...)
	if err != nil {
		return 0, err
	}
	if cfg.Keys != nil {
		return 0, errors.Wrap(ErrNotSupported, "Read state with keys option has not been implemented yet")
	}
	value, err := sf.dao.Get(cfg.Namespace, cfg.Key)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return sf.currentChainHeight, errors.Wrapf(state.ErrStateNotExist, "failed to get state of ns = %x and key = %x", cfg.Namespace, cfg.Key)
		}
		return sf.currentChainHeight, err
	}

	return sf.currentChainHeight, state.Deserialize(s, value)
}

// States returns a set states in the state factory
func (sf *factory) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	cfg, err := processOptions(opts...)
	if err != nil {
		return 0, nil, err
	}
	if cfg.Key != nil {
		return sf.currentChainHeight, nil, errors.Wrap(ErrNotSupported, "Read states with key option has not been implemented yet")
	}
	keys, values, err := readStatesFromTLT(sf.twoLayerTrie, cfg.Namespace, cfg.Keys)
	if err != nil {
		return 0, nil, err
	}
	iter, err := state.NewIterator(keys, values)
	if err != nil {
		return 0, nil, err
	}
	return sf.currentChainHeight, iter, nil
}

// ReadView reads the view
func (sf *factory) ReadView(name string) (interface{}, error) {
	return sf.protocolView.Read(name)
}

//======================================
// private trie constructor functions
//======================================

func (sf *factory) rootHash() ([]byte, error) {
	return sf.twoLayerTrie.RootHash()
}

func namespaceKey(ns string) []byte {
	h := hash.Hash160b([]byte(ns))
	return h[:]
}

func toLegacyKey(input []byte) []byte {
	key := hash.Hash160b(input)
	return key[:]
}

func legacyKeyLen() int {
	return 20
}

func (sf *factory) createGenesisStates(ctx context.Context) error {
	ws, err := sf.newWorkingSet(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "failed to obtain working set from state factory")
	}
	// add Genesis states
	if err := ws.CreateGenesisStates(ctx); err != nil {
		return err
	}

	return ws.Commit(ctx)
}

// getFromWorkingSets returns (workingset, true) if it exists in a cache, otherwise generates new workingset and return (ws, false)
func (sf *factory) getFromWorkingSets(ctx context.Context, key hash.Hash256) (*workingSet, bool, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	if ws := sf.chamber.GetWorkingSet(key); ws != nil {
		// if it is already validated, return workingset
		return ws, true, nil
	}
	ws, err := sf.newWorkingSet(ctx, sf.currentChainHeight+1)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to obtain working set from state factory")
	}
	return ws, false, nil
}
