package baseapp

import (
	"fmt"

	abci "github.com/iotexproject/iotex-sdk/abci/types"
	sdk "github.com/iotexproject/iotex-sdk/types"
)

const (
	runTxModeCheck    runTxMode = iota // Check a transaction
	runTxModeReCheck                   // Recheck a (pending) transaction after a commit
	runTxModeSimulate                  // Simulate a transaction
	runTxModeDeliver                   // Deliver a transaction
)

type (
	// Enum mode for app.runTx
	runTxMode uint8
)

// BaseApp reflects the ABCI application implementation.
type BaseApp struct {
	name string // application name from abci.Info
	// db
	// cache

	anteHandler sdk.AnteHandler // ante handler for fee and auth
	postHandler sdk.AnteHandler // post handler, optional, e.g. for tips
	// initChainer  sdk.InitChainer  // initialize state with validators and state blob
	beginBlocker sdk.BeginBlocker // logic to run before any txs
	// endBlocker   sdk.EndBlocker   // logic to run after all txs, and to determine valset changes
}

func (app *BaseApp) CheckTx(req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	var mode runTxMode

	switch {
	case req.Type == abci.CheckTxType_NEW:
		mode = runTxModeCheck

	case req.Type == abci.CheckTxType_RECHECK:
		mode = runTxModeReCheck

	default:
		panic(fmt.Sprintf("unknown RequestCheckTx type: %s", req.Type))
	}

	gInfo, result, _, _, err := app.runTx(mode, req.Tx)
	if err != nil {
		return nil, err
	}

	return &abci.ResponseCheckTx{
		GasWanted: int64(gInfo.GasWanted), // TODO: Should type accept unsigned ints?
		GasUsed:   int64(gInfo.GasUsed),   // TODO: Should type accept unsigned ints?
		Log:       result.Log,
		Data:      result.Data,
		// Events:    sdk.MarkEventsToIndex(result.Events, app.indexEvents),
		// Priority:  priority,
	}, nil
}

func (app *BaseApp) DeliverTx(req *abci.RequestDeliverTx) (*abci.ResponseDeliverTx, error) {
	gInfo := abci.GasInfo{}
	// resultStr := "successful"

	gInfo, result, _, _, err := app.runTx(runTxModeDeliver, req.Tx)
	if err != nil {
		// resultStr = "failed"
		return nil, err
	}

	return &abci.ResponseDeliverTx{
		GasWanted: int64(gInfo.GasWanted), // TODO: Should type accept unsigned ints?
		GasUsed:   int64(gInfo.GasUsed),   // TODO: Should type accept unsigned ints?
		Log:       result.Log,
		Data:      result.Data,
		// Events:    sdk.MarkEventsToIndex(result.Events, app.indexEvents),
	}, nil
}

func (app *BaseApp) BeginBlock(req *abci.RequestBeginBlock) (*abci.RequestBeginBlock, error) {

	return &abci.RequestBeginBlock{}, nil
}

func (app *BaseApp) runTx(mode runTxMode, txBytes []byte) (gInfo abci.GasInfo, result *abci.Result, anteEvents []abci.Event, priority int64, err error) {

	return gInfo, nil, nil, 0, nil
}
