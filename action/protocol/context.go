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

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	blockchainContextKey struct{}

	blockContextKey struct{}

	actionContextKey struct{}

	registryContextKey struct{}

	featureContextKey struct{}

	featureWithHeightContextKey struct{}

	vmConfigContextKey struct{}

	// TipInfo contains the tip block information
	TipInfo struct {
		Height    uint64
		Hash      hash.Hash256
		Timestamp time.Time
	}

	// BlockchainCtx provides blockchain auxiliary information.
	BlockchainCtx struct {
		// Tip is the information of tip block
		Tip TipInfo
		//ChainID of the node
		ChainID uint32
	}

	// BlockCtx provides block auxiliary information.
	BlockCtx struct {
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
	ActionCtx struct {
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
	}

	// CheckFunc is function type to check by height.
	CheckFunc func(height uint64) bool

	// FeatureCtx provides features information.
	FeatureCtx struct {
		FixDoubleChargeGas          bool
		SystemWideActionGasLimit    bool
		NotFixTopicCopyBug          bool
		SetRevertMessageToReceipt   bool
		FixGetHashFnHeight          bool
		UsePendingNonceOption       bool
		AsyncContractTrie           bool
		AddOutOfGasToTransactionLog bool
		AddChainIDToConfig          bool
		UseV2Storage                bool
		CannotUnstakeAgain          bool
		SkipStakingIndexer          bool
		ReturnFetchError            bool
		CannotTranferToSelf         bool
		NewStakingReceiptFormat     bool
		UpdateBlockMeta             bool
		CurrentEpochProductivity    bool
		FixSnapshotOrder            bool
		AllowCorrectDefaultChainID  bool
		ContractAddressInReceipt    bool
	}

	// FeatureWithHeightCtx provides feature check functions.
	FeatureWithHeightCtx struct {
		GetUnproductiveDelegates CheckFunc
		ReadStateFromDB          CheckFunc
		UseV2Staking             CheckFunc
		EnableNativeStaking      CheckFunc
		StakingCorrectGas        CheckFunc
		CalculateProbationList   CheckFunc
		LoadCandidatesLegacy     CheckFunc
	}
)

// WithRegistry adds registry to context
func WithRegistry(ctx context.Context, reg *Registry) context.Context {
	return context.WithValue(ctx, registryContextKey{}, reg)
}

// GetRegistry returns the registry from context
func GetRegistry(ctx context.Context) (*Registry, bool) {
	reg, ok := ctx.Value(registryContextKey{}).(*Registry)
	return reg, ok
}

// MustGetRegistry returns the registry from context
func MustGetRegistry(ctx context.Context) *Registry {
	reg, ok := ctx.Value(registryContextKey{}).(*Registry)
	if !ok {
		log.S().Panic("Miss registry context")
	}
	return reg
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

// WithFeatureCtx add FeatureCtx into context.
func WithFeatureCtx(ctx context.Context) context.Context {
	g := genesis.MustExtractGenesisContext(ctx)
	height := MustGetBlockCtx(ctx).BlockHeight
	return context.WithValue(
		ctx,
		featureContextKey{},
		FeatureCtx{
			FixDoubleChargeGas:          g.IsPacific(height),
			SystemWideActionGasLimit:    !g.IsAleutian(height),
			NotFixTopicCopyBug:          !g.IsAleutian(height),
			SetRevertMessageToReceipt:   g.IsHawaii(height),
			FixGetHashFnHeight:          g.IsHawaii(height),
			UsePendingNonceOption:       g.IsHawaii(height),
			AsyncContractTrie:           g.IsGreenland(height),
			AddOutOfGasToTransactionLog: !g.IsGreenland(height),
			AddChainIDToConfig:          g.IsIceland(height),
			UseV2Storage:                g.IsGreenland(height),
			CannotUnstakeAgain:          g.IsGreenland(height),
			SkipStakingIndexer:          !g.IsFairbank(height),
			ReturnFetchError:            !g.IsGreenland(height),
			CannotTranferToSelf:         g.IsHawaii(height),
			NewStakingReceiptFormat:     g.IsFbkMigration(height),
			UpdateBlockMeta:             g.IsGreenland(height),
			CurrentEpochProductivity:    g.IsGreenland(height),
			FixSnapshotOrder:            g.IsKamchatka(height),
			AllowCorrectDefaultChainID:  g.IsToBeEnabled(height),
			ContractAddressInReceipt:    g.IsToBeEnabled(height),
		},
	)
}

// GetFeatureCtx gets FeatureCtx.
func GetFeatureCtx(ctx context.Context) (FeatureCtx, bool) {
	fc, ok := ctx.Value(featureContextKey{}).(FeatureCtx)
	return fc, ok
}

// MustGetFeatureCtx must get FeatureCtx.
// If context doesn't exist, this function panic.
func MustGetFeatureCtx(ctx context.Context) FeatureCtx {
	fc, ok := ctx.Value(featureContextKey{}).(FeatureCtx)
	if !ok {
		log.S().Panic("Miss feature context")
	}
	return fc
}

// WithFeatureWithHeightCtx add FeatureWithHeightCtx into context.
func WithFeatureWithHeightCtx(ctx context.Context) context.Context {
	g := genesis.MustExtractGenesisContext(ctx)
	return context.WithValue(
		ctx,
		featureWithHeightContextKey{},
		FeatureWithHeightCtx{
			GetUnproductiveDelegates: func(height uint64) bool {
				return !g.IsEaster(height)
			},
			ReadStateFromDB: func(height uint64) bool {
				return g.IsGreenland(height)
			},
			UseV2Staking: func(height uint64) bool {
				return g.IsFairbank(height)
			},
			EnableNativeStaking: func(height uint64) bool {
				return g.IsCook(height)
			},
			StakingCorrectGas: func(height uint64) bool {
				return g.IsDaytona(height)
			},
			CalculateProbationList: func(height uint64) bool {
				return g.IsEaster(height)
			},
			LoadCandidatesLegacy: func(height uint64) bool {
				return !g.IsEaster(height)
			},
		},
	)
}

// GetFeatureWithHeightCtx gets FeatureWithHeightCtx.
func GetFeatureWithHeightCtx(ctx context.Context) (FeatureWithHeightCtx, bool) {
	fc, ok := ctx.Value(featureWithHeightContextKey{}).(FeatureWithHeightCtx)
	return fc, ok
}

// MustGetFeatureWithHeightCtx must get FeatureWithHeightCtx.
// If context doesn't exist, this function panic.
func MustGetFeatureWithHeightCtx(ctx context.Context) FeatureWithHeightCtx {
	fc, ok := ctx.Value(featureWithHeightContextKey{}).(FeatureWithHeightCtx)
	if !ok {
		log.S().Panic("Miss feature context")
	}
	return fc
}

// WithVMConfigCtx adds vm config to context
func WithVMConfigCtx(ctx context.Context, vmConfig vm.Config) context.Context {
	return context.WithValue(ctx, vmConfigContextKey{}, vmConfig)
}

// GetVMConfigCtx returns the vm config from context
func GetVMConfigCtx(ctx context.Context) (vm.Config, bool) {
	cfg, ok := ctx.Value(vmConfigContextKey{}).(vm.Config)
	return cfg, ok
}
