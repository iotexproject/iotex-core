package net

import (
	"context"
)

type noDialCtxKey struct{}

var noDial = noDialCtxKey{}

// WithNoDial constructs a new context with an option that instructs the network
// to not attempt a new dial when opening a stream.
func WithNoDial(ctx context.Context, reason string) context.Context {
	return context.WithValue(ctx, noDial, reason)
}

// GetNoDial returns true if the no dial option is set in the context.
func GetNoDial(ctx context.Context) (nodial bool, reason string) {
	v := ctx.Value(noDial)
	if v != nil {
		return true, v.(string)
	}

	return false, ""
}
