// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"math"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// validateZanzibarEraMargin rejects a Zanzibar height that would activate
// IIP-59 too late to freeze the first era it settles.
//
// An era is settled at the last block of a boundary epoch, but the window it
// settles against is opened ~1.5 epochs earlier: PutPollResult is created at the
// midpoint of the *preceding* epoch, and that is where FreezeCandidateRewardSnapshots
// and BeginEraCOWWindow run. Both the freeze and the settlement are gated on the
// same IsZanzibar predicate, and GrantEpochReward treats "this is a boundary
// epoch" as equivalent to "a window was opened for this era" -- reward.go says
// so explicitly, and notes there is no second condition.
//
// Activate between those two points and the equivalence breaks: the freeze
// no-ops because the feature is not live yet, then the settlement fires looking
// for a window nobody opened, and that era's entire epoch reward is lost. It was
// observed as a silent zero on a local chain; nothing in the logs explains it.
//
// The rule is deliberately the exact one rather than a rounded "leave two whole
// epochs" margin. This runs at startup against the operator's real genesis, so a
// rule stricter than the mechanism would take a correctly configured fleet down
// on restart -- and Genesis.validate() is skipped for defaultConfig() and test
// literals, so no test suite would catch that first.
func validateZanzibarEraMargin(g genesis.Genesis, rp *rolldpos.Protocol) error {
	if g.ZanzibarBlockHeight == math.MaxUint64 {
		// Not scheduled; nothing to constrain.
		return nil
	}
	if rp == nil {
		// No rolldpos protocol means no epochs to reason about (non-RollDPoS
		// consensus, i.e. tests and standalone). Genesis.validate() already
		// covers the parts that do not need epoch arithmetic.
		return nil
	}
	eraLen := g.Rewarding.EpochsPerRewardEra
	if eraLen == 0 {
		// IsEraBoundary is false for every epoch, so no era is ever settled and
		// there is no freeze to be late for. Genesis.validate() rejects this
		// separately once Zanzibar is scheduled.
		return nil
	}

	activationEpoch := rp.GetEpochNum(g.ZanzibarBlockHeight)
	if activationEpoch == 0 {
		return nil
	}
	// The first boundary epoch at or after activation is the first era this
	// height is responsible for settling.
	boundaryEpoch := ((activationEpoch + eraLen - 1) / eraLen) * eraLen
	if boundaryEpoch == 0 {
		return nil
	}
	freezeHeight := eraFreezeHeight(rp, boundaryEpoch)
	if g.ZanzibarBlockHeight <= freezeHeight {
		return nil
	}
	return errors.Errorf(
		"genesis: zanzibarHeight %d lands in epoch %d, past the freeze at height %d that opens era %d "+
			"(settled at height %d) -- that era's epoch reward would be lost in full. "+
			"Use a height at or below %d, or schedule past era %d by using a height at or above %d",
		g.ZanzibarBlockHeight, activationEpoch, freezeHeight, boundaryEpoch,
		rp.GetEpochHeight(boundaryEpoch)+rp.NumBlocksByEpoch(boundaryEpoch)-1,
		freezeHeight, boundaryEpoch, rp.GetEpochHeight(boundaryEpoch)+rp.NumBlocksByEpoch(boundaryEpoch),
	)
}

// eraFreezeHeight returns the height at which the window for a boundary epoch is
// opened.
//
// It mirrors createPostSystemActions, which emits PutPollResult once the block
// reaches the midpoint of the epoch: `blkCtx.BlockHeight >= epochHeight +
// (nextEpochHeight-epochHeight)/2`. The epoch that carries it is the one before
// the boundary, so the freeze lands roughly 1.5 epochs before the settlement.
func eraFreezeHeight(rp *rolldpos.Protocol, boundaryEpoch uint64) uint64 {
	if boundaryEpoch == 0 {
		return 0
	}
	prev := boundaryEpoch - 1
	if prev == 0 {
		return rp.GetEpochHeight(boundaryEpoch)
	}
	start := rp.GetEpochHeight(prev)
	return start + rp.NumBlocksByEpoch(prev)/2
}

// checkZanzibarEraMargin resolves the registered rolldpos protocol and applies
// the margin rule.
//
// It lives here rather than in Genesis.validate() because blockchain/genesis
// cannot import rolldpos: genesis -> rolldpos -> action/protocol -> genesis is a
// cycle. rolldpos.Protocol is the epoch calculator this rule needs and
// duplicating its three-regime arithmetic (numSubEpochs / Dardanelles / Wake)
// would leave two copies of consensus-adjacent math to keep in lockstep.
func checkZanzibarEraMargin(g genesis.Genesis, reg *protocol.Registry) error {
	return validateZanzibarEraMargin(g, rolldpos.FindProtocol(reg))
}
