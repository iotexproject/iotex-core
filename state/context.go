// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"context"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

type runActionsCtxKey struct{}

// RunActionsCtx provides the runactions with auxiliary information.
type RunActionsCtx struct {
	// height of block containing those actions
	BlockHeight uint64
	// hash of block containing those actions
	BlockHash hash.Hash32B
	// public key of producer who compose those actions
	ProducerPubKey keypair.PublicKey
	// timestamp of block containing those actions
	BlockTimeStamp int64
	// producer who compose those actions
	ProducerAddr string
	// gas Limit for perform those actions
	GasLimit *uint64
	// whether disable gas charge
	EnableGasCharge bool
}

// WithRunActionsCtx add RunActionsCtx into context.
func WithRunActionsCtx(ctx context.Context, ra RunActionsCtx) context.Context {
	return context.WithValue(ctx, runActionsCtxKey{}, ra)
}

// GetRunActionsCtx gets runActions context
func GetRunActionsCtx(ctx context.Context) (RunActionsCtx, bool) {
	ra, ok := ctx.Value(runActionsCtxKey{}).(RunActionsCtx)
	return ra, ok
}
