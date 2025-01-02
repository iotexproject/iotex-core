package factory

import (
	"context"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	erigonstate "github.com/ledgerwatch/erigon/core/state"
	erigonlog "github.com/ledgerwatch/log/v3"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	systemNS = "erigonsystem"
)

var (
	heightKey = []byte("height")
)

type HistoryState interface {
	// save historical states
	blockdao.BlockIndexer

	// query historical states
	// 1. query history account, impl state reader
	// 2. query history storage, impl state manager
	// 3. simulate in history, impl state manager
	StateManagerAt(height uint64) protocol.StateManager
}

type HistoryStateIndex struct {
	sdb    *stateDB
	rw     kv.RwDB
	path   string
	height uint64

	getBlockTime func(uint64) (time.Time, error)
}

func NewHistoryStateIndex(sdb Factory, path string, getBlockTime func(uint64) (time.Time, error)) *HistoryStateIndex {
	h := &HistoryStateIndex{
		sdb:          sdb.(*stateDB),
		path:         path,
		getBlockTime: getBlockTime,
	}
	return h
}

func (h *HistoryStateIndex) Start(ctx context.Context) error {
	lg := erigonlog.New()
	lg.SetHandler(erigonlog.StdoutHandler)
	rw, err := mdbx.NewMDBX(lg).Path(h.path).Open(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open history state index")
	}
	h.rw = rw
	tx, err := rw.BeginRo(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to begin read transaction")
	}
	defer tx.Rollback()
	heightBytes, err := tx.GetOne(systemNS, heightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get height")
	}
	if heightBytes == nil {
		log.L().Info("history state index is empty")
		return nil
	}
	height := uint256.NewInt(0).SetBytes(heightBytes).Uint64()
	atomic.StoreUint64(&h.height, height)
	log.L().Info("history state index is started", zap.Uint64("height", height))
	return nil
}

func (h *HistoryStateIndex) Stop(ctx context.Context) error {
	if h.rw != nil {
		h.rw.Close()
	}
	return nil
}

func (h *HistoryStateIndex) Height() (uint64, error) {
	return atomic.LoadUint64(&h.height), nil
}

func (h *HistoryStateIndex) PutBlock(ctx context.Context, blk *block.Block) error {
	ws, err := h.sdb.newWorkingSet(ctx, blk.Height())
	if err != nil {
		return err
	}
	tx, err := h.rw.BeginRw(ctx)
	if err != nil {
		return err
	}
	r, tsw := erigonstate.NewDbStateReader(tx), erigonstate.NewDbStateWriter(tx, blk.Height())
	intraBlockState := erigonstate.New(r)

	hws := &historyWorkingSet{
		workingSet: ws,
		sw:         tsw,
		intra:      intraBlockState,
	}
	err = hws.Process(protocol.WithErigonCtx(ctx), blk.Actions)
	if err != nil {
		return err
	}

	g := h.sdb.cfg.Genesis
	chainCfg, err := evm.NewChainConfig(g.Blockchain, blk.Height(), protocol.MustGetBlockchainCtx(ctx).EvmNetworkID, h.getBlockTime)
	if err != nil {
		return err
	}

	chainRules := chainCfg.Rules(big.NewInt(int64(blk.Height())), g.IsSumatra(blk.Height()), uint64(blk.Timestamp().Unix()))
	rules := evm.NewErigonRules(&chainRules)
	err = intraBlockState.CommitBlock(rules, tsw)
	if err != nil {
		return err
	}
	err = tsw.WriteChangeSets()
	if err != nil {
		return err
	}
	err = tsw.WriteHistory()
	if err != nil {
		return err
	}
	tx.Put(systemNS, heightKey, uint256.NewInt(blk.Height()).Bytes())
	err = tx.Commit()
	if err != nil {
		return err
	}
	atomic.StoreUint64(&h.height, blk.Height())
	return nil
}

type historyWorkingSet struct {
	*workingSet
	sw    *erigonstate.DbStateWriter
	intra *erigonstate.IntraBlockState
}

func (hws *historyWorkingSet) StateWriter() *erigonstate.DbStateWriter {
	return hws.sw
}
func (hws *historyWorkingSet) Intra() *erigonstate.IntraBlockState {
	return hws.intra
}
