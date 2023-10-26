package evm

import (
	"context"

	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	helperContextKey struct{}

	// HelperContext is the context for EVM helper
	HelperContext struct {
		getBlockTime GetBlockTime
	}
)

// WithHelperCtx returns a new context with helper context
func WithHelperCtx(ctx context.Context, hctx HelperContext) context.Context {
	return context.WithValue(ctx, helperContextKey{}, hctx)
}

func NewHelperCtx(getBlockTime GetBlockTime) HelperContext {
	return HelperContext{
		getBlockTime: getBlockTime,
	}
}

// mustGetHelperCtx returns the helper context from the context
func mustGetHelperCtx(ctx context.Context) HelperContext {
	hc, ok := ctx.Value(helperContextKey{}).(HelperContext)
	if !ok {
		log.S().Panic("Miss evm helper context")
	}
	return hc
}
