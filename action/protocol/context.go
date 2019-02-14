// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"

	"github.com/iotexproject/iotex-core/address"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

type runActionsCtxKey struct{}

type validateActionsCtxKey struct{}

// RunActionsCtx provides the runactions with auxiliary information.
type RunActionsCtx struct {
	// EpochNumber is the epoch number
	EpochNumber uint64
	// height of block containing those actions
	BlockHeight uint64
	// hash of block containing those actions
	BlockHash hash.Hash256
	// timestamp of block containing those actions
	BlockTimeStamp int64
	// gas Limit for perform those actions
	GasLimit *uint64
	// ActionGasLimit is the action level gas limit
	ActionGasLimit uint64
	// whether disable gas charge
	EnableGasCharge bool
	// Producer is the address of whom composes the block containing this action
	Producer address.Address
	// Caller is the address of whom issues this action
	Caller address.Address
	// ActionHash is the hash of the action with the sealed envelope
	ActionHash hash.Hash256
	// Nonce is the nonce of the action
	Nonce uint64
}

// ValidateActionsCtx provides action validators with auxiliary information.
type ValidateActionsCtx struct {
	// height of block containing those actions
	BlockHeight uint64
	// public key of producer who compose those actions
	ProducerAddr string
	// Caller is the address of whom issues the action
	Caller address.Address
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

// WithValidateActionsCtx add ValidateActionsCtx into context.
func WithValidateActionsCtx(ctx context.Context, va ValidateActionsCtx) context.Context {
	return context.WithValue(ctx, validateActionsCtxKey{}, va)
}

// GetValidateActionsCtx gets validateActions context
func GetValidateActionsCtx(ctx context.Context) (ValidateActionsCtx, bool) {
	va, ok := ctx.Value(validateActionsCtxKey{}).(ValidateActionsCtx)
	return va, ok
}
