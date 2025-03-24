package factory

import (
	"context"

	"github.com/iotexproject/iotex-core/v2/action"
)

type (
	preActionOverwrite struct {
		acts []*action.SealedEnvelope
	}
)

func WithPreActionOverwrite(acts ...*action.SealedEnvelope) StateOverwrite {
	return &preActionOverwrite{acts}
}

func (pa *preActionOverwrite) Overwrite(ctx context.Context, ws *workingSet) error {
	return ws.Process(ctx, pa.acts)
}
