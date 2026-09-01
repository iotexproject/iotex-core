// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
		Height        uint64
		GasUsed       uint64
		Hash          hash.Hash256
		Timestamp     time.Time
		BaseFee       *big.Int
		BlobGasUsed   uint64
		ExcessBlobGas uint64
	}

	// BlockchainCtx provides blockchain auxiliary information.
	BlockchainCtx struct {
		// Tip is the information of tip block
		Tip TipInfo
		//ChainID is the native chain ID
		ChainID uint32
		// EvmNetworkID is the EVM network ID
		EvmNetworkID uint32
		// GetBlockHash is the function to get block hash by height
		GetBlockHash func(uint64) (hash.Hash256, error)
		// GetBlockTime is the function to get block time by height
		GetBlockTime func(uint64) (time.Time, error)
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
		// AccumTips is the accumulated tips of the block
		AccumulatedTips big.Int
		// BaseFee is the base fee of the block
		BaseFee *big.Int
		// ExcessBlobGas is the excess blob gas of the block
		ExcessBlobGas uint64
		// SkipSidecarValidation dictates to validate sidecar (for blob tx) or not
		SkipSidecarValidation bool
		// Simulate is used for read-only APIs
		Simulate bool
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
		// ReadOnly indicates two scenarios: eth_estimateGas and eth_call
		ReadOnly bool
	}

	// CheckFunc is function type to check by height.
	CheckFunc func(height uint64) bool

	// FeatureCtx provides features information.
	FeatureCtx struct {
		FixDoubleChargeGas                      bool
		SystemWideActionGasLimit                bool
		NotFixTopicCopyBug                      bool
		SetRevertMessageToReceipt               bool
		FixGetHashFnHeight                      bool
		FixSortCacheContractsAndUsePendingNonce bool
		AsyncContractTrie                       bool
		AddOutOfGasToTransactionLog             bool
		AddChainIDToConfig                      bool
		UseV2Storage                            bool
		CannotUnstakeAgain                      bool
		SkipStakingIndexer                      bool
		ReturnFetchError                        bool
		CannotTranferToSelf                     bool
		NewStakingReceiptFormat                 bool
		UpdateBlockMeta                         bool
		CurrentEpochProductivity                bool
		FixSnapshotOrder                        bool
		AllowCorrectDefaultChainID              bool
		CorrectGetHashFn                        bool
		CorrectTxLogIndex                       bool
		RevertLog                               bool
		TolerateLegacyAddress                   bool
		CreateLegacyNonceAccount                bool
		FixGasAndNonceUpdate                    bool
		FixUnproductiveDelegates                bool
		CorrectGasRefund                        bool
		SufficentBalanceGuarantee               bool
		TolerateEmptyCandidateName              bool
		SkipSystemActionNonce                   bool
		ValidateSystemAction                    bool
		AllowCorrectChainIDOnly                 bool
		AddContractStakingVotes                 bool
		FixContractStakingWeightedVotes         bool
		ExecutionSizeLimit32KB                  bool
		UseZeroNonceForFreshAccount             bool
		CandidateRegisterMustWithStake          bool
		DisableDelegateEndorsement              bool
		RefactorFreshAccountConversion          bool
		SuicideTxLogMismatchPanic               bool
		PanicUnrecoverableError                 bool
		CandidateIdentifiedByOwner              bool
		LimitedStakingContract                  bool
		MigrateNativeStake                      bool
		AddClaimRewardAddress                   bool
		EnforceLegacyEndorsement                bool
		EnableDynamicFeeTx                      bool
		EnableBlobTransaction                   bool
		EnableCancunEVM                         bool
		CorrectValidationOrder                  bool
		UnstakedButNotClearSelfStakeAmount      bool
		CheckStakingDurationUpperLimit          bool
		FixRevertSnapshot                       bool
		TimestampedStakingContract              bool
		PreStateSystemAction                    bool
		CreatePostActionStates                  bool
		NotSlashUnproductiveDelegates           bool
		CandidateBLSPublicKey                   bool
		NotUseMinSelfStakeToBeActive            bool
		StoreVoteOfNFTBucketIntoView            bool
		CandidateSlashByOwner                   bool
		CandidateBLSPublicKeyNotCopied          bool
		OnlyOwnerCanUpdateBLSPublicKey          bool
		PrePectraEVM                            bool
		// AlwaysWriteCachedContract if true, CommitContracts writes back all cached
		// contracts regardless of whether they were modified; if false, only dirty
		// contracts are committed and written back
		AlwaysWriteCachedContract     bool
		NoCandidateExitQueue          bool
		FixInContractTransferLogTopic bool
		// CorrectPrestateForAbsentKeys, when true, makes contract.GetCommittedState
		// return the true tx-start prestate value (zero, via trie.ErrNotExist) for
		// storage slots that were absent in the pre-tx trie. Prior to this gate the
		// contract-level committed[] cache was populated with post-mutation values
		// for such slots, causing EIP-2200 SSTORE dynamic gas to misclassify
		// dirty in-place writes as SSTORE_RESET (100 → 2900 gas overcharge per hit).
		CorrectPrestateForAbsentKeys bool
		// NoVoterRewardDistribution gates IIP-59's protocol-native voter reward
		// distribution. Pre-fork (true) the poll layer does NOT freeze the
		// per-candidate CandidateRewardSnapshot and rewarding stays on the legacy
		// Hermes path. Post-fork (false) the snapshot is written at every
		// PutPollResult and rewarding consumes it. Bound to
		// !g.IsZanzibar(height): default zero-value = active after fork.
		NoVoterRewardDistribution bool
		// FixEpochSettlementFaultHandling corrects how faults raised during
		// epoch settlement are classified, in three places:
		//
		//   - the auto-deposit lookup and the compound bucket read no longer
		//     reroute a voter's share on a fault only some nodes saw; an absent
		//     or withdrawn bucket, which every node reads identically, still
		//     degrades to a direct credit;
		//   - an era boundary reached with no copy-on-write window skips the
		//     drain cursor instead of failing the whole epoch grant, which used
		//     to take that epoch's commissions, foundation bonus and sentinel
		//     down with it;
		//   - a GrantEpochReward error settles a Failure receipt only when it
		//     is derivable from committed state; the rest fail the block rather
		//     than let one node commit "this epoch paid nobody" against
		//     everyone else's full grant.
		//
		// Deliberately separate from NoVoterRewardDistribution. That gate is
		// what turns IIP-59 on, so a chain where it has already activated is
		// executing the pre-correction behaviour; changing it in place would
		// alter blocks that chain has already committed. This flag therefore
		// needs its own height, one fork later than the gate above:
		// Zanzibar Beta.
		FixEpochSettlementFaultHandling bool
		// RequireProfileForHermesMigration makes the activation-block Hermes
		// opt-in migration skip candidates whose DelegateProfile portions are
		// missing or only half set, leaving them on the off-chain Hermes payout
		// instead of moving them into an on-chain path that freezes them at
		// 100% commission and pays their voters nothing.
		//
		// Separate from NoVoterRewardDistribution because the migration is a
		// one-shot that runs in the single block that gate turns on at. A chain
		// where it has already fired has that block committed; changing what it
		// did would alter the block's receipt root and fork any node replaying
		// history. Carried by Zanzibar Beta.
		//
		// Because that migration is a one-shot in the block Zanzibar activates
		// at, this flag only ever has an effect where Zanzibar Beta is not
		// scheduled after Zanzibar. On a chain that activated Zanzibar first
		// the migration has already run unguarded and cannot be re-run, so the
		// flag is inert there by construction -- it exists to protect a chain
		// that activates both together. See ZanzibarBetaBlockHeight.
		RequireProfileForHermesMigration bool
		// EmitEraFreezeLog makes the era freeze emit one DelegateRewardFrozen
		// log per frozen delegate.
		//
		// Freezing is otherwise silent: the freeze block and its neighbours
		// carry the same single untopiced block-reward log, so an event-driven
		// indexer cannot tell an era was frozen, which delegates are in it, or
		// what commission each was frozen at.
		//
		// Receipt logs are part of the receipt root, so this needs its own
		// height rather than riding on the gate that turns IIP-59 on: a chain
		// that has already produced freeze blocks without these logs would
		// recompute different roots for them. Carried by Zanzibar Beta.
		EmitEraFreezeLog bool
		// EnforceBLSPoP gates the BLS proof-of-possession requirement at
		// candidate register / update. The staking handler validates
		// blsPubKey only with BLS12381PublicKeyFromBytes (format +
		// subgroup); without a possession proof, IIP-52's planned
		// FastAggregateVerify path is vulnerable to a rogue-key
		// aggregate-forgery attack (a registered candidate could publish
		// pk_rogue = g^x − Σ(other pubkeys) and, once aggregation goes
		// live, forge a 2/3+ quorum certificate with a single signature).
		// Activating EnforceBLSPoP BEFORE the BLS aggregation fork closes
		// the window for collecting un-attested pubkeys.
		EnforceBLSPoP bool
		// OptionalCandidateBLSPublicKey relaxes the Xingu-era rule that every
		// candidate register / update must carry a BLS public key. Post-fork
		// the key is optional -- nothing consumes it until IIP-52 aggregation
		// activates -- but a proof-of-possession may not arrive without one.
		// An update that omits both leaves any previously registered key
		// untouched.
		OptionalCandidateBLSPublicKey bool
		// ValidateHeaderGasUsed makes a validating node recompute the header's
		// gasUsed and blobGasUsed from the receipts it produced while executing
		// the block, and reject the block when either disagrees with the value
		// the proposer put in the header.
		//
		// The proposer fills both fields from its own receipts (see
		// workingSet.CreateBuilder), but until now nothing on the validating
		// side re-derived them, so the two header fields were accepted as
		// given. gasUsed feeds the EIP-1559 base fee of the next block and
		// blobGasUsed feeds its excessBlobGas, and both are served over the
		// API, so they need to agree with the executed block.
		//
		// Needs its own height: a chain that has already committed blocks whose
		// header gas fields do not match their receipts must keep accepting
		// them on replay, so the check may only start at a fork boundary.
		ValidateHeaderGasUsed bool
	}

	// FeatureWithHeightCtx provides feature check functions.
	FeatureWithHeightCtx struct {
		GetUnproductiveDelegates        CheckFunc
		ReadStateFromDB                 CheckFunc
		UseV2Staking                    CheckFunc
		EnableNativeStaking             CheckFunc
		StakingCorrectGas               CheckFunc
		CalculateProbationList          CheckFunc
		LoadCandidatesLegacy            CheckFunc
		CandCenterHasAlias              CheckFunc
		CandidateWithoutIdentity        CheckFunc
		CandidateWithoutIdentityStorage CheckFunc
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
			FixDoubleChargeGas:                      g.IsPacific(height),
			SystemWideActionGasLimit:                !g.IsAleutian(height),
			NotFixTopicCopyBug:                      !g.IsAleutian(height),
			SetRevertMessageToReceipt:               g.IsHawaii(height),
			FixGetHashFnHeight:                      g.IsHawaii(height),
			FixSortCacheContractsAndUsePendingNonce: g.IsHawaii(height),
			AsyncContractTrie:                       g.IsGreenland(height),
			AddOutOfGasToTransactionLog:             !g.IsGreenland(height),
			AddChainIDToConfig:                      g.IsIceland(height),
			UseV2Storage:                            g.IsGreenland(height),
			CannotUnstakeAgain:                      g.IsGreenland(height),
			SkipStakingIndexer:                      !g.IsFairbank(height),
			ReturnFetchError:                        !g.IsGreenland(height),
			CannotTranferToSelf:                     g.IsHawaii(height),
			NewStakingReceiptFormat:                 g.IsFbkMigration(height),
			UpdateBlockMeta:                         g.IsGreenland(height),
			CurrentEpochProductivity:                g.IsGreenland(height),
			FixSnapshotOrder:                        g.IsKamchatka(height),
			AllowCorrectDefaultChainID:              g.IsMidway(height),
			CorrectGetHashFn:                        g.IsMidway(height),
			CorrectTxLogIndex:                       g.IsMidway(height),
			RevertLog:                               g.IsMidway(height),
			TolerateLegacyAddress:                   !g.IsNewfoundland(height),
			CreateLegacyNonceAccount:                !g.IsOkhotsk(height),
			FixGasAndNonceUpdate:                    g.IsOkhotsk(height),
			FixUnproductiveDelegates:                g.IsOkhotsk(height),
			CorrectGasRefund:                        g.IsOkhotsk(height),
			SufficentBalanceGuarantee:               g.IsOkhotsk(height),
			TolerateEmptyCandidateName:              !g.IsPalau(height),
			SkipSystemActionNonce:                   g.IsPalau(height),
			ValidateSystemAction:                    g.IsQuebec(height),
			AllowCorrectChainIDOnly:                 g.IsQuebec(height),
			AddContractStakingVotes:                 g.IsQuebec(height),
			FixContractStakingWeightedVotes:         g.IsRedsea(height),
			ExecutionSizeLimit32KB:                  !g.IsSumatra(height),
			UseZeroNonceForFreshAccount:             g.IsSumatra(height),
			CandidateRegisterMustWithStake:          !g.IsTsunami(height),
			DisableDelegateEndorsement:              !g.IsTsunami(height),
			RefactorFreshAccountConversion:          g.IsTsunami(height),
			SuicideTxLogMismatchPanic:               g.IsUpernavik(height),
			PanicUnrecoverableError:                 g.IsUpernavik(height),
			CandidateIdentifiedByOwner:              !g.IsUpernavik(height),
			LimitedStakingContract:                  !g.IsUpernavik(height),
			MigrateNativeStake:                      g.IsUpernavik(height),
			AddClaimRewardAddress:                   g.IsUpernavik(height),
			EnforceLegacyEndorsement:                !g.IsUpernavik(height),
			EnableDynamicFeeTx:                      g.IsVanuatu(height),
			EnableBlobTransaction:                   g.IsVanuatu(height),
			EnableCancunEVM:                         g.IsVanuatu(height),
			CorrectValidationOrder:                  g.IsVanuatu(height),
			UnstakedButNotClearSelfStakeAmount:      !g.IsVanuatu(height),
			CheckStakingDurationUpperLimit:          g.IsVanuatu(height),
			FixRevertSnapshot:                       g.IsVanuatu(height),
			TimestampedStakingContract:              g.IsWake(height),
			PreStateSystemAction:                    !g.IsWake(height),
			CreatePostActionStates:                  g.IsWake(height),
			NotSlashUnproductiveDelegates:           !g.IsXingu(height),
			CandidateBLSPublicKey:                   g.IsXingu(height),
			NotUseMinSelfStakeToBeActive:            !g.IsXingu(height),
			StoreVoteOfNFTBucketIntoView:            !g.IsXingu(height),
			CandidateSlashByOwner:                   !g.IsXinguBeta(height),
			CandidateBLSPublicKeyNotCopied:          !g.IsXinguBeta(height),
			OnlyOwnerCanUpdateBLSPublicKey:          !g.IsYap(height),
			PrePectraEVM:                            !g.IsYap(height),
			AlwaysWriteCachedContract:               !g.IsYap(height),
			NoCandidateExitQueue:                    !g.IsYap(height),
			// Zanzibar. These five are exactly what v2.5.0-rc0 activated, and
			// testnet has already run it -- so this set is fixed by what that
			// chain committed, not by what would be tidy to group together.
			FixInContractTransferLogTopic: g.IsZanzibar(height),
			CorrectPrestateForAbsentKeys:  g.IsZanzibar(height),
			NoVoterRewardDistribution:     !g.IsZanzibar(height),
			EnforceBLSPoP:                 g.IsZanzibar(height),
			OptionalCandidateBLSPublicKey: g.IsZanzibar(height),
			// Zanzibar Beta. Every one of these corrects behaviour Zanzibar
			// already turned on, and none of them existed in rc0. Selecting
			// them with Zanzibar would rewrite blocks testnet has committed.
			FixEpochSettlementFaultHandling:  g.IsZanzibarBeta(height),
			RequireProfileForHermesMigration: g.IsZanzibarBeta(height),
			EmitEraFreezeLog:                 g.IsZanzibarBeta(height),
			// Same batch: a correction that did not exist in rc0, so it rides
			// Beta for the same reason the three above do.
			ValidateHeaderGasUsed: g.IsZanzibarBeta(height),
		},
	)
}

func (fCtx *FeatureCtx) Tolerate(err error) bool {
	if fCtx.TolerateEmptyCandidateName && errors.Cause(err) == action.ErrInvalidCanName {
		return true
	}
	return false
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
		log.L().Panic("Miss feature context")
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
			CandCenterHasAlias: func(height uint64) bool {
				return !g.IsOkhotsk(height)
			},
			CandidateWithoutIdentity: func(height uint64) bool {
				return !g.IsYapBeta(height)
			},
			CandidateWithoutIdentityStorage: func(height uint64) bool {
				return !g.IsYap(height)
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

// IsEraBoundary reports whether the given epoch number falls on an IIP-59 voter reward era boundary.
// An era boundary is any epoch where epochNum%epochsPerEra == 0. Epoch 0 is never a boundary because
// the genesis pre-epoch has no rewards to distribute; the first live boundary is at epoch epochsPerEra.
// epochsPerEra == 0 disables era boundaries entirely (used by tests that opt out of the era cadence).
func IsEraBoundary(epochNum, epochsPerEra uint64) bool {
	if epochsPerEra == 0 || epochNum == 0 {
		return false
	}
	return epochNum%epochsPerEra == 0
}
