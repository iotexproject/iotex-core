package erigonstore

import (
	"context"
	"fmt"
	"math/big"

	"github.com/erigontech/erigon-lib/common/datadir"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/kv/mdbx"
	"github.com/erigontech/erigon-lib/kv/temporal/historyv2"
	erigonlog "github.com/erigontech/erigon-lib/log/v3"
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/erigontech/erigon/eth/ethconfig"
	"github.com/erigontech/erigon/eth/stagedsync"
	"github.com/erigontech/erigon/eth/stagedsync/stages"
	"github.com/erigontech/erigon/ethdb/prune"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

const (
	systemNS = "erigonsystem"
)

var (
	heightKey = []byte("height")
)

// ErigonDB implements the Erigon database
type ErigonDB struct {
	path string
	rw   kv.RwDB
}

// ErigonWorkingSetStore implements the Erigon working set store
type ErigonWorkingSetStore struct {
	db      *ErigonDB
	backend *contractBackend
	tx      kv.Tx
}

// NewErigonDB creates a new ErigonDB
func NewErigonDB(path string) *ErigonDB {
	return &ErigonDB{path: path}
}

// Start starts the ErigonDB
func (db *ErigonDB) Start(ctx context.Context) error {
	log.L().Info("starting history state index")
	lg := erigonlog.New()
	lg.SetHandler(erigonlog.StdoutHandler)
	rw, err := mdbx.NewMDBX(lg).Path(db.path).WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
		defaultBuckets[systemNS] = kv.TableCfgItem{}
		return defaultBuckets
	}).Open(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open history state index")
	}
	db.rw = rw
	return nil
}

// Stop stops the ErigonDB
func (db *ErigonDB) Stop(ctx context.Context) {
	if db.rw != nil {
		db.rw.Close()
	}
}

// NewErigonStore creates a new ErigonWorkingSetStore
func (db *ErigonDB) NewErigonStore(ctx context.Context, height uint64) (*ErigonWorkingSetStore, error) {
	tx, err := db.rw.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	r := erigonstate.NewPlainStateReader(tx)
	intraBlockState := erigonstate.New(r)
	return &ErigonWorkingSetStore{
		db:      db,
		tx:      tx,
		backend: newContractBackend(ctx, intraBlockState, r),
	}, nil
}

// NewErigonStoreDryrun creates a new ErigonWorkingSetStore for dryrun
func (db *ErigonDB) NewErigonStoreDryrun(ctx context.Context, height uint64) (*ErigonWorkingSetStore, error) {
	tx, err := db.rw.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	tsw := erigonstate.NewPlainState(tx, height, nil)
	intraBlockState := erigonstate.New(tsw)
	return &ErigonWorkingSetStore{
		db:      db,
		tx:      tx,
		backend: newContractBackend(ctx, intraBlockState, tsw),
	}, nil
}

// BatchPrune prunes the ErigonWorkingSetStore in batches
func (db *ErigonDB) BatchPrune(ctx context.Context, from, to, batch uint64) error {
	if from >= to {
		return errors.Errorf("invalid prune range: from %d >= to %d", from, to)
	}
	tx, err := db.rw.BeginRo(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to begin erigon working set store transaction")
	}
	base, err := historyv2.AvailableFrom(tx)
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "failed to get available from erigon working set store")
	}
	tx.Rollback()

	if base >= from {
		log.L().Debug("batch prune nothing", zap.Uint64("from", from), zap.Uint64("to", to), zap.Uint64("base", base))
		// nothing to prune
		return nil
	}

	for batchFrom := base; batchFrom < from; batchFrom += batch {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		log.L().Info("batch prune", zap.Uint64("from", batchFrom), zap.Uint64("to", to))
		if err = db.Prune(ctx, nil, batchFrom, to); err != nil {
			return err
		}
	}
	log.L().Info("batch prune", zap.Uint64("from", from), zap.Uint64("to", to))
	return db.Prune(ctx, nil, from, to)
}

// Prune prunes the ErigonWorkingSetStore from 'from' to 'to'
func (db *ErigonDB) Prune(ctx context.Context, tx kv.RwTx, from, to uint64) error {
	if from >= to {
		return errors.Errorf("invalid prune range: from %d >= to %d", from, to)
	}
	log.L().Debug("erigon working set store prune execution stage",
		zap.Uint64("from", from),
		zap.Uint64("to", to),
	)
	s := stagedsync.PruneState{ID: stages.Execution, ForwardProgress: to}
	num := to - from
	cfg := stagedsync.StageExecuteBlocksCfg(db.rw,
		prune.Mode{
			History:    prune.Distance(num),
			Receipts:   prune.Distance(num),
			CallTraces: prune.Distance(num),
		},
		0, nil, nil, nil, nil, nil, false, false, false, datadir.Dirs{}, nil, nil, nil, ethconfig.Sync{}, nil, nil)
	err := stagedsync.PruneExecutionStage(&s, tx, cfg, ctx, false)
	if err != nil {
		return errors.Wrapf(err, "failed to prune execution stage from %d to %d", from, to)
	}
	log.L().Debug("erigon working set store prune execution stage done",
		zap.Uint64("from", from),
		zap.Uint64("to", to),
		zap.Uint64("progress", s.PruneProgress),
	)
	if to%5000 == 0 {
		log.L().Info("prune execution stage", zap.Uint64("from", from), zap.Uint64("to", to), zap.Uint64("progress", s.PruneProgress))
	}
	return nil
}

// Start starts the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) Start(ctx context.Context) error {
	return nil
}

// Stop stops the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) Stop(ctx context.Context) error {
	return nil
}

// FinalizeTx finalizes the transaction
func (store *ErigonWorkingSetStore) FinalizeTx(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	chainCfg, err := evm.NewChainConfig(ctx)
	if err != nil {
		return err
	}
	chainRules := chainCfg.Rules(new(big.Int).SetUint64(blkCtx.BlockHeight), g.IsSumatra(blkCtx.BlockHeight), uint64(blkCtx.BlockTimeStamp.Unix()))
	rules := evm.NewErigonRules(&chainRules)
	return store.backend.intraBlockState.FinalizeTx(rules, erigonstate.NewNoopWriter())
}

// Finalize finalizes the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) Finalize(ctx context.Context) error {
	return nil
}

func (store *ErigonWorkingSetStore) prepareCommit(ctx context.Context, tx kv.RwTx, retention uint64) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	height := blkCtx.BlockHeight
	ts := blkCtx.BlockTimeStamp.Unix()
	g := genesis.MustExtractGenesisContext(ctx)
	chainCfg, err := evm.NewChainConfig(ctx)
	if err != nil {
		return err
	}

	chainRules := chainCfg.Rules(big.NewInt(int64(height)), g.IsSumatra(height), uint64(ts))
	rules := evm.NewErigonRules(&chainRules)
	tsw := erigonstate.NewPlainStateWriter(tx, tx, height)
	log.L().Debug("intraBlockState Commit block", zap.Uint64("height", height))
	err = store.backend.intraBlockState.CommitBlock(rules, tsw)
	if err != nil {
		return err
	}
	log.L().Debug("erigon store finalize", zap.Uint64("height", height), zap.String("tsw", fmt.Sprintf("%T", tsw)))
	// store.intraBlockState.Print(*rules)

	log.L().Debug("erigon store write changesets", zap.Uint64("height", height))
	err = tsw.WriteChangeSets()
	if err != nil {
		return err
	}
	err = tsw.WriteHistory()
	if err != nil {
		return err
	}
	log.L().Debug("erigon store commit tx", zap.Uint64("height", height))
	err = tx.Put(systemNS, heightKey, uint256.NewInt(height).Bytes())
	if err != nil {
		return err
	}
	// Prune store if retention is set
	if retention == 0 || retention >= blkCtx.BlockHeight {
		return nil
	}
	from, to := blkCtx.BlockHeight-retention, blkCtx.BlockHeight
	return store.db.Prune(ctx, tx, from, to)
}

// Commit commits the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) Commit(ctx context.Context, retention uint64) error {
	defer store.tx.Rollback()
	// BeginRw accounting for the context Done signal
	// statedb has been committed, so we should not use the context
	tx, err := store.db.rw.BeginRw(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to begin erigon working set store transaction")
	}
	defer tx.Rollback()

	if err = store.prepareCommit(ctx, tx, retention); err != nil {
		return errors.Wrap(err, "failed to prepare erigon working set store commit")
	}
	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit erigon working set store transaction")
	}
	return nil
}

// Close closes the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) Close() {
	store.tx.Rollback()
}

// Snapshot creates a snapshot of the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) Snapshot() int {
	return store.backend.intraBlockState.Snapshot()
}

// RevertSnapshot reverts the ErigonWorkingSetStore to a snapshot
func (store *ErigonWorkingSetStore) RevertSnapshot(sn int) error {
	store.backend.intraBlockState.RevertToSnapshot(sn)
	return nil
}

// ResetSnapshots resets the snapshots of the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) ResetSnapshots() {}

// PutObject puts an object into the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) PutObject(ns string, key []byte, obj any) (err error) {
	storage, err := store.NewObjectStorage(ns, obj)
	if err != nil {
		return err
	}
	if storage == nil {
		// TODO: return error after all types are supported
		return nil
	}
	log.L().Debug("put object", zap.String("namespace", ns), log.Hex("key", key), zap.String("type", fmt.Sprintf("%T", obj)), zap.Any("content", obj))
	return storage.Store(key, obj)
}

// GetObject gets an object from the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) GetObject(ns string, key []byte, obj any) error {
	storage, err := store.NewObjectStorage(ns, obj)
	if err != nil {
		return err
	}
	if storage == nil {
		// TODO: return error after all types are supported
		return nil
	}
	defer func() {
		log.L().Debug("get object", zap.String("namespace", ns), log.Hex("key", key), zap.String("type", fmt.Sprintf("%T", obj)))
	}()
	// return storage.LoadFromContract(ns, key, store.newContractBackend(store.ctx, store.intraBlockState, store.sr))
	return storage.Load(key, obj)
}

// DeleteObject deletes an object from the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) DeleteObject(ns string, key []byte, obj any) error {
	storage, err := store.NewObjectStorage(ns, obj)
	if err != nil {
		return err
	}
	if storage == nil {
		// TODO: return error after all types are supported
		return nil
	}
	log.L().Debug("delete object", zap.String("namespace", ns), log.Hex("key", key), zap.String("type", fmt.Sprintf("%T", obj)))
	return storage.Delete(key)
}

// States gets multiple objects from the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) States(ns string, obj any, keys [][]byte) (state.Iterator, error) {
	storage, err := store.NewObjectStorage(ns, obj)
	if err != nil {
		return nil, err
	}
	if storage == nil {
		return nil, errors.Errorf("unsupported object type %T in ns %s", obj, ns)
	}
	if len(keys) == 0 {
		return storage.List()
	}
	return storage.Batch(keys)
}

// Digest returns the digest of the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) Digest() hash.Hash256 {
	return hash.ZeroHash256
}

// CreateGenesisStates creates the genesis states in the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) CreateGenesisStates(_ context.Context) error {
	deployer := store.backend
	for idx, contract := range systemContracts {
		exists := deployer.Exists(contract.Address)
		if !exists {
			log.S().Infof("Deploying system contract [%d] %s", idx, contract.Address.String())
			msg := &ethereum.CallMsg{
				From:  common.BytesToAddress(systemContractCreatorAddr[:]),
				Data:  contract.Code,
				Value: big.NewInt(0),
				Gas:   10000000,
			}
			if addr, err := deployer.Deploy(msg); err != nil {
				return fmt.Errorf("failed to deploy system contract %s: %w", contract.Address.String(), err)
			} else if addr.String() != contract.Address.String() {
				return fmt.Errorf("deployed contract address %s does not match expected address %s", addr.String(), contract.Address.String())
			}
			log.S().Infof("System contract [%d] %s deployed successfully", idx, contract.Address.String())
		} else {
			log.S().Infof("System contract [%d] %s already exists", idx, contract.Address.String())
		}
	}
	return nil
}

// Height returns the current height of the ErigonDB
func (db *ErigonDB) Height() (uint64, error) {
	var height uint64
	err := db.rw.View(context.Background(), func(tx kv.Tx) error {
		heightBytes, err := tx.GetOne(systemNS, heightKey)
		if err != nil {
			return errors.Wrap(err, "failed to get height from erigon working set store")
		}
		if len(heightBytes) == 0 {
			return nil // height not set yet
		}
		height256 := new(uint256.Int)
		height256.SetBytes(heightBytes)
		height = height256.Uint64()
		return nil
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to get height from erigon working set store")
	}
	return height, nil
}

func newContractBackend(ctx context.Context, intraBlockState *erigonstate.IntraBlockState, sr erigonstate.StateReader) *contractBackend {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g, ok := genesis.ExtractGenesisContext(ctx)
	if !ok {
		log.S().Panic("failed to extract genesis context from block context")
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	return NewContractBackend(intraBlockState, sr, blkCtx.BlockHeight, blkCtx.BlockTimeStamp, blkCtx.Producer, &g, bcCtx.EvmNetworkID, protocol.MustGetFeatureCtx(ctx).UseZeroNonceForFreshAccount)
}

// KVStore returns nil as ErigonWorkingSetStore does not implement KVStore
func (store *ErigonWorkingSetStore) KVStore() db.KVStore {
	return nil
}

// IntraBlockState returns the intraBlockState of the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) IntraBlockState() *erigonstate.IntraBlockState {
	return store.backend.intraBlockState
}

// StateReader returns the state reader of the ErigonWorkingSetStore
func (store *ErigonWorkingSetStore) StateReader() erigonstate.StateReader {
	return store.backend.org
}

func (store *ErigonWorkingSetStore) NewObjectStorage(ns string, obj any) (ObjectStorage, error) {
	var contractAddr address.Address
	switch ns {
	case "Account":
		if _, ok := obj.(*state.Account); !ok {
			return nil, nil
		}
		return newAccountStorage(
			common.BytesToAddress(systemContracts[AccountInfoContractIndex].Address.Bytes()),
			store.backend,
		)
	case "BlockMeta":
		contractAddr = systemContracts[PollBlockMetaContractIndex].Address
	default:
		// TODO: fail unknown namespace
		return nil, nil
	}
	// TODO: cache storage
	contract, err := systemcontracts.NewGenericStorageContract(common.BytesToAddress(contractAddr.Bytes()[:]), store.backend, common.Address(systemContractCreatorAddr))
	if err != nil {
		return nil, err
	}
	return newContractObjectStorage(contract), nil
}
