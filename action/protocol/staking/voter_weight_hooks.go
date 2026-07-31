// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
)

type contractBucketVoterWeightObserver struct {
	csm                 CandidateStateManager
	calculateVoteWeight CalculateVoteWeightFunc
	height              uint64
}

func newContractBucketVoterWeightObserver(
	sm protocol.StateManager,
	calculateVoteWeight CalculateVoteWeightFunc,
	height uint64,
) (*contractBucketVoterWeightObserver, error) {
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return nil, err
	}
	return &contractBucketVoterWeightObserver{
		csm: csm, calculateVoteWeight: calculateVoteWeight, height: height,
	}, nil
}

func (o *contractBucketVoterWeightObserver) PutContractBucket(previous, current *contractstaking.Bucket) {
	o.apply(previous, true)
	o.apply(current, false)
}

func (o *contractBucketVoterWeightObserver) DeleteContractBucket(previous *contractstaking.Bucket) {
	o.apply(previous, true)
}

func (o *contractBucketVoterWeightObserver) ReviseContractBucket(bucket *contractstaking.Bucket) {
	if o == nil || bucket == nil || o.csm == nil || o.calculateVoteWeight == nil || o.height == 0 {
		return
	}
	previous := o.calculateVoteWeight(bucket, o.height-1)
	current := o.calculateVoteWeight(bucket, o.height)
	o.applyWeight(bucket, new(big.Int).Sub(current, previous))
}

func (o *contractBucketVoterWeightObserver) apply(bucket *contractstaking.Bucket, subtract bool) {
	if o == nil || bucket == nil || o.csm == nil || o.calculateVoteWeight == nil {
		return
	}
	weight := o.calculateVoteWeight(bucket, o.height)
	if subtract {
		weight.Neg(weight)
	}
	o.applyWeight(bucket, weight)
}

func (o *contractBucketVoterWeightObserver) applyWeight(bucket *contractstaking.Bucket, weight *big.Int) {
	if weight == nil || weight.Sign() == 0 {
		return
	}
	candidate := o.csm.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return
	}
	applyVoterWeightDelta(o.csm, candidate.GetIdentifier(), bucket.Owner, weight)
}

// addCandidateVotes credits weight to a candidate's aggregate vote total and to
// the voter's entry in the IIP-59 view, deriving both from the same number.
//
// Every site that changes a candidate's votes on behalf of a bucket owner must
// go through this or subCandidateVotes rather than calling Candidate.AddVote /
// Candidate.SubVote directly. A site that updates the candidate total but not
// the view leaves the two derivations of the same quantity to drift, and the
// drift stays invisible until an era freeze bakes the wrong per-voter weights
// into a reward snapshot. TestCandidateVoteMutationsUseChokePoint fails the
// build if a new call site bypasses them.
//
// The error is returned unwrapped so callers keep their own receipt failure
// status, which differs between the add and sub directions at several sites.
//
// Pass a nil voter when the caller deliberately attributes the view delta
// elsewhere (the migration path does this — see newNFTBucketEventHandlerForMigration).
func addCandidateVotes(csm CandidateStateManager, cand *Candidate, voter address.Address, weight *big.Int) error {
	if err := cand.AddVote(weight); err != nil {
		return err
	}
	applyVoterWeightDelta(csm, cand.GetIdentifier(), voter, weight)
	return nil
}

// subCandidateVotes debits weight from a candidate's aggregate vote total and
// from the voter's entry in the IIP-59 view. See addCandidateVotes.
func subCandidateVotes(csm CandidateStateManager, cand *Candidate, voter address.Address, weight *big.Int) error {
	if err := cand.SubVote(weight); err != nil {
		return err
	}
	applyVoterWeightDelta(csm, cand.GetIdentifier(), voter, new(big.Int).Neg(weight))
	return nil
}

// applyVoterWeightDelta is the single entry point every staking handler uses
// to keep the IIP-59 VoterWeightView in sync with on-chain bucket changes.
// No-op when the view has not been installed (pre-fork / test setups that skip
// Protocol.Start) and when delta is zero, so callers can wire it next to any
// existing candidate.AddVote / candidate.SubVote site without first checking
// the fork flag.
//
// Prefer addCandidateVotes / subCandidateVotes. Call this directly only when
// the voter's weight moves without the candidate's total changing — transferring
// a bucket between owners, and the contract-bucket observer, which is driven by
// the contract stake view rather than by a candidate mutation.
//
// candIdentifier must be the candidate's identifier address (not operator) —
// same key the view uses internally. voter is the bucket owner.
func applyVoterWeightDelta(csm CandidateStateManager, candIdentifier address.Address, voter address.Address, delta *big.Int) {
	if delta == nil || delta.Sign() == 0 {
		return
	}
	if csm == nil || candIdentifier == nil || voter == nil {
		return
	}
	view := csm.DirtyView()
	if view == nil || view.voterWeights == nil {
		return
	}
	view.voterWeights.Apply(hash.BytesToHash160(candIdentifier.Bytes()), voter, delta)
}
