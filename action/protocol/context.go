// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"math/big"
	"time"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type runActionsCtxKey struct{}

type validateActionsCtxKey struct{}

// RunActionsCtx provides the runactions with auxiliary information.
type RunActionsCtx struct {
	// height of block containing those actions
	BlockHeight uint64
	// timestamp of block containing those actions
	BlockTimeStamp time.Time
	// gas Limit for perform those actions
	GasLimit uint64
	// Producer is the address of whom composes the block containing this action
	Producer address.Address
	// Caller is the address of whom issues this action
	Caller address.Address
	// ActionHash is the hash of the action with the sealed envelope
	ActionHash hash.Hash256
	// ActionGasLimit is the action gas limit
	ActionGasLimit uint64
	// GasPrice is the action gas price
	GasPrice *big.Int
	// IntrinsicGas is the action intrinsic gas
	IntrinsicGas uint64
	// Nonce is the nonce of the action
	Nonce uint64
	// Registry is the pointer protocol registry
	Registry *Registry
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

// MustGetRunActionsCtx must get runActions context.
// If context doesn't exist, this function panic.
func MustGetRunActionsCtx(ctx context.Context) RunActionsCtx {
	ra, ok := ctx.Value(runActionsCtxKey{}).(RunActionsCtx)
	if !ok {
		log.S().Panic("Miss run actions context")
	}
	return ra
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

// MustGetValidateActionsCtx gets validateActions context.
// If context doesn't exist, this function panic.
func MustGetValidateActionsCtx(ctx context.Context) ValidateActionsCtx {
	va, ok := ctx.Value(validateActionsCtxKey{}).(ValidateActionsCtx)
	if !ok {
		log.S().Panic("Miss run actions context")
	}
	return va
}
