package factory

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	erigonlog "github.com/ledgerwatch/log/v3"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
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
	path         string
	rw           kv.RwDB
	getBlockTime func(uint64) (time.Time, error)
}

type erigonStore struct {
	tsw             erigonstate.StateWriter
	intraBlockState *erigonstate.IntraBlockState
	tx              kv.Tx
	getBlockTime    func(uint64) (time.Time, error)
}

func newErigonDB(path string, getBlockTime func(uint64) (time.Time, error)) *erigonDB {
	return &erigonDB{path: path, getBlockTime: getBlockTime}
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

func (db *erigonDB) newErigonStore(ctx context.Context, height uint64) (*erigonStore, error) {
	tx, err := db.rw.BeginRw(ctx)
	if err != nil {
		return nil, err
	}
	r, tsw := erigonstate.NewPlainStateReader(tx), erigonstate.NewPlainStateWriter(tx, tx, height)
	intraBlockState := erigonstate.New(r)
	return &erigonStore{
		tsw:             tsw,
		tx:              tx,
		intraBlockState: intraBlockState,
		getBlockTime:    db.getBlockTime,
	}, nil
}

func (db *erigonDB) newErigonStoreDryrun(ctx context.Context, height uint64) (*erigonStore, error) {
	tx, err := db.rw.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	tsw := erigonstate.NewPlainState(tx, height, nil)
	intraBlockState := erigonstate.New(tsw)
	return &erigonStore{
		tsw:             tsw,
		tx:              tx,
		intraBlockState: intraBlockState,
		getBlockTime:    db.getBlockTime,
	}, nil
}

func (store *erigonStore) finalizeTx(ctx context.Context) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	chainCfg, err := evm.NewChainConfig(g.Blockchain, blkCtx.BlockHeight, protocol.MustGetBlockchainCtx(ctx).EvmNetworkID, store.getBlockTime)
	if err != nil {
		return err
	}
	chainRules := chainCfg.Rules(big.NewInt(int64(blkCtx.BlockHeight)), g.IsSumatra(blkCtx.BlockHeight), uint64(blkCtx.BlockTimeStamp.Unix()))
	rules := evm.NewErigonRules(&chainRules)
	return store.intraBlockState.FinalizeTx(rules, store.tsw)
}

func (store *erigonStore) finalize(ctx context.Context, height uint64, ts uint64) error {
	g := genesis.MustExtractGenesisContext(ctx)
	chainCfg, err := evm.NewChainConfig(g.Blockchain, height, protocol.MustGetBlockchainCtx(ctx).EvmNetworkID, store.getBlockTime)
	if err != nil {
		return err
	}

	chainRules := chainCfg.Rules(big.NewInt(int64(height)), g.IsSumatra(height), uint64(ts))
	rules := evm.NewErigonRules(&chainRules)
	log.L().Debug("intraBlockState Commit block", zap.Uint64("height", height))
	err = store.intraBlockState.CommitBlock(rules, store.tsw)
	if err != nil {
		return err
	}
	log.L().Debug("erigon store finalize", zap.Uint64("height", height), zap.String("tsw", fmt.Sprintf("%+T", store.tsw)))
	// store.intraBlockState.Print(*rules)

	if c, ok := store.tsw.(erigonstate.WriterWithChangeSets); ok {
		log.L().Debug("erigon store write changesets", zap.Uint64("height", height))
		err = c.WriteChangeSets()
		if err != nil {
			return err
		}
		err = c.WriteHistory()
		if err != nil {
			return err
		}
	}
	if tx, ok := store.tx.(kv.RwTx); ok {
		log.L().Debug("erigon store commit tx", zap.Uint64("height", height))
		err = tx.Put(systemNS, heightKey, uint256.NewInt(height).Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func (store *erigonStore) commit(ctx context.Context) error {
	defer store.tx.Rollback()
	err := store.tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (store *erigonStore) rollback() {
	store.tx.Rollback()
}

func (store *erigonStore) snapshot() int {
	return store.intraBlockState.Snapshot()
}

func (store *erigonStore) revertToSnapshot(sn int) {
	store.intraBlockState.RevertToSnapshot(sn)
}

func (store *erigonStore) put(ns string, key []byte, value []byte) (err error) {
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

func (store *erigonStore) get(ns string, key []byte) ([]byte, error) {
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
