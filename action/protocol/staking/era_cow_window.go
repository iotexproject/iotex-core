// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/state"
)

// This file owns the staking-side lifecycle of the IIP-59 copy-on-write window.
// Native and contract frozen reads live with their respective state readers;
// the storage-neutral COW mechanism itself lives in eracow.

// BeginEraCOWWindow opens the copy-on-write window for the era frozen at
// freezeHeight.
//
// The poll protocol calls BeginEraCOWWindow immediately after
// FreezeCandidateRewardSnapshots at the end of the freeze block H. Everything
// written from there on is "after H" and is copied aside on first touch.
//
// H is NOT the era boundary block. FreezeCandidateRewardSnapshots rides a PutPollResult
// action, which is created around the midpoint of the epoch *preceding* the
// target epoch, while voter reward distribution state is created at the last block
// of the boundary epoch -- roughly 1.5 epochs later (~2,160 blocks, ~90 minutes
// on mainnet). That gap is deliberate and is not a divergence risk, because H
// travels with the work as FreezeHeight and every recompute evaluates at it.
// See docs/iip-59-distribution-architecture.md §2.1.
//
// Besides opening the window this freezes the two bucket high-water marks:
//
//   - the native bucket index upper bound, read from totalBucketCount. It is
//     the next index putBucket will hand out. Indices are strictly monotonic
//     (delBucket never decrements the counter), so a native bucket with index
//     >= this number cannot have existed at H.
//   - each staking contract's NumOfBuckets, which is the highest contract
//     bucket id seen so far, burnt ones included. Contract bucket ids come from
//     a strictly monotonic counter inside the contract and are never reused, so
//     a contract bucket with id > its contract's number cannot have existed at
//     H either. Note the boundary differs: the native number is a next-index,
//     the contract number is a max-seen-id.
//
// Both are frozen as scalars rather than copied on write. That is strictly
// stronger: a scalar still rejects a post-H bucket even if that bucket's own
// copy were missed, whereas a copied counter would only be as good as the copy.
//
// No-op pre-activation; eracow.Begin checks the fork gate before touching
// state, and the two reads below are behind the same check.
func BeginEraCOWWindow(ctx context.Context, sm protocol.StateManager, freezeHeight uint64) error {
	if !eracow.Enabled(ctx) {
		return nil
	}
	if freezeHeight == 0 {
		// Post-activation there is always a block context, so this cannot
		// happen on a real chain. Refuse rather than open a window whose
		// FreezeHeight is indistinguishable from "no frozen era": every
		// consumer reads 0 as absence and would silently fall back to live
		// state for a whole drain.
		return errors.New("staking: cannot open an era copy-on-write window at height 0")
	}
	var tc totalBucketCount
	if _, err := sm.State(
		&tc,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return errors.Wrap(err, "staking: read total bucket count for era freeze")
	}
	contractLimits, err := contractstaking.BucketIndexUpperBounds(sm)
	if err != nil {
		return errors.Wrap(err, "staking: read contract bucket counts for era freeze")
	}
	return eracow.Begin(ctx, sm, freezeHeight, tc.Count(), contractLimits)
}

// SealEraCOWWindow closes the era window and queues its copies for collection.
//
// Call it when the era's drain completes. After it, the copy-on-write hooks on
// every bucket write become branch-only no-ops until the next boundary.
//
// No-op pre-activation and when no window is open.
func SealEraCOWWindow(ctx context.Context, sm protocol.StateManager) error {
	return eracow.Seal(ctx, sm)
}

// CollectEraCOWGarbage deletes up to max copied entries older than the open
// window and returns how many it deleted.
//
// Intended to be called once per block. It is bounded on purpose: an era can
// accumulate tens of thousands of copies and deleting them in one block would
// blow the very block budget the drain is chunked to respect.
//
// No-op pre-activation and when there is no backlog.
func CollectEraCOWGarbage(ctx context.Context, sm protocol.StateManager, max int) (int, error) {
	return eracow.CollectGarbage(ctx, sm, max)
}

// LoadEraCOWWindow returns the open era window, or the zero value when none is
// open. The drain uses it for the bucket high-water marks.
func LoadEraCOWWindow(sr protocol.StateReader) (eracow.Window, error) {
	return eracow.LoadWindow(sr)
}
