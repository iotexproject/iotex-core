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
	"github.com/iotexproject/iotex-core/state"
)

// TipInfo contains the tip block information
type TipInfo struct {
	Height    uint64
	Hash      hash.Hash256
	Timestamp time.Time
}

type blockchainContextKey struct{}

type blockContextKey struct{}

type actionContextKey struct{}

type commitContextKey struct{}

// BlockchainCtx provides blockchain auxiliary information.
type BlockchainCtx struct {
	// Genesis is a copy of current genesis
	Genesis genesis.Genesis
	// History indicates whether to save account/contract history or not
	History bool
	// Registry is the pointer protocol registry
	Registry *Registry
	// Tip is the information of tip block
	Tip TipInfo
	// Candidates is a list of candidates of current round
	Candidates []*state.Candidate
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

// WithCommitCtx add CommitCtx into context.
func WithCommitCtx(ctx context.Context, commit bool) context.Context {
	return context.WithValue(ctx, commitContextKey{}, commit)
}

// GetCommitCtx gets CommitCtx
func GetCommitCtx(ctx context.Context) (bool, bool) {
	cc, ok := ctx.Value(commitContextKey{}).(bool)
	return cc, ok
}
