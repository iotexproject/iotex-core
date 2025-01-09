package factory

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type reader interface {
	Get(string, []byte) ([]byte, error)
	States(string, [][]byte) ([][]byte, [][]byte, error)
	Digest() hash.Hash256
	ReadView(string) (interface{}, error)
}

type writer interface {
	WriteView(name string, value interface{}) error
	Put(ns string, key []byte, value []byte) error
	Delete(ns string, key []byte) error
	Snapshot() int
	RevertSnapshot(snapshot int) error
	ResetSnapshots()
}

// treat erigon as 3rd output, still read from statedb
// it's used for PutBlock, generating historical states in erigon
type stateDBWorkingSetStoreWithErigonOutput struct {
	reader
	store       *stateDBWorkingSetStore
	erigonStore *erigonStore
	snMap       map[int]int
}

type erigonStore struct {
	tsw             erigonstate.StateWriter
	intraBlockState *erigonstate.IntraBlockState
	tx              kv.Tx
	getBlockTime    func(uint64) (time.Time, error)
}

func newStateDBWorkingSetStoreWithErigonOutput(store *stateDBWorkingSetStore, erigonStore *erigonStore) *stateDBWorkingSetStoreWithErigonOutput {
	return &stateDBWorkingSetStoreWithErigonOutput{
		reader:      store,
		store:       store,
		erigonStore: erigonStore,
		snMap:       make(map[int]int),
	}
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Start(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Stop(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Finalize(ctx context.Context, height uint64) error {
	if err := store.store.Finalize(ctx, height); err != nil {
		return err
	}
	return store.erigonStore.finalize(ctx, height, uint64(protocol.MustGetBlockCtx(ctx).BlockTimeStamp.Unix()))
}

func (store *stateDBWorkingSetStoreWithErigonOutput) WriteView(name string, value interface{}) error {
	return store.store.WriteView(name, value)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Put(ns string, key []byte, value []byte) error {
	if err := store.store.Put(ns, key, value); err != nil {
		return err
	}
	return store.erigonStore.put(ns, key, value)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Delete(ns string, key []byte) error {
	// delete won't happen in account and contract
	return store.store.Delete(ns, key)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Commit(ctx context.Context) error {
	if err := store.store.Commit(ctx); err != nil {
		return err
	}
	return store.erigonStore.commit(ctx)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Snapshot() int {
	sn := store.store.Snapshot()
	isn := store.erigonStore.intraBlockState.Snapshot()
	store.snMap[sn] = isn
	return sn
}

func (store *stateDBWorkingSetStoreWithErigonOutput) RevertSnapshot(sn int) error {
	store.store.RevertSnapshot(sn)
	if isn, ok := store.snMap[sn]; ok {
		store.erigonStore.intraBlockState.RevertToSnapshot(isn)
		delete(store.snMap, sn)
	} else {
		panic(fmt.Sprintf("no isn for sn %d", sn))
	}
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonOutput) ResetSnapshots() {
	store.store.ResetSnapshots()
	store.snMap = make(map[int]int)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Close() {
	store.erigonStore.tx.Rollback()
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
		acc := &state.Account{}
		addr := libcommon.Address(key)
		if !store.intraBlockState.Exist(addr) {
			return nil, state.ErrStateNotExist
		}
		balance := store.intraBlockState.GetBalance(addr)
		acc.Balance = balance.ToBig()
		acc.SetPendingNonce(store.intraBlockState.GetNonce(addr))
		if ch := store.intraBlockState.GetCodeHash(addr); len(ch) > 0 {
			acc.CodeHash = store.intraBlockState.GetCodeHash(addr).Bytes()
		}
		return acc.Serialize()
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

// used in historical states query
// account & contract read & write on erigon
type stateDBWorkingSetStoreWithErigonDryrun struct {
	writer
	store       *stateDBWorkingSetStore // fallback to statedb for staking, rewarding and poll
	erigonStore *erigonStore
}

func newStateDBWorkingSetStoreWithErigonDryrun(store *stateDBWorkingSetStore, erigonStore *erigonStore) *stateDBWorkingSetStoreWithErigonDryrun {
	return &stateDBWorkingSetStoreWithErigonDryrun{
		store:       store,
		erigonStore: erigonStore,
		writer:      newStateDBWorkingSetStoreWithErigonOutput(store, erigonStore),
	}
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Start(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Stop(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Get(ns string, key []byte) ([]byte, error) {
	switch ns {
	case AccountKVNamespace, evm.CodeKVNameSpace:
		return store.erigonStore.get(ns, key)
	default:
		return store.store.Get(ns, key)
	}
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) States(ns string, keys [][]byte) ([][]byte, [][]byte, error) {
	// currently only used for staking & poll, no need to read from erigon
	return store.store.States(ns, keys)
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Finalize(_ context.Context, height uint64) error {
	// do nothing for dryrun
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) ReadView(name string) (interface{}, error) {
	// only used for staking
	return store.store.ReadView(name)
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Digest() hash.Hash256 {
	return store.store.Digest()
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Commit(context.Context) error {
	// do nothing for dryrun
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Close() {
	store.erigonStore.tx.Rollback()
}

// func (store *stateDBWorkingSetStoreWithErigonDryrun) WriteView(name string, value interface{}) error {
// 	return store.outer.WriteView(name, value)
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) Put(ns string, key []byte, value []byte) error {
// 	return store.outer.Put(ns, key, value)
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) Delete(ns string, key []byte) error {
// 	return store.outer.Delete(ns, key)
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) Snapshot() int {
// 	return store.outer.Snapshot()
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) RevertSnapshot(sn int) error {
// 	return store.outer.RevertSnapshot(sn)
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) ResetSnapshots() {
// 	store.outer.ResetSnapshots()
// }
