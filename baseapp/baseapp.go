package baseapp

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/gasstation"

	abci "github.com/iotexproject/iotex-sdk/abci/types"
	sdk "github.com/iotexproject/iotex-sdk/types"
)

const (
	_runTxModeCheck    runTxMode = iota // Check a transaction
	_runTxModeReCheck                   // Recheck a (pending) transaction after a commit
	_runTxModeSimulate                  // Simulate a transaction
	_runTxModeDeliver                   // Deliver a transaction
)

type (
	// Enum mode for app.runTx
	runTxMode uint8
)

// BaseApp reflects the ABCI application implementation.
type BaseApp struct {
	name      string        // application name from abci.Info
	router    sdk.Router    // handle any kind of message
	txDecoder sdk.TxDecoder // unmarshal []byte into sdk.Tx

	anteHandler sdk.AnteHandler // ante handler for fee and auth
	postHandler sdk.AnteHandler // post handler, optional, e.g. for tips
	// initChainer  sdk.InitChainer  // initialize state with validators and state blob
	beginBlocker sdk.BeginBlocker // logic to run before any txs
	// endBlocker   sdk.EndBlocker   // logic to run after all txs, and to determine valset changes

	// volatile states:
	// checkState is set on InitChain and reset on Commit
	// deliverState is set on InitChain and BeginBlock and set to nil on Commit
	checkState   *state // for CheckTx
	deliverState *state // for DeliverTx

	cfg config.API
	bc  blockchain.Blockchain
	dao blockdao.BlockDAO
	gs  *gasstation.GasStation
}

func NewBaseApp(
	name string,
	cfg config.API,
	chain blockchain.Blockchain,
	dao blockdao.BlockDAO,
	txDecoder sdk.TxDecoder,
	options ...func(*BaseApp),
) *BaseApp {
	app := &BaseApp{
		name:      name,
		cfg:       cfg,
		bc:        chain,
		dao:       dao,
		txDecoder: txDecoder,
		gs:        gasstation.NewGasStation(chain, dao, cfg),
	}
	for _, option := range options {
		option(app)
	}
	return app
}

func (app *BaseApp) CheckTx(req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	var mode runTxMode

	switch {
	case req.Type == abci.CheckTxType_NEW:
		mode = _runTxModeCheck
	case req.Type == abci.CheckTxType_RECHECK:
		mode = _runTxModeReCheck
	default:
		panic(fmt.Sprintf("unknown RequestCheckTx type: %s", req.Type))
	}

	gInfo, result, err := app.runTx(mode, req.Tx)
	if err != nil {
		return nil, err
	}

	return &abci.ResponseCheckTx{
		GasWanted: int64(gInfo.GasWanted), // TODO: Should type accept unsigned ints?
		GasUsed:   int64(gInfo.GasUsed),   // TODO: Should type accept unsigned ints?
		Log:       result.Log,
		Data:      result.Data,
	}, nil
}

func (app *BaseApp) DeliverTx(req *abci.RequestDeliverTx) (*abci.ResponseDeliverTx, error) {
	gInfo, result, err := app.runTx(_runTxModeDeliver, req.Tx)
	if err != nil {
		return nil, err
	}

	return &abci.ResponseDeliverTx{
		GasWanted: int64(gInfo.GasWanted), // TODO: Should type accept unsigned ints?
		GasUsed:   int64(gInfo.GasUsed),   // TODO: Should type accept unsigned ints?
		Log:       result.Log,
		Data:      result.Data,
	}, nil
}

func (app *BaseApp) BeginBlock(req *abci.RequestBeginBlock) (*abci.RequestBeginBlock, error) {

	return &abci.RequestBeginBlock{}, nil
}

// Returns the applications's deliverState if app is in runTxModeDeliver,
// otherwise it returns the application's checkstate.
func (app *BaseApp) getState(mode runTxMode) *state {
	if mode == _runTxModeDeliver {
		return app.deliverState
	}

	return app.checkState
}

// retrieve the context for the tx w/ txBytes and other memoized values.
func (app *BaseApp) getContextForTx(mode runTxMode, txBytes []byte) sdk.Context {
	ctx := app.getState(mode).ctx.
		WithTxBytes(txBytes)

	if mode == _runTxModeReCheck {
		ctx = ctx.WithIsReCheckTx(true)
	}
	if mode == _runTxModeSimulate {
		// ctx, _ = ctx.CacheContext()
	}
	return ctx
}

// cacheTxContext returns a new context based off of the provided context with
// a cache wrapped multi-store.
// func (app *BaseApp) cacheTxContext(ctx sdk.Context, txBytes []byte) (sdk.Context, sdk.CacheMultiStore) {
// 	ms := ctx.MultiStore()
// }

func (app *BaseApp) runTx(mode runTxMode, txBytes []byte) (abci.GasInfo, *abci.Result, error) {
	tx, err := app.txDecoder(txBytes)
	if err != nil {
		return abci.GasInfo{}, nil, err
	}

	gasWanted, err := app.gs.SuggestGasPrice()
	if err != nil {
		return abci.GasInfo{}, nil, err
	}

	// _, receipt, err := evm.ExecuteContract(ctx, sm, exec, dao.BlockHash, depositGas)

	gInfo := abci.GasInfo{GasWanted: gasWanted}

	msgs := tx.GetMsgs()
	if err := validateBasicTxMsgs(msgs); err != nil {
		return abci.GasInfo{}, nil, err
	}

	// ctx := app.getContextForTx(mode, txBytes)
	// cacheCtx, msCache = app.cacheTxContext(ctx, txBytes)
	cacheCtx := sdk.Context{}
	if app.anteHandler != nil {
		_, err := app.anteHandler(cacheCtx, tx, mode == _runTxModeSimulate)
		if err != nil {
			return gInfo, nil, err
		}
	}

	result, err := app.runMsgs(cacheCtx, msgs, mode)
	if err == nil && mode == _runTxModeDeliver {
		// msCache.Write()
	}

	return gInfo, result, err
}

func (app *BaseApp) runMsgs(ctx sdk.Context, msgs []sdk.Msg, mode runTxMode) (*abci.Result, error) {
	// msgLogs := make([]*action.TransactionLog, 0, len(msgs))
	data := make([]byte, 0, len(msgs))
	// events := sdk.EmptyEvents()

	for i, msg := range msgs {
		if mode == _runTxModeCheck || mode == _runTxModeReCheck {
			break
		}

		msgRoute := msg.Route()
		handler := app.router.Route(ctx, msgRoute)
		if handler == nil {
			return nil, errors.Errorf("unrecognized message route: %s; message index: %d", msgRoute, i)
		}

		msgResult, err := handler(ctx, msg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to execute message; message index: %d", i)
		}

		data = append(data, msgResult.Data...)
		// msgLogs = append(msgLogs, msgResult.Log)
	}
	return &abci.Result{
		Data: data,
		// Log:
		// Events:
	}, nil
}

// validateBasicTxMsgs executes basic validator calls for messages.
func validateBasicTxMsgs(msgs []sdk.Msg) error {
	if len(msgs) == 0 {
		return errors.New("must contain at least one message")
	}

	for _, msg := range msgs {
		err := msg.ValidateBasic()
		if err != nil {
			return err
		}
	}
	return nil
}
