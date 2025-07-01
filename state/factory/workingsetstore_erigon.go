package factory

import (
	"context"
	"fmt"
	"math/big"

	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/kv/mdbx"
	erigonlog "github.com/erigontech/erigon-lib/log/v3"
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

const (
	systemNS = "erigonsystem"
)

var (
	heightKey = []byte("height")
)

type erigonDB struct {
	path string
	rw   kv.RwDB
}

type erigonWorkingSetStore struct {
	db              *erigonDB
	intraBlockState *erigonstate.IntraBlockState
	tx              kv.Tx
}

func newErigonDB(path string) *erigonDB {
	return &erigonDB{path: path}
}

func (db *erigonDB) Start(ctx context.Context) error {
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

func (db *erigonDB) Stop(ctx context.Context) {
	if db.rw != nil {
		db.rw.Close()
	}
}

func (db *erigonDB) newErigonStore(ctx context.Context, height uint64) (*erigonWorkingSetStore, error) {
	tx, err := db.rw.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	r := erigonstate.NewPlainStateReader(tx)
	intraBlockState := erigonstate.New(r)
	return &erigonWorkingSetStore{
		db:              db,
		tx:              tx,
		intraBlockState: intraBlockState,
	}, nil
}

func (db *erigonDB) newErigonStoreDryrun(ctx context.Context, height uint64) (*erigonWorkingSetStore, error) {
	tx, err := db.rw.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	tsw := erigonstate.NewPlainState(tx, height, nil)
	intraBlockState := erigonstate.New(tsw)
	return &erigonWorkingSetStore{
		db:              db,
		tx:              tx,
		intraBlockState: intraBlockState,
	}, nil
}

func (store *erigonWorkingSetStore) Start(ctx context.Context) error {
	return nil
}

func (store *erigonWorkingSetStore) Stop(ctx context.Context) error {
	return nil
}

func (store *erigonWorkingSetStore) FinalizeTx(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	chainCfg, err := evm.NewChainConfig(ctx)
	if err != nil {
		return err
	}
	chainRules := chainCfg.Rules(new(big.Int).SetUint64(blkCtx.BlockHeight), g.IsSumatra(blkCtx.BlockHeight), uint64(blkCtx.BlockTimeStamp.Unix()))
	rules := evm.NewErigonRules(&chainRules)
	return store.intraBlockState.FinalizeTx(rules, erigonstate.NewNoopWriter())
}

func (store *erigonWorkingSetStore) Finalize(ctx context.Context) error {
	return nil
}

func (store *erigonWorkingSetStore) prepareCommit(ctx context.Context, tx kv.RwTx) error {
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
	err = store.intraBlockState.CommitBlock(rules, tsw)
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
	return nil
}

func (store *erigonWorkingSetStore) Commit(ctx context.Context) error {
	defer store.tx.Rollback()
	// BeginRw accounting for the context Done signal
	// statedb has been committed, so we should not use the context
	tx, err := store.db.rw.BeginRw(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to begin erigon working set store transaction")
	}
	defer tx.Rollback()

	if err = store.prepareCommit(ctx, tx); err != nil {
		return errors.Wrap(err, "failed to prepare erigon working set store commit")
	}
	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit erigon working set store transaction")
	}
	return nil
}

func (store *erigonWorkingSetStore) Close() {
	store.tx.Rollback()
}

func (store *erigonWorkingSetStore) Snapshot() int {
	return store.intraBlockState.Snapshot()
}

func (store *erigonWorkingSetStore) RevertSnapshot(sn int) error {
	store.intraBlockState.RevertToSnapshot(sn)
	return nil
}

func (store *erigonWorkingSetStore) ResetSnapshots() {}

func (store *erigonWorkingSetStore) Put(ns string, key []byte, value []byte) (err error) {
	// only handling account, contract storage handled by evm adapter
	// others are ignored
	if ns != AccountKVNamespace {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			log.L().Warn("store no account in account namespace", zap.Any("recover", r), log.Hex("key", key), zap.String("ns", ns), zap.ByteString("value", value))
			err = nil
		}
	}()
	acc := &state.Account{}
	if err := acc.Deserialize(value); err != nil {
		// should be legacy rewarding funds
		log.L().Warn("store no account in account namespace", log.Hex("key", key), zap.String("ns", ns), zap.ByteString("value", value))
		return nil
	}
	addr := libcommon.Address(key)
	if !store.intraBlockState.Exist(addr) {
		store.intraBlockState.CreateAccount(addr, false)
	}
	store.intraBlockState.SetBalance(addr, uint256.MustFromBig(acc.Balance))
	store.intraBlockState.SetNonce(addr, acc.PendingNonce()) // TODO(erigon): not sure if this is correct
	return nil
}

func (store *erigonWorkingSetStore) Get(ns string, key []byte) ([]byte, error) {
	switch ns {
	case AccountKVNamespace:
		accProto := &accountpb.Account{}
		addr := libcommon.Address(key)
		if !store.intraBlockState.Exist(addr) {
			return nil, state.ErrStateNotExist
		}
		balance := store.intraBlockState.GetBalance(addr)
		accProto.Balance = balance.String()
		nonce := store.intraBlockState.GetNonce(addr)
		accProto.Nonce = nonce
		accProto.Type = accountpb.AccountType_ZERO_NONCE
		if ch := store.intraBlockState.GetCodeHash(addr); len(ch) > 0 {
			accProto.CodeHash = store.intraBlockState.GetCodeHash(addr).Bytes()
		}
		return proto.Marshal(accProto)
	case evm.CodeKVNameSpace:
		addr := libcommon.Address(key)
		if !store.intraBlockState.Exist(addr) {
			return nil, state.ErrStateNotExist
		}
		return store.intraBlockState.GetCode(addr), nil
	default:
		return nil, errors.Errorf("unexpected erigon get namespace %s, key %x", ns, key)
	}
}

func (store *erigonWorkingSetStore) Delete(ns string, key []byte) error {
	return nil
}

func (store *erigonWorkingSetStore) WriteBatch(batch.KVStoreBatch) error {
	return nil
}

func (store *erigonWorkingSetStore) Filter(string, db.Condition, []byte, []byte) ([][]byte, [][]byte, error) {
	return nil, nil, nil
}

func (store *erigonWorkingSetStore) States(string, [][]byte) ([][]byte, [][]byte, error) {
	return nil, nil, nil
}

func (store *erigonWorkingSetStore) Digest() hash.Hash256 {
	return hash.ZeroHash256
}

func (store *erigonDB) Height() (uint64, error) {
	var height uint64
	err := store.rw.View(context.Background(), func(tx kv.Tx) error {
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
