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

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	erigonlog "github.com/ledgerwatch/log/v3"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
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

func (h *HistoryStateIndex) SetGetBlockTime(getBlockTime func(uint64) (time.Time, error)) {
	h.getBlockTime = getBlockTime
}

func (h *HistoryStateIndex) Start(ctx context.Context) error {
	log.L().Info("starting history state index")
	lg := erigonlog.New()
	lg.SetHandler(erigonlog.StdoutHandler)
	rw, err := mdbx.NewMDBX(lg).Path(h.path).WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
		defaultBuckets[systemNS] = kv.TableCfgItem{}
		return defaultBuckets
	}).Open(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open history state index")
	}
	h.rw = rw
	tx, err := rw.BeginRw(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to begin read transaction")
	}
	defer tx.Rollback()
	exist, err := tx.Has(systemNS, heightKey)
	if err != nil {
		return errors.Wrap(err, "failed to check height key")
	}
	if !exist {
		log.L().Info("history state index is empty")
		ctx = protocol.WithBlockCtx(
			ctx,
			protocol.BlockCtx{
				BlockHeight:    0,
				BlockTimeStamp: time.Unix(h.sdb.cfg.Genesis.Timestamp, 0),
				Producer:       h.sdb.cfg.Chain.ProducerAddress(),
				GasLimit:       h.sdb.cfg.Genesis.BlockGasLimitByHeight(0),
			})
		ctx = protocol.WithFeatureCtx(ctx)
		// init the state factory
		ws, err := h.sdb.newWorkingSet(ctx, 0)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		r, tsw := erigonstate.NewPlainStateReader(tx), erigonstate.NewPlainStateWriter(tx, tx, 0)
		intraBlockState := erigonstate.New(r)
		intraBlockState.SetTrace(true)
		hws := &historyWorkingSetRo{
			historyWorkingSet: &historyWorkingSet{
				workingSet: ws,
				sw:         tsw,
				intra:      intraBlockState,
			},
		}
		ctx = protocol.WithRegistry(ctx, h.sdb.registry)
		if err := hws.CreateGenesisStates(ctx); err != nil {
			return err
		}
		return h.commit(ctx, tx, tsw, intraBlockState, 0, uint64(h.sdb.cfg.Genesis.Timestamp))
	}
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
	log.L().Info("stopping history state index")
	if h.rw != nil {
		h.rw.Close()
	}
	log.L().Info("history state index is stopped")
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
	defer tx.Rollback()
	r, tsw := erigonstate.NewPlainStateReader(tx), erigonstate.NewPlainStateWriter(tx, tx, blk.Height())
	intraBlockState := erigonstate.New(r)
	intraBlockState.SetTrace(true)

	hws := &historyWorkingSet{
		workingSet: ws,
		sw:         tsw,
		intra:      intraBlockState,
	}
	ctx = protocol.WithRegistry(ctx, h.sdb.registry)
	err = hws.processWithCorrectOrder(protocol.WithErigonCtx(ctx), blk.Actions)
	if err != nil {
		return err
	}

	return h.commit(ctx, tx, tsw, intraBlockState, blk.Height(), uint64(blk.Timestamp().Unix()))
}

func (h *HistoryStateIndex) commit(ctx context.Context, tx kv.RwTx, tsw *erigonstate.PlainStateWriter, intraBlockState *erigonstate.IntraBlockState, height uint64, ts uint64) error {
	g := h.sdb.cfg.Genesis
	chainCfg, err := evm.NewChainConfig(g.Blockchain, height, protocol.MustGetBlockchainCtx(ctx).EvmNetworkID, h.getBlockTime)
	if err != nil {
		return err
	}

	chainRules := chainCfg.Rules(big.NewInt(int64(height)), g.IsSumatra(height), uint64(ts))
	rules := evm.NewErigonRules(&chainRules)
	log.L().Debug("intraBlockState Commit block", zap.Uint64("height", height))
	err = intraBlockState.CommitBlock(rules, tsw)
	if err != nil {
		return err
	}
	intraBlockState.Print(*rules)

	err = tsw.WriteChangeSets()
	if err != nil {
		return err
	}
	err = tsw.WriteHistory()
	if err != nil {
		return err
	}
	err = tx.Put(systemNS, heightKey, uint256.NewInt(height).Bytes())
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	atomic.StoreUint64(&h.height, height)
	return nil
}

func (h *HistoryStateIndex) StateManagerAt(ctx context.Context, height uint64) (*historyWorkingSetRo, error) {
	ws, err := h.sdb.newWorkingSet(ctx, height)
	if err != nil {
		return nil, err
	}
	tx, err := h.rw.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	tsw := erigonstate.NewPlainState(tx, height, nil)
	intraBlockState := erigonstate.New(tsw)
	return &historyWorkingSetRo{
		historyWorkingSet: &historyWorkingSet{
			workingSet: ws,
			sw:         tsw,
			intra:      intraBlockState,
			cleanup:    func() { tx.Rollback() },
		},
	}, nil
}

type historyWorkingSet struct {
	*workingSet
	sw      erigonstate.StateWriter
	intra   *erigonstate.IntraBlockState
	cleanup func()
}

func (hws *historyWorkingSet) StateWriter() erigonstate.StateWriter {
	return hws.sw
}

func (hws *historyWorkingSet) Intra() *erigonstate.IntraBlockState {
	return hws.intra
}

func (hws *historyWorkingSet) Close() {
	hws.cleanup()
}

func (ws *historyWorkingSet) processWithCorrectOrder(ctx context.Context, actions []*action.SealedEnvelope) error {
	reg := protocol.MustGetRegistry(ctx)
	for _, p := range reg.All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return err
			}
		}
	}
	var (
		receipts            = make([]*action.Receipt, 0)
		ctxWithBlockContext = ctx
		blkCtx              = protocol.MustGetBlockCtx(ctx)
		fCtx                = protocol.MustGetFeatureCtx(ctx)
	)
	for _, act := range actions {
		actionCtx, err := withActionCtx(ctxWithBlockContext, act)
		if err != nil {
			return err
		}
		receipt, err := ws.runAction(actionCtx, act)
		if err != nil {
			return errors.Wrap(err, "error when run action")
		}
		receipts = append(receipts, receipt)
		if !action.IsSystemAction(act) {
			blkCtx.GasLimit -= receipt.GasConsumed
			if fCtx.EnableDynamicFeeTx && receipt.PriorityFee() != nil {
				(&blkCtx.AccumulatedTips).Add(&blkCtx.AccumulatedTips, receipt.PriorityFee())
			}
			ctxWithBlockContext = protocol.WithBlockCtx(ctx, blkCtx)
		}
	}
	return nil
}

func (ws *historyWorkingSet) runAction(
	ctx context.Context,
	selp *action.SealedEnvelope,
) (*action.Receipt, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	fCtx := protocol.MustGetFeatureCtx(ctx)
	// if it's a tx container, unfold the tx inside
	if fCtx.UseTxContainer && !fCtx.UnfoldContainerBeforeValidate {
		if container, ok := selp.Envelope.(action.TxContainer); ok {
			if err := container.Unfold(selp, ctx, ws.checkContract); err != nil {
				return nil, errors.Wrap(errUnfoldTxContainer, err.Error())
			}
		}
	}
	// Handle action
	reg, ok := protocol.GetRegistry(ctx)
	if !ok {
		return nil, errors.New("protocol is empty")
	}
	selpHash, err := selp.Hash()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get hash")
	}
	defer ws.ResetSnapshots()
	if err := ws.freshAccountConversion(ctx, &actCtx); err != nil {
		return nil, err
	}
	for _, actionHandler := range reg.All() {
		receipt, err := actionHandler.Handle(ctx, selp.Envelope, ws)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"error when action %x mutates states",
				selpHash,
			)
		}
		if receipt != nil {
			if fCtx.EnableBlobTransaction && len(selp.BlobHashes()) > 0 {
				if err = ws.handleBlob(ctx, selp, receipt); err != nil {
					return nil, err
				}
			}
			return receipt, nil
		}
	}
	return nil, errors.New("receipt is empty")
}

func (ws *historyWorkingSet) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	h, err := ws.workingSet.PutState(s, opts...)
	if err != nil {
		return h, err
	}
	cfg, err := processOptions(opts...)
	if err != nil {
		return h, err
	}
	if cfg.Namespace != AccountKVNamespace {
		return h, nil
	}
	if acc, ok := s.(*state.Account); ok {
		addr := libcommon.Address(cfg.Key)
		if !ws.intra.Exist(addr) {
			ws.intra.CreateAccount(addr, false)
		}
		ws.intra.SetBalance(addr, uint256.MustFromBig(acc.Balance))
		ws.intra.SetNonce(addr, acc.PendingNonce()) // TODO(erigon): not sure if this is correct
	}
	return h, nil
}

type historyWorkingSetRo struct {
	*historyWorkingSet
}

func (ws *historyWorkingSetRo) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := processOptions(opts...)
	if err != nil {
		return ws.height, err
	}
	if cfg.Keys != nil {
		return 0, errors.Wrap(ErrNotSupported, "Read state with keys option has not been implemented yet")
	}
	switch cfg.Namespace {
	case AccountKVNamespace:
		if acc, ok := s.(*state.Account); ok {
			addr := libcommon.Address(cfg.Key)
			if !ws.intra.Exist(addr) {
				return ws.height, state.ErrStateNotExist
			}
			balance := ws.intra.GetBalance(addr)
			acc.Balance = balance.ToBig()
			acc.SetPendingNonce(ws.intra.GetNonce(addr))
			if ch := ws.intra.GetCodeHash(addr); len(ch) > 0 {
				acc.CodeHash = ws.intra.GetCodeHash(addr).Bytes()
			}
			return ws.height, nil
		}
	case evm.CodeKVNameSpace:
		addr := libcommon.Address(cfg.Key)
		if !ws.intra.Exist(addr) {
			return ws.height, state.ErrStateNotExist
		}
		return ws.height, state.Deserialize(s, ws.intra.GetCode(addr))
	}
	return ws.historyWorkingSet.State(s, opts...)
}

func (ws *historyWorkingSetRo) CreateGenesisStates(ctx context.Context) error {
	if reg, ok := protocol.GetRegistry(ctx); ok {
		for _, p := range reg.All() {
			if gsc, ok := p.(*account.Protocol); ok {
				if err := gsc.CreateGenesisStates(ctx, ws); err != nil {
					return errors.Wrap(err, "failed to create genesis states for protocol")
				}
			}
		}
	}
	return nil
}

func (ws *historyWorkingSetRo) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	h := ws.height
	cfg, err := processOptions(opts...)
	if err != nil {
		return h, err
	}
	if cfg.Namespace != AccountKVNamespace {
		return h, nil
	}
	if acc, ok := s.(*state.Account); ok {
		addr := libcommon.Address(cfg.Key)
		if !ws.intra.Exist(addr) {
			ws.intra.CreateAccount(addr, false)
		}
		ws.intra.SetBalance(addr, uint256.MustFromBig(acc.Balance))
		ws.intra.SetNonce(addr, acc.PendingNonce()) // TODO(erigon): not sure if this is correct
	}
	return h, nil
}
