package rolldpos

import (
	"context"

	"github.com/iotexproject/iotex-core/pkg/log"
)

type evmNetworkIDCtx struct{}

func withEvmNetworkIDCtx(parent context.Context, evmNetworkID uint32) context.Context {
	return context.WithValue(parent, evmNetworkIDCtx{}, evmNetworkID)
}

func evmNetworkIDFromCtx(ctx context.Context) (uint32, bool) {
	evmNetworkID, ok := ctx.Value(evmNetworkIDCtx{}).(uint32)
	return evmNetworkID, ok
}

func mustGetEvmNetworkIDFromCtx(ctx context.Context) uint32 {
	id, ok := ctx.Value(evmNetworkIDCtx{}).(uint32)
	if !ok {
		log.S().Panic("Miss evmNetworkID context")
	}
	return id
}
