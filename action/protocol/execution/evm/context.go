package evm

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	helperContextKey struct{}

	tracerContextKey struct{}

	statelessValidationContextKey struct{}

	// HelperContext is the context for EVM helper
	HelperContext struct {
		GetBlockHash   GetBlockHash
		GetBlockTime   GetBlockTime
		DepositGasFunc protocol.DepositGas
	}
	// TracerContext is the context for EVM tracer
	TracerContext struct {
		CaptureTx                       func([]byte, *action.Receipt)
		CaptureContractStorageAccesses  func([]ContractStorageAccess)
		CaptureContractStorageWitnesses func(map[common.Address]*ContractStorageWitness)
	}

	// StatelessValidationContext provides the per-block contract-storage witness
	// set used by storage-light validators.
	StatelessValidationContext struct {
		Enabled         bool
		ActionWitnesses map[hash.Hash256]map[common.Address]*ContractStorageWitness
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

// WithTracerCtx returns a new context with tracer context
func WithTracerCtx(ctx context.Context, tctx TracerContext) context.Context {
	return context.WithValue(ctx, tracerContextKey{}, tctx)
}

// GetTracerCtx returns the tracer context from the context
func GetTracerCtx(ctx context.Context) (TracerContext, bool) {
	tc, ok := ctx.Value(tracerContextKey{}).(TracerContext)
	return tc, ok
}

// WithStatelessValidationCtx returns a new context with stateless-validation data.
func WithStatelessValidationCtx(ctx context.Context, svCtx StatelessValidationContext) context.Context {
	return context.WithValue(ctx, statelessValidationContextKey{}, svCtx)
}

// GetStatelessValidationCtx returns the stateless-validation context from the context.
func GetStatelessValidationCtx(ctx context.Context) (StatelessValidationContext, bool) {
	svCtx, ok := ctx.Value(statelessValidationContextKey{}).(StatelessValidationContext)
	return svCtx, ok
}

// ContractStorageWitnessesForAction returns the witness set for one action.
func (svCtx StatelessValidationContext) ContractStorageWitnessesForAction(actionHash hash.Hash256) map[common.Address]*ContractStorageWitness {
	if svCtx.ActionWitnesses == nil {
		return nil
	}
	return svCtx.ActionWitnesses[actionHash]
}
