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
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/tracer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
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

	dbBatchSizelMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_db_batch_size",
			Help: "DB batch size",
		},
		[]string{},
	)
)

func init() {
	prometheus.MustRegister(dbBatchSizelMtc)
}

type (
	// Factory defines an interface for managing states
	Factory interface {
		lifecycle.StartStopper
		protocol.StateReader
		Register(protocol.Protocol) error
		Validate(context.Context, *block.Block) error
		// NewBlockBuilder creates block builder
		NewBlockBuilder(context.Context, actpool.ActPool, func(action.Envelope) (action.SealedEnvelope, error)) (*block.Builder, error)
		SimulateExecution(context.Context, address.Address, *action.Execution, evm.GetBlockHash) ([]byte, *action.Receipt, error)
		ReadContractStorage(context.Context, address.Address, []byte) ([]byte, error)
		PutBlock(context.Context, *block.Block) error
		DeleteTipBlock(*block.Block) error
		StateAtHeight(uint64, interface{}, ...protocol.StateOption) error
		StatesAtHeight(uint64, ...protocol.StateOption) (state.Iterator, error)
	}

	// factory implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
	factory struct {
		lifecycle                lifecycle.Lifecycle
		mutex                    sync.RWMutex
		cfg                      config.Config
		registry                 *protocol.Registry
		currentChainHeight       uint64
		saveHistory              bool
		twoLayerTrie             trie.TwoLayerTrie // global state trie, this is a read only trie
		dao                      db.KVStore        // the underlying DB for account/contract storage
		timerFactory             *prometheustimer.TimerFactory
		workingsets              *cache.ThreadSafeLruCache // lru cache for workingsets
		protocolView             protocol.View
		skipBlockValidationOnPut bool
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

// RegistryOption sets the registry in state db
func RegistryOption(reg *protocol.Registry) Option {
	return func(sf *factory, cfg config.Config) error {
		sf.registry = reg
		return nil
	}
}

// SkipBlockValidationOption skips block validation on PutBlock
func SkipBlockValidationOption() Option {
	return func(sf *factory, cfg config.Config) error {
		sf.skipBlockValidationOnPut = true
		return nil
	}
}

func newTwoLayerTrie(ns string, dao db.KVStore, rootKey string, create bool) (trie.TwoLayerTrie, error) {
	dbForTrie, err := trie.NewKVStore(ns, dao)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create db for trie")
	}
	_, err = dbForTrie.Get([]byte(rootKey))
	switch errors.Cause(err) {
	case trie.ErrNotExist:
		if !create {
			return nil, err
		}
	case nil:
		break
	default:
		return nil, err
	}
	return mptrie.NewTwoLayerTrie(dbForTrie, rootKey), nil
}

// NewFactory creates a new state factory
func NewFactory(cfg config.Config, opts ...Option) (Factory, error) {
	sf := &factory{
		cfg:                cfg,
		currentChainHeight: 0,
		registry:           protocol.NewRegistry(),
		saveHistory:        cfg.Chain.EnableArchiveMode,
		protocolView:       protocol.View{},
		workingsets:        cache.NewThreadSafeLruCache(int(cfg.Chain.WorkingSetCacheSize)),
	}

	for _, opt := range opts {
		if err := opt(sf, cfg); err != nil {
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
				Producer:       sf.cfg.ProducerAddress(),
				GasLimit:       sf.cfg.Genesis.BlockGasLimit,
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
	sf.workingsets.Clear()
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

func (sf *factory) newWorkingSet(ctx context.Context, height uint64) (*workingSet, error) {
	span := tracer.SpanFromContext(ctx)
	span.AddEvent("factory.newWorkingSet")
	defer span.End()

	flusher, err := db.NewKVStoreFlusher(sf.dao, batch.NewCachedBatch(), sf.flusherOptions(ctx, height)...)
	if err != nil {
		return nil, err
	}
	tlt, err := newTwoLayerTrie(ArchiveTrieNamespace, flusher.KVStoreWithBuffer(), ArchiveTrieRootKey, true)
	if err != nil {
		return nil, err
	}
	if err := tlt.Start(ctx); err != nil {
		return nil, err
	}
	finalized := false
	trieRoots := make(map[int][]byte)

	return &workingSet{
		height:    height,
		finalized: false,
		dock:      protocol.NewDock(),
		getStateFunc: func(ns string, key []byte, s interface{}) error {
			value, err := readState(tlt, ns, key)
			if err != nil {
				return err
			}
			return state.Deserialize(s, value)
		},
		putStateFunc: func(ns string, key []byte, s interface{}) error {
			ss, err := state.Serialize(s)
			if err != nil {
				return errors.Wrapf(err, "failed to convert account %v to bytes", s)
			}
			flusher.KVStoreWithBuffer().MustPut(ns, key, ss)
			nsHash := hash.Hash160b([]byte(ns))

			return tlt.Upsert(nsHash[:], toLegacyKey(key), ss)
		},
		delStateFunc: func(ns string, key []byte) error {
			flusher.KVStoreWithBuffer().MustDelete(ns, key)
			nsHash := hash.Hash160b([]byte(ns))

			err := tlt.Delete(nsHash[:], toLegacyKey(key))
			if errors.Cause(err) == trie.ErrNotExist {
				return errors.Wrapf(state.ErrStateNotExist, "key %x doesn't exist in namespace %x", key, nsHash)
			}
			return err
		},
		statesFunc: func(ns string, keys [][]byte) ([][]byte, error) {
			values := [][]byte{}
			if keys == nil {
				iter, err := mptrie.NewLayerTwoLeafIterator(tlt, namespaceKey(ns), legacyKeyLen())
				if err != nil {
					return nil, err
				}
				for {
					_, value, err := iter.Next()
					if err == trie.ErrEndOfIterator {
						break
					}
					if err != nil {
						return nil, err
					}
					values = append(values, value)
				}
			} else {
				for _, key := range keys {
					value, err := readState(tlt, ns, key)
					switch errors.Cause(err) {
					case db.ErrNotExist, db.ErrBucketNotExist:
						values = append(values, nil)
					case nil:
						values = append(values, value)
					default:
						return nil, err
					}
				}
			}
			return values, nil
		},
		digestFunc: func() hash.Hash256 {
			return hash.Hash256b(flusher.SerializeQueue())
		},
		finalizeFunc: func(h uint64) error {
			if finalized {
				return errors.New("Cannot finalize a working set twice")
			}
			rootHash, err := tlt.RootHash()
			if err != nil {
				return err
			}
			finalized = true
			flusher.KVStoreWithBuffer().MustPut(AccountKVNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(h))
			flusher.KVStoreWithBuffer().MustPut(ArchiveTrieNamespace, []byte(ArchiveTrieRootKey), rootHash)
			// Persist the historical accountTrie's root hash
			flusher.KVStoreWithBuffer().MustPut(
				ArchiveTrieNamespace,
				[]byte(fmt.Sprintf("%s-%d", ArchiveTrieRootKey, h)),
				rootHash,
			)
			return nil
		},
		commitFunc: func(h uint64) error {
			dbBatchSizelMtc.WithLabelValues().Set(float64(flusher.KVStoreWithBuffer().Size()))
			if err := flusher.Flush(); err != nil {
				return errors.Wrap(err, "failed to Commit all changes to underlying DB in a batch")
			}
			rh, err := tlt.RootHash()
			if err != nil {
				return err
			}
			if err := sf.twoLayerTrie.SetRootHash(rh); err != nil {
				return err
			}
			sf.currentChainHeight = h
			return nil
		},
		readviewFunc: func(name string) (interface{}, error) {
			return sf.ReadView(name)
		},
		writeviewFunc: func(name string, v interface{}) error {
			return sf.protocolView.Write(name, v)
		},
		snapshotFunc: func() int {
			rh, err := tlt.RootHash()
			if err != nil {
				log.L().Panic("failed to do snapshot", zap.Error(err))
			}
			s := flusher.KVStoreWithBuffer().Snapshot()
			trieRoots[s] = rh
			return s
		},
		revertFunc: func(snapshot int) error {
			if err := flusher.KVStoreWithBuffer().Revert(snapshot); err != nil {
				return err
			}
			root, ok := trieRoots[snapshot]
			if !ok {
				// this should not happen, b/c we save the trie root on a successful return of Snapshot(), but check anyway
				return errors.Wrapf(trie.ErrInvalidTrie, "failed to get trie root for snapshot = %d", snapshot)
			}
			return tlt.SetRootHash(root[:])
		},
		dbFunc: func() db.KVStore {
			return flusher.KVStoreWithBuffer()
		},
	}, nil
}

func (sf *factory) flusherOptions(ctx context.Context, height uint64) []db.KVStoreFlusherOption {
	g := genesis.MustExtractGenesisContext(ctx)
	preEaster := !g.IsEaster(height)
	opts := []db.KVStoreFlusherOption{
		db.SerializeFilterOption(func(wi *batch.WriteInfo) bool {
			if wi.Namespace() == ArchiveTrieNamespace {
				return true
			}
			if wi.Namespace() != evm.CodeKVNameSpace {
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
		sf.putIntoWorkingSets(key, ws)
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
	sign func(action.Envelope) (action.SealedEnvelope, error),
) (*block.Builder, error) {
	sf.mutex.Lock()
	ctx = protocol.WithRegistry(ctx, sf.registry)
	ws, err := sf.newWorkingSet(ctx, sf.currentChainHeight+1)
	sf.mutex.Unlock()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to obtain working set from state factory")
	}
	postSystemActions := make([]action.SealedEnvelope, 0)
	for _, p := range sf.registry.All() {
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
	blkBuilder, err := ws.CreateBuilder(ctx, ap, postSystemActions, sf.cfg.Chain.AllowedBlockGasResidue)
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
	ctx, span := tracer.NewSpan(ctx, "factory.SimulateExecution")
	defer span.End()

	sf.mutex.Lock()
	ws, err := sf.newWorkingSet(ctx, sf.currentChainHeight+1)
	sf.mutex.Unlock()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to obtain working set from state factory")
	}

	return evm.SimulateExecution(ctx, ws, caller, ex, getBlockHash)
}

// ReadContractStorage reads contract's storage
func (sf *factory) ReadContractStorage(ctx context.Context, contract address.Address, key []byte) ([]byte, error) {
	sf.mutex.Lock()
	ws, err := sf.newWorkingSet(ctx, sf.currentChainHeight+1)
	sf.mutex.Unlock()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate working set from state factory")
	}
	return evm.ReadContractStorage(ctx, ws, contract, key)
}

// PutBlock persists all changes in RunActions() into the DB
func (sf *factory) PutBlock(ctx context.Context, blk *block.Block) error {
	sf.mutex.Lock()
	timer := sf.timerFactory.NewTimer("Commit")
	sf.mutex.Unlock()
	defer timer.End()
	producer := blk.PublicKey().Address()
	if producer == nil {
		return errors.New("failed to get address")
	}
	g := genesis.MustExtractGenesisContext(ctx)
	ctx = protocol.WithBlockCtx(
		protocol.WithRegistry(ctx, sf.registry),
		protocol.BlockCtx{
			BlockHeight:    blk.Height(),
			BlockTimeStamp: blk.Timestamp(),
			GasLimit:       g.BlockGasLimit,
			Producer:       producer,
		},
	)
	ctx = protocol.WithFeatureCtx(ctx)
	key := generateWorkingSetCacheKey(blk.Header, blk.Header.ProducerAddress())
	ws, isExist, err := sf.getFromWorkingSets(ctx, key)
	if err != nil {
		return err
	}
	if !isExist {
		// regenerate workingset
		if !sf.skipBlockValidationOnPut {
			err = ws.ValidateBlock(ctx, blk)
		} else {
			err = ws.Process(ctx, blk.RunnableActions().Actions())
		}
		if err != nil {
			log.L().Error("Failed to update state.", zap.Error(err))
			return err
		}
	}
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	receipts, err := ws.Receipts()
	if err != nil {
		return err
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

	return ws.Commit(ctx)
}

func (sf *factory) DeleteTipBlock(_ *block.Block) error {
	return errors.Wrap(ErrNotSupported, "cannot delete tip block from factory")
}

// StateAtHeight returns a confirmed state at height -- archive mode
func (sf *factory) StateAtHeight(height uint64, s interface{}, opts ...protocol.StateOption) error {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	cfg, err := processOptions(opts...)
	if err != nil {
		return err
	}
	if cfg.Keys != nil {
		return errors.Wrap(ErrNotSupported, "Read state with keys option has not been implemented yet")
	}
	if height > sf.currentChainHeight {
		return errors.Errorf("query height %d is higher than tip height %d", height, sf.currentChainHeight)
	}
	return sf.stateAtHeight(height, cfg.Namespace, cfg.Key, s)
}

// StatesAtHeight returns a set states in the state factory at height -- archive mode
func (sf *factory) StatesAtHeight(height uint64, opts ...protocol.StateOption) (state.Iterator, error) {
	sf.mutex.RLock()
	defer sf.mutex.RUnlock()
	if height > sf.currentChainHeight {
		return nil, errors.Errorf("query height %d is higher than tip height %d", height, sf.currentChainHeight)
	}
	return nil, errors.Wrap(ErrNotSupported, "Read historical states has not been implemented yet")
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

// State returns a set states in the state factory
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
	var values [][]byte
	if cfg.Keys == nil {
		_, values, err = sf.dao.Filter(cfg.Namespace, func(key, value []byte) bool { return true }, nil, nil)
		if err != nil {
			if errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == db.ErrBucketNotExist {
				return sf.currentChainHeight, nil, errors.Wrapf(state.ErrStateNotExist, "failed to get states of ns = %x", cfg.Namespace)
			}
			return sf.currentChainHeight, nil, err
		}
	} else {
		for _, key := range cfg.Keys {
			value, err := sf.dao.Get(cfg.Namespace, key)
			switch errors.Cause(err) {
			case db.ErrNotExist, db.ErrBucketNotExist:
				values = append(values, nil)
			case nil:
				values = append(values, value)
			default:
				return 0, nil, err
			}
		}
	}

	return sf.currentChainHeight, state.NewIterator(values), nil
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

func readState(tlt trie.TwoLayerTrie, ns string, key []byte) ([]byte, error) {
	ltKey := toLegacyKey(key)
	data, err := tlt.Get(namespaceKey(ns), ltKey)
	if err != nil {
		if errors.Cause(err) == trie.ErrNotExist {
			return nil, errors.Wrapf(state.ErrStateNotExist, "failed to get state of ns = %x and key = %x", ns, key)
		}
		return nil, err
	}

	return data, nil
}

func toLegacyKey(input []byte) []byte {
	key := hash.Hash160b(input)
	return key[:]
}

func legacyKeyLen() int {
	return 20
}

func (sf *factory) stateAtHeight(height uint64, ns string, key []byte, s interface{}) error {
	if !sf.saveHistory {
		return ErrNoArchiveData
	}
	tlt, err := newTwoLayerTrie(ArchiveTrieNamespace, sf.dao, fmt.Sprintf("%s-%d", ArchiveTrieRootKey, height), false)
	if err != nil {
		return errors.Wrapf(err, "failed to generate trie for %d", height)
	}
	if err := tlt.Start(context.Background()); err != nil {
		return err
	}
	defer tlt.Stop(context.Background())

	value, err := readState(tlt, ns, key)
	if err != nil {
		return err
	}
	return state.Deserialize(s, value)
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
	if data, ok := sf.workingsets.Get(key); ok {
		if ws, ok := data.(*workingSet); ok {
			// if it is already validated, return workingset
			return ws, true, nil
		}
		return nil, false, errors.New("type assertion failed to be WorkingSet")
	}
	ws, err := sf.newWorkingSet(ctx, sf.currentChainHeight+1)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to obtain working set from state factory")
	}
	return ws, false, nil
}

func (sf *factory) putIntoWorkingSets(key hash.Hash256, ws *workingSet) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	sf.workingsets.Add(key, ws)
}
