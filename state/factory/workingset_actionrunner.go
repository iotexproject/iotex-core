package factory

import (
	"context"

	"github.com/iotexproject/iotex-core/v2/action"
)

type (
	workingSetActionRunner struct {
		*workingSet
	}
)

func newWorkingSetSimulator(ws *workingSet) *workingSetActionRunner {
	return &workingSetActionRunner{ws}
}

func (ws *workingSetActionRunner) RunAction(ctx context.Context, selp *action.SealedEnvelope) (*action.Receipt, error) {
	receipts, err := ws.runActions(ctx, []*action.SealedEnvelope{selp})
	if err != nil {
		return nil, err
	}
	return receipts[0], nil
}
