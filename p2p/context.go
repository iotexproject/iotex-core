// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package p2p

import "context"

type p2pCtxKey struct{}

// Context provides the auxiliary information Agent network operations
type Context struct {
	ChainID uint32
}

// WitContext add Agent context into context.
func WitContext(ctx context.Context, p2pCtx Context) context.Context {
	return context.WithValue(ctx, p2pCtxKey{}, p2pCtx)
}

// GetContext gets Agent context
func GetContext(ctx context.Context) (Context, bool) {
	p2pCtx, ok := ctx.Value(p2pCtxKey{}).(Context)
	return p2pCtx, ok
}
