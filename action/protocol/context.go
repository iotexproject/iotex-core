// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"math/big"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
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
	// Genesis is a copy of current genesis
	Genesis genesis.Genesis
	// Tip is the information of tip block
	Tip TipInfo
	// Producer is the address of whom composes the block containing this action
	Producer address.Address
	// Caller is the address of whom issues this action
	Caller address.Address
	// ActionHash is the hash of the action with the sealed envelope
	ActionHash hash.Hash256
	// GasPrice is the action gas price
	GasPrice *big.Int
	// IntrinsicGas is the action intrinsic gas
	IntrinsicGas uint64
	// Nonce is the nonce of the action
	Nonce uint64
	// History indicates whether to save account/contract history or not
	History bool
	// Registry is the pointer of protocol registry
	Registry *Registry
}

// TipInfo contains the tip block information
type TipInfo struct {
	Height    uint64
	Hash      hash.Hash256
	Timestamp time.Time
}

// ValidateActionsCtx provides action validators with auxiliary information.
type ValidateActionsCtx struct {
	// height of block containing those actions
	BlockHeight uint64
	// public key of producer who compose those actions
	ProducerAddr string
	// information of the tip block
	Tip TipInfo
	// Caller is the address of whom issues the action
	Caller address.Address
	// Genesis is a copy of current genesis
	Genesis genesis.Genesis
	// Registry is the pointer of protocol registry
	Registry *Registry
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
		log.S().Panic("Miss validate actions context")
	}
	return va
}

// TODO: replace RunActionsCtx and ValidateActionsCtx with below classified independent contexts

type blockchainContextKey struct{}

type blockContextKey struct{}

type actionContextKey struct{}

// BlockchainCtx provides blockchain auxiliary information.
type BlockchainCtx struct {
	// Genesis is a copy of current genesis
	Genesis genesis.Genesis
	// History indicates whether to save account/contract history or not
	History bool
	// Registry is the pointer protocol registry
	Registry *Registry
}

// BlockCtx provides block auxiliary information.
type BlockCtx struct {
	// height of block containing those actions
	BlockHeight uint64
	// timestamp of block containing those actions
	BlockTimeStamp time.Time
	// gas Limit for perform those actions
	GasLimit uint64
	// Producer is the address of whom composes the block containing this action
	Producer address.Address
}

// ActionCtx provides action auxiliary information.
type ActionCtx struct {
	// Caller is the address of whom issues this action
	Caller address.Address
	// ActionHash is the hash of the action with the sealed envelope
	ActionHash hash.Hash256
	// GasPrice is the action gas price
	GasPrice *big.Int
	// IntrinsicGas is the action intrinsic gas
	IntrinsicGas uint64
	// Nonce is the nonce of the action
	Nonce uint64
	// History indicates whether to save account/contract history or not
}

// WithBlockchainCtx add BlockchainCtx into context.
func WithBlockchainCtx(ctx context.Context, bc BlockchainCtx) context.Context {
	return context.WithValue(ctx, blockchainContextKey{}, bc)
}

// GetBlockchainCtx gets BlockchainCtx
func GetBlockchainCtx(ctx context.Context) (BlockchainCtx, bool) {
	bc, ok := ctx.Value(blockchainContextKey{}).(BlockchainCtx)
	return bc, ok
}

// MustGetBlockchainCtx must get BlockchainCtx.
// If context doesn't exist, this function panic.
func MustGetBlockchainCtx(ctx context.Context) BlockchainCtx {
	bc, ok := ctx.Value(blockchainContextKey{}).(BlockchainCtx)
	if !ok {
		log.S().Panic("Miss blockchain context")
	}
	return bc
}

// WithBlockCtx add BlockCtx into context.
func WithBlockCtx(ctx context.Context, blk BlockCtx) context.Context {
	return context.WithValue(ctx, blockContextKey{}, blk)
}

// GetBlockCtx gets BlockCtx
func GetBlockCtx(ctx context.Context) (BlockCtx, bool) {
	blk, ok := ctx.Value(blockContextKey{}).(BlockCtx)
	return blk, ok
}

// MustGetBlockCtx must get BlockCtx .
// If context doesn't exist, this function panic.
func MustGetBlockCtx(ctx context.Context) BlockCtx {
	blk, ok := ctx.Value(blockContextKey{}).(BlockCtx)
	if !ok {
		log.S().Panic("Miss block context")
	}
	return blk
}

// WithActionCtx add ActionCtx into context.
func WithActionCtx(ctx context.Context, ac ActionCtx) context.Context {
	return context.WithValue(ctx, actionContextKey{}, ac)
}

// GetActionCtx gets ActionCtx
func GetActionCtx(ctx context.Context) (ActionCtx, bool) {
	ac, ok := ctx.Value(actionContextKey{}).(ActionCtx)
	return ac, ok
}

// MustGetActionCtx must get ActionCtx .
// If context doesn't exist, this function panic.
func MustGetActionCtx(ctx context.Context) ActionCtx {
	ac, ok := ctx.Value(actionContextKey{}).(ActionCtx)
	if !ok {
		log.S().Panic("Miss action context")
	}
	return ac
}
