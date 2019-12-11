package tracker

import (
	"context"

	"github.com/iotexproject/iotex-core/pkg/log"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
)

type trackerCtxKey struct{}

// Context provides auxiliary information from tracker.
type Context struct {
	Genesis genesis.Genesis
}

// WithTrackerCtx add TrackerCtx into context.
func WithTrackerCtx(ctx context.Context, t Context) context.Context {
	return context.WithValue(ctx, trackerCtxKey{}, t)
}

// GetTrackerCtx gets tracker context
func GetTrackerCtx(ctx context.Context) (Context, bool) {
	t, ok := ctx.Value(trackerCtxKey{}).(Context)
	return t, ok
}

// MustGetTrackerCtx must get tracker context.
// If context doesn't exist, this function panic.
func MustGetTrackerCtx(ctx context.Context) Context {
	t, ok := ctx.Value(trackerCtxKey{}).(Context)
	if !ok {
		log.S().Panic("Miss tracker context")
	}
	return t
}
