// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

// This file is the staking-side face of the IIP-59 era copy-on-write layer:
// opening and closing the window, and reading covered keys as of the freeze
// height. The mechanism itself lives in the eracow package; what is here is the
// knowledge of which staking keys are covered and how they are addressed.

// freezeHeightOf returns the height an era boundary running now freezes state
// at. It prefers the block context and falls back to the state manager's own
// height for callers (tests, tooling) that have no block context.
//
// Zero means "no height is available", which is not an error here: it is what
// a unit test driving the freezer with a bare context gets, and it flows
// through as FreezeHeight=0 — the "no frozen era" value every consumer already
// has to handle. A real boundary cannot occur at height 0, and
// beginEraCOWWindow rejects the combination of an open fork gate and a zero
// height so the case cannot pass silently where it would matter.
func freezeHeightOf(ctx context.Context, sr protocol.StateReader) (uint64, error) {
	if blkCtx, ok := protocol.GetBlockCtx(ctx); ok && blkCtx.BlockHeight > 0 {
		return blkCtx.BlockHeight, nil
	}
	h, err := sr.Height()
	if err != nil {
		return 0, errors.Wrap(err, "staking: read height for era freeze")
	}
	return h, nil
}

// beginEraCOWWindow opens the copy-on-write window for the era frozen at
// freezeHeight.
//
// It runs inside FreezePollSnapshot, i.e. at the end of the freeze block H,
// after every mutation belonging to that block has already been applied.
// Everything written from here on is "after H" and is copied aside on first
// touch.
//
// H is NOT the era boundary block. FreezePollSnapshot rides a PutPollResult
// action, which is created around the midpoint of the epoch *preceding* the
// target epoch, while the drain cursor for the era is created at the last block
// of the boundary epoch -- roughly 1.5 epochs later (~2,160 blocks, ~90 minutes
// on mainnet). That gap is deliberate and is not a divergence risk, because H
// travels with the work as FreezeHeight and every recompute evaluates at it.
// See docs/iip-59-distribution-architecture.md §2.1.
//
// Besides opening the window this freezes the two bucket high-water marks:
//
//   - the native totalBucketCount, which is the next index putBucket will hand
//     out. Indices are strictly monotonic (delBucket never decrements the
//     counter), so a native bucket with index >= this number cannot have
//     existed at H.
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
func beginEraCOWWindow(ctx context.Context, sm protocol.StateManager, freezeHeight uint64) error {
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
	contractCounts, err := contractstaking.BucketHighWaterMarks(sm)
	if err != nil {
		return errors.Wrap(err, "staking: read contract bucket counts for era freeze")
	}
	return eracow.Begin(ctx, sm, freezeHeight, tc.Count(), contractCounts)
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

// CollectEraCOWGarbage deletes up to max copied entries belonging to already
// sealed eras and returns how many it deleted.
//
// Intended to be called once per block. It is bounded on purpose: an era can
// accumulate tens of thousands of copies and deleting them in one block would
// blow the very block budget the drain is chunked to respect.
//
// No-op pre-activation and when there is no backlog.
func CollectEraCOWGarbage(ctx context.Context, sm protocol.StateManager, max int) (int, error) {
	return eracow.CollectGarbage(ctx, sm, max)
}

// EraCOWWindow returns the open era window, or the zero value when none is
// open. The drain uses it for the bucket high-water marks.
func EraCOWWindow(sr protocol.StateReader) (eracow.Window, error) {
	return eracow.LoadWindow(sr)
}

// NoSelfStakeBucketIndex is the sentinel a candidate carries when it has no
// self-stake bucket. Exported so the rewarding protocol, which freezes this
// index into its per-delegate drain work, can spell the absence the same way
// instead of keeping its own copy of the constant.
const NoSelfStakeBucketIndex = uint64(candidateNoSelfStakeBucketIndex)

// FrozenSelfStake is an era's view of one candidate's self-stake bucket.
//
// The candidate record is mutable during a drain, so the index is frozen at
// the boundary and carried forward. FreezeHeight doubles as the presence flag:
// zero means the caller has no frozen era at all (a pre-activation or legacy
// record), which is different from "the candidate had no self-stake bucket at
// H" and must not be collapsed into it — bucket index 0 is a real bucket.
type FrozenSelfStake struct {
	FreezeHeight uint64
	BucketIdx    uint64
}

// Known reports whether this value came from a real era freeze.
func (f FrozenSelfStake) Known() bool { return f.FreezeHeight > 0 }

// Covers reports whether bucketIdx was the candidate's self-stake bucket at
// the freeze height. Always false when the era is unknown.
func (f FrozenSelfStake) Covers(bucketIdx uint64) bool {
	return f.Known() && f.BucketIdx == bucketIdx
}

// ---------------------------------------------------------- frozen reads --

// contractStakingAddresser returns the reader that owns the layout of
// contract-staking keys, for the sole purpose of asking it where a covered key
// lives.
//
// It is built with no options, which is a statement, not an omission: the era
// drain reads the state trie, never the Erigon-only mirror. The mirror writer
// (nftEventHandler's manager, built with protocol.ErigonStoreOnlyOption) is
// excluded from the copy-on-write layer altogether —
// ContractStakingStateManager.cowSession returns nil for it — so there are no
// copies on that side to resolve against, and addressing the mirror here would
// pair a trie-side copy with a mirror-side live value.
//
// Taking the address from this reader rather than assembling it here is the
// point: whatever options the layout grows, both halves of a resolve get them
// from the same place.
func contractStakingAddresser(sr protocol.StateReader) *contractstaking.ContractStakingStateReader {
	return contractstaking.NewStateReader(sr)
}

// ErrBucketPostFreeze is returned when a bucket cannot have existed at the
// freeze height, judged by its index alone. It is a normal outcome: buckets are
// created constantly and the drain must skip the ones that postdate the era it
// is paying out.
var ErrBucketPostFreeze = errors.New("staking: bucket did not exist at the era freeze height")

// FrozenNativeBucket reads a native bucket as of the era freeze height.
//
// Two independent filters have to agree that the bucket existed at H: the
// high-water mark on window rejects an index that was never assigned by then,
// and the copy-on-write layer supplies the pre-mutation value for one that was.
// Either alone would do in the absence of bugs; both together mean a miss in
// one is not a payout against post-boundary state.
//
// Returns ErrBucketPostFreeze for a bucket that postdates H, and
// state.ErrStateNotExist for one that has since been withdrawn and was never
// copied (which can only happen if a mutation bypassed the choke points).
func FrozenNativeBucket(sr protocol.StateReader, window eracow.Window, index uint64) (*VoteBucket, error) {
	if !window.Open() {
		return nil, errors.New("staking: no era window open")
	}
	if !window.NativeBucketExisted(index) {
		return nil, errors.Wrapf(ErrBucketPostFreeze, "native bucket %d", index)
	}
	vb := &VoteBucket{}
	err := eracow.Resolve(
		sr, window.FreezeHeight,
		eracow.KindNativeBucket, eracow.NativeBucketSubkey(index),
		vb,
		nativeBucketStateOpts(index)...,
	)
	switch {
	case err == nil:
		// bucketKey carries the index but the stored value does not always, so
		// keep the caller's view of Index authoritative — the weight recompute
		// compares it against the frozen SelfStakeBucketIdx.
		vb.Index = index
		return vb, nil
	case errors.Is(err, eracow.ErrNotFrozen):
		return nil, errors.Wrapf(ErrBucketPostFreeze, "native bucket %d", index)
	default:
		return nil, err
	}
}

// FrozenNativeBucketIndices reads a voter's native bucket index list as of the
// era freeze height.
//
// An absent list is not an error: a voter with no native buckets at H simply
// has none to enumerate, and the empty slice says so.
func FrozenNativeBucketIndices(sr protocol.StateReader, window eracow.Window, voter address.Address) (BucketIndices, error) {
	if !window.Open() {
		return nil, errors.New("staking: no era window open")
	}
	var bis BucketIndices
	err := eracow.Resolve(
		sr, window.FreezeHeight,
		eracow.KindNativeVoterIndex, eracow.AddrSubkey(voter.Bytes()),
		&bis,
		nativeBucketIndexStateOpts(voter, _voterIndex)...,
	)
	switch {
	case err == nil:
		return bis, nil
	case errors.Is(err, eracow.ErrNotFrozen), errors.Cause(err) == state.ErrStateNotExist:
		return nil, nil
	default:
		return nil, err
	}
}

// FrozenContractBucket reads a contract-staking bucket as of the era freeze
// height. See FrozenNativeBucket; the id boundary differs (max-seen id, so the
// bound is inclusive).
func FrozenContractBucket(
	sr protocol.StateReader,
	window eracow.Window,
	contract address.Address,
	bucketID uint64,
) (*contractstaking.Bucket, error) {
	if !window.Open() {
		return nil, errors.New("staking: no era window open")
	}
	if !window.ContractBucketExisted(contract.Bytes(), bucketID) {
		// Distinguish "id is above the frozen mark" (routine: the bucket was
		// minted after H) from "this contract has no frozen mark at all". The
		// second drops every bucket of the contract from every frozen weight,
		// so it must not pass as routine. It is still a *deny* — defaulting to
		// allow here would admit post-freeze buckets into a frozen era, which
		// is the one outcome worse than an under-payment — but it is a deny
		// that says so.
		if !window.ContractKnown(contract.Bytes()) {
			log.L().Error("IIP-59: contract-staking contract has no frozen bucket high-water mark; "+
				"all of its buckets are excluded from this era's voter weights",
				zap.String("contract", contract.String()),
				zap.Uint64("bucketID", bucketID),
				zap.Uint64("freezeHeight", window.FreezeHeight),
			)
		}
		return nil, errors.Wrapf(ErrBucketPostFreeze, "contract bucket %d of %s", bucketID, contract.String())
	}
	bkt := &contractstaking.Bucket{}
	err := eracow.Resolve(
		sr, window.FreezeHeight,
		eracow.KindLSDBucket, eracow.LSDBucketSubkey(contract.Bytes(), bucketID),
		bkt,
		contractStakingAddresser(sr).BucketStateOpts(contract, bucketID)...,
	)
	switch {
	case err == nil:
		return bkt, nil
	case errors.Is(err, eracow.ErrNotFrozen):
		return nil, errors.Wrapf(ErrBucketPostFreeze, "contract bucket %d of %s", bucketID, contract.String())
	default:
		return nil, err
	}
}

// FrozenContractBucketRefs reads an owner's contract-staking bucket list as of
// the era freeze height. An absent list yields nil, not an error.
func FrozenContractBucketRefs(
	sr protocol.StateReader,
	window eracow.Window,
	owner address.Address,
) (contractstaking.ContractBucketRefs, error) {
	if !window.Open() {
		return nil, errors.New("staking: no era window open")
	}
	var refs contractstaking.ContractBucketRefs
	err := eracow.Resolve(
		sr, window.FreezeHeight,
		eracow.KindLSDVoterIndex, eracow.AddrSubkey(owner.Bytes()),
		&refs,
		contractStakingAddresser(sr).OwnerIndexStateOpts(owner)...,
	)
	switch {
	case err == nil:
		return refs, nil
	case errors.Is(err, eracow.ErrNotFrozen), errors.Cause(err) == state.ErrStateNotExist:
		return nil, nil
	default:
		return nil, err
	}
}

// FrozenVoterWeight recomputes what one voter's buckets are worth to one
// candidate as of an era freeze height.
//
// evalHeight is an explicit parameter, and callers must pass the era's freeze
// height rather than the height of the block they are running in. Contract
// buckets that are not timestamp-based have their remaining duration measured
// against a block height, so the same bucket evaluated in two different drain
// chunks would otherwise be worth two different amounts. Copy-on-write cannot
// fix that: the drifting input is the evaluation height itself, not a stored
// value. Making it a parameter rather than reading blkCtx here is the point --
// a caller cannot pass the wrong height by omission.
//
// selfStakeBucketIdx must likewise be the value frozen into the era's
// per-delegate work record, not the live candidate's: the drain mutates
// candidates as it pays them.
func FrozenVoterWeight(
	sr protocol.StateReader,
	window eracow.Window,
	p *Protocol,
	candidate address.Address,
	voter address.Address,
	selfStakeBucketIdx uint64,
	evalHeight uint64,
) (*big.Int, error) {
	if p == nil {
		return nil, errors.New("staking: nil protocol")
	}
	total := new(big.Int)

	indices, err := FrozenNativeBucketIndices(sr, window, voter)
	if err != nil {
		return nil, err
	}
	for _, index := range indices {
		bkt, err := FrozenNativeBucket(sr, window, index)
		switch {
		case err == nil:
		case errors.Is(err, ErrBucketPostFreeze), errors.Cause(err) == state.ErrStateNotExist:
			continue
		default:
			return nil, err
		}
		if bkt.Candidate == nil || !address.Equal(bkt.Candidate, candidate) {
			continue
		}
		if bkt.isUnstaked() {
			continue
		}
		selfStake := bkt.Index == selfStakeBucketIdx
		total.Add(total, p.calculateVoteWeight(bkt, selfStake))
	}

	refs, err := FrozenContractBucketRefs(sr, window, voter)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		bkt, err := FrozenContractBucket(sr, window, ref.Contract, ref.BucketID)
		switch {
		case err == nil:
		case errors.Is(err, ErrBucketPostFreeze), errors.Cause(err) == state.ErrStateNotExist:
			continue
		default:
			return nil, err
		}
		if bkt.Candidate == nil || !address.Equal(bkt.Candidate, candidate) {
			continue
		}
		// Contract buckets never carry the self-stake bonus, which is why
		// selfStakeBucketIdx does not appear here: a candidate's self-stake
		// bucket is always a native one, and contract buckets all report
		// Index = 0, so testing the index here would hand the bonus to every
		// contract bucket of a candidate whose self-stake bucket is index 0.
		total.Add(total, p.calculateContractBucketVoteWeight(bkt, evalHeight))
	}
	return total, nil
}
