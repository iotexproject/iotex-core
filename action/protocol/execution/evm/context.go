package evm

import (
	"context"

	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	helperContextKey struct{}
	loggerContextKey struct{}

	// HelperContext is the context for EVM helper
	HelperContext struct {
		GetBlockHash   GetBlockHash
		GetBlockTime   GetBlockTime
		DepositGasFunc protocol.DepositGas
	}
	// LoggerContext is the context for EVM logger
	LoggerContext struct {
		buildLogger func() vm.EVMLogger
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

// WithLoggerCtx returns a new context with logger context
func WithLoggerCtx(ctx context.Context, buildLogger func() vm.EVMLogger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, LoggerContext{buildLogger: buildLogger})
}

// GetLoggerCtx returns the logger context from the context
func GetLoggerCtx(ctx context.Context) (LoggerContext, bool) {
	tc, ok := ctx.Value(loggerContextKey{}).(LoggerContext)
	return tc, ok
}
