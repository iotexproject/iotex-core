package evm

import (
	"context"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	helperContextKey struct{}

	// HelperContext is the context for EVM helper
	HelperContext struct {
		GetBlockHash   GetBlockHash
		GetBlockTime   GetBlockTime
		DepositGasFunc protocol.DepositGas
	}
)

// WithHelperCtx returns a new context with helper context
func WithHelperCtx(ctx context.Context, hctx HelperContext) context.Context {
	return context.WithValue(ctx, helperContextKey{}, hctx)
}

// mustGetHelperCtx returns the helper context from the context
func mustGetHelperCtx(ctx context.Context) HelperContext {
	hc, ok := ctx.Value(helperContextKey{}).(HelperContext)
	if !ok {
		log.S().Panic("Miss evm helper context")
	}
	return hc
}
